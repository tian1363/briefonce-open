package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/charactergen"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/userctx"
)

var imageTransformLabels = map[string]string{
	"studio_white":   "专业白底图",
	"visible_detail": "可见细节特写",
}

func imageTransformPrompt(mode, focus string) string {
	base := "只使用输入照片中真实可见的商品作为依据。保持商品的颜色、轮廓、材质纹理、缝线、五金、图案、文字和标识与原图一致。不要添加未出现的配件、功能、品牌、尺寸、背面或被遮挡结构。输出单件商品摄影图，不要添加宣传文案或水印。"
	if mode == "studio_white" {
		return base + "去除原始挂杆、衣架、背景杂物和原图环境；商品保持自然完整的原有形状和比例，置于纯白背景，以柔和均匀棚拍光线呈现。不要把原图不可见部分补画成确定细节，也不要改造成另一款商品。"
	}
	if focus != "" {
		return base + "制作商品细节特写。用户指定的部位名称为“" + focus + "”；只放大原图中清晰可见的对应区域，若该区域不可见，不要虚构。保留真实表面纹理与结构，用简洁浅色背景和柔和光线突出局部。"
	}
	return base + "制作一张商品细节特写，只选择原图中最清晰可见的局部，例如材质、缝线或五金；不要虚构无法辨认的细节。保留真实表面纹理与结构，用简洁浅色背景和柔和光线突出局部。"
}

func (h *Handler) listImageTransformJobs(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	userID := userIDFromRequest(r)
	if _, err := h.store.GetUserProduct(userID, productID); err != nil {
		writeError(w, http.StatusNotFound, errors.New("产品资料不存在"))
		return
	}
	items, err := h.store.ListImageTransformJobs(userID, productID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) startImageTransformBatch(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	userID := userIDFromRequest(r)
	if _, err := h.store.GetUserProduct(userID, productID); err != nil {
		writeError(w, http.StatusNotFound, errors.New("产品资料不存在"))
		return
	}
	var in struct {
		SourceAssetID    string               `json:"source_asset_id"`
		Modes            []string             `json:"modes"`
		DetailFocus      string               `json:"detail_focus"`
		ReferenceAssetID string               `json:"reference_asset_id"`
		ReferenceFocus   string               `json:"reference_focus"`
		ImageOptions     charactergen.Options `json:"image_options"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(&struct{}{}) != io.EOF {
		writeError(w, http.StatusBadRequest, errors.New("商品图处理参数无效"))
		return
	}
	if len(in.Modes) == 0 || len(in.Modes) > 2 {
		writeError(w, http.StatusBadRequest, errors.New("请选择一到两种商品图处理方式"))
		return
	}
	seen := map[string]bool{}
	for _, mode := range in.Modes {
		if imageTransformLabels[mode] == "" || seen[mode] {
			writeError(w, http.StatusBadRequest, errors.New("商品图处理方式无效或重复"))
			return
		}
		seen[mode] = true
	}
	in.DetailFocus = strings.TrimSpace(in.DetailFocus)
	if utf8.RuneCountInString(in.DetailFocus) > 80 {
		writeError(w, http.StatusBadRequest, errors.New("细节部位请控制在 80 字以内"))
		return
	}
	in.ReferenceFocus = strings.TrimSpace(in.ReferenceFocus)
	if in.ReferenceAssetID != "" && in.ReferenceFocus == "" {
		in.ReferenceFocus = "参考构图与视觉风格；不得复制参考图中的商品、文字或标识"
	}
	if utf8.RuneCountInString(in.ReferenceFocus) > 220 {
		writeError(w, http.StatusBadRequest, errors.New("参考重点请控制在 220 字以内"))
		return
	}
	asset, err := h.store.GetProductAsset(in.SourceAssetID)
	if err != nil || asset.ProductID != productID || asset.Kind != "image" {
		writeError(w, http.StatusNotFound, errors.New("商品参考图不存在"))
		return
	}
	if info, statErr := os.Stat(asset.Path); statErr != nil || info.Size() > 10<<20 || asset.SizeBytes > 10<<20 {
		writeError(w, http.StatusBadRequest, errors.New("商品图不可用或超过 10MB"))
		return
	}
	inputImage, err := os.ReadFile(asset.Path)
	if err != nil || len(inputImage) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("商品图读取失败"))
		return
	}
	if mime := http.DetectContentType(inputImage); mime != "image/jpeg" && mime != "image/png" {
		writeError(w, http.StatusBadRequest, errors.New("请使用 JPG 或 PNG 商品图"))
		return
	}
	var referenceImage []byte
	if in.ReferenceAssetID != "" {
		ref, refErr := h.store.GetProductAsset(in.ReferenceAssetID)
		if refErr != nil || ref.ProductID != productID || ref.Kind != "reference" {
			writeError(w, http.StatusNotFound, errors.New("视觉参考图不存在"))
			return
		}
		referenceImage, refErr = os.ReadFile(ref.Path)
		if refErr != nil || len(referenceImage) == 0 || len(referenceImage) > 10<<20 || (http.DetectContentType(referenceImage) != "image/jpeg" && http.DetectContentType(referenceImage) != "image/png") {
			writeError(w, http.StatusBadRequest, errors.New("视觉参考图不可用"))
			return
		}
	} else if in.ReferenceFocus != "" {
		writeError(w, http.StatusBadRequest, errors.New("请先选择视觉参考图"))
		return
	}
	cfg, options, err := h.imageRuntime(r, "image_edit", in.ImageOptions)
	if err != nil || cfg.Model == "" {
		writeError(w, http.StatusBadRequest, errors.New("请先配置图片编辑模型"))
		return
	}
	if cfg.Source == "managed" && !h.cfg.AllowManagedMode {
		writeError(w, http.StatusForbidden, errors.New("当前账号不能使用平台图片额度"))
		return
	}
	if cfg.APIKey == "" {
		writeError(w, http.StatusBadRequest, errors.New("请先连接图片编辑模型 API Key"))
		return
	}
	if !h.authLimit.allow("image-transform:"+userID, 3, 5*time.Minute) {
		writeError(w, http.StatusTooManyRequests, errors.New("生成请求较多，请稍后重试"))
		return
	}
	if cfg.Provider == "zexapi" {
		options = normalizeImageOptions(options, "1:1")
		refs := [][]byte{inputImage}
		if len(referenceImage) > 0 {
			refs = append(refs, referenceImage)
		}
		for _, mode := range in.Modes {
			prompt := imageTransformPrompt(mode, in.DetailFocus) + in.ReferenceFocus
			if err = charactergen.ValidateInput(options, prompt, refs); err != nil {
				writeError(w, 400, err)
				return
			}
		}
	}
	charges := make([]string, len(in.Modes))
	if cfg.Source == "managed" {
		for i := range charges {
			charges[i], err = h.reserveImage(r, "image_edit", cfg, options)
			if err != nil {
				for _, id := range charges {
					_ = h.store.FinishUsage(id, false)
				}
				writeError(w, http.StatusPaymentRequired, err)
				return
			}
		}
	}
	batchID := fmt.Sprintf("product-image-%d", time.Now().UTC().UnixNano())
	items, err := h.store.CreateImageTransformBatch(userID, productID, asset.ID, batchID, in.DetailFocus, in.ReferenceAssetID, in.ReferenceFocus, in.Modes)
	if err != nil {
		for _, id := range charges {
			_ = h.store.FinishUsage(id, false)
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	usageTasks := make([]jobs.UsageTask, len(items))
	for i, item := range items {
		usageTasks[i] = jobs.UsageTask{UserID: userID, TaskID: item.ID, ChargeID: charges[i], Title: imageTransformLabels[item.Mode], ProductID: productID, Model: cfg.Model, Source: cfg.Source, Capability: "image_edit"}
	}
	if err := h.store.RecordUsageTasks(usageTasks); err != nil {
		for i, item := range items {
			_ = h.store.FinishUsage(charges[i], false)
			_ = h.store.FinishImageTransformJob(userID, item.ID, "", "计费记录保存失败")
		}
		writeError(w, 500, err)
		return
	}
	linkAnalyticsUsage(r, usageTasks)
	if cfg.Provider == "zexapi" {
		refs := [][]byte{inputImage}
		if len(referenceImage) > 0 {
			refs = append(refs, referenceImage)
		}
		for i, item := range items {
			prompt := imageTransformPrompt(item.Mode, item.DetailFocus)
			if len(referenceImage) > 0 {
				prompt += " 图片1是商品事实依据，图片2仅作为视觉风格参考，不得复制图片2的商品和文字。参考重点：" + item.ReferenceFocus
			}
			err = h.queueImage(jobs.ProviderImageJob{ID: item.ID, UserID: userID, Kind: "transform", ProductID: productID, ChargeID: charges[i], Capability: "image_edit"}, cfg, options, prompt, refs)
			if err != nil {
				for k := i; k < len(items); k++ {
					_ = h.store.FinishUsage(charges[k], false)
					_ = h.store.FinishImageTransformJob(userID, items[k].ID, "", err.Error())
				}
				writeError(w, 500, err)
				return
			}
		}
		writeJSON(w, http.StatusAccepted, items)
		return
	}
	user, _ := userctx.FromContext(r.Context())
	go func() {
		var batchStop error
		for i, item := range items {
			func() {
				success := false
				defer func() { _ = h.store.FinishUsage(charges[i], success) }()
				if err := h.store.StartImageTransformJob(userID, item.ID); err != nil {
					_ = h.store.FinishImageTransformJob(userID, item.ID, "", err.Error())
					return
				}
				if batchStop != nil {
					_ = h.store.FinishImageTransformJob(userID, item.ID, "", "批次已停止："+batchStop.Error())
					return
				}
				ctx, cancel := context.WithTimeout(userctx.WithUser(context.Background(), user), 4*time.Minute)
				defer cancel()
				prompt := imageTransformPrompt(item.Mode, item.DetailFocus)
				var data []byte
				var genErr error
				if len(referenceImage) > 0 {
					prompt += " 图片1是唯一的商品事实依据；图片2仅供参考用户指定的视觉风格，不得复制图片2中的商品、品牌、人物、文字或未证实属性。参考重点：" + item.ReferenceFocus
					data, genErr = h.characterImages.EditReferences(ctx, cfg, prompt, "错误商品、改变颜色和标识、复制参考图商品或文字、虚构细节、水印", "1024*1024", [][]byte{inputImage, referenceImage})
				} else {
					data, genErr = h.characterImages.EditReference(ctx, cfg, prompt, "错误商品、改变颜色和标识、虚构细节、文字、水印", inputImage)
				}
				if genErr != nil {
					if errors.Is(genErr, charactergen.ErrRateLimited) || errors.Is(genErr, charactergen.ErrModelUnavailable) || errors.Is(genErr, charactergen.ErrAccessDenied) {
						batchStop = genErr
					}
					_ = h.store.FinishImageTransformJob(userID, item.ID, "", genErr.Error())
					return
				}
				path, saveErr := h.files.SaveProductImage(data)
				if saveErr != nil {
					_ = h.store.FinishImageTransformJob(userID, item.ID, "", saveErr.Error())
					return
				}
				baseName := strings.TrimSuffix(filepath.Base(asset.OriginalName), filepath.Ext(asset.OriginalName))
				if baseName == "" {
					baseName = "商品图"
				}
				if utf8.RuneCountInString(baseName) > 40 {
					baseName = string([]rune(baseName)[:40])
				}
				assetOut, saveErr := h.store.CreateProductAsset(jobs.ProductAsset{ProductID: productID, Kind: "image", Path: path, OriginalName: baseName + "-" + imageTransformLabels[item.Mode] + filepath.Ext(path), MimeType: http.DetectContentType(data), SizeBytes: int64(len(data))})
				if saveErr != nil {
					_ = os.Remove(path)
					_ = h.store.FinishImageTransformJob(userID, item.ID, "", "商品图保存失败")
					return
				}
				if err := h.store.FinishImageTransformJob(userID, item.ID, assetOut.ID, ""); err != nil {
					_ = h.store.DeleteProductAsset(assetOut.ID)
					_ = os.Remove(path)
					return
				}
				success = true
			}()
		}
	}()
	writeJSON(w, http.StatusAccepted, items)
}
