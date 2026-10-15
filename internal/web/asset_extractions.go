package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/assetextract"
	"github.com/tian1363/scriptagent/internal/charactergen"
	"github.com/tian1363/scriptagent/internal/storage"
)

func (h *Handler) listAssetExtractions(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListAssetExtractions(userIDFromRequest(r))
	if err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, items)
}

func (h *Handler) listProductObservations(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	userID := userIDFromRequest(r)
	if _, err := h.store.GetUserProduct(userID, productID); err != nil {
		writeError(w, 404, errors.New("产品不存在"))
		return
	}
	items, err := h.store.ListAssetExtractions(userID)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	type observation struct {
		ID          string    `json:"id"`
		Label       string    `json:"label"`
		Content     string    `json:"content"`
		Description string    `json:"description"`
		AssetIDs    []string  `json:"asset_ids"`
		CreatedAt   time.Time `json:"created_at"`
	}
	result := []observation{}
	for _, item := range items {
		if item.ProductID != productID {
			continue
		}
		content, readErr := h.store.ReadProductObservation(userID, productID, item.ID)
		if readErr == nil {
			assetIDs := []string{}
			for _, variant := range item.Variants {
				if variant.ProductAssetID != "" {
					assetIDs = append(assetIDs, h.store.CurrentProductAssetID(variant.ProductAssetID))
				}
			}
			if len(assetIDs) == 0 && item.ProductAssetID != "" {
				assetIDs = append(assetIDs, h.store.CurrentProductAssetID(item.ProductAssetID))
			}
			result = append(result, observation{ID: item.ID, Label: item.Label, Content: content, Description: assetextract.ProductAssetDescription(item), AssetIDs: assetIDs, CreatedAt: item.CreatedAt})
		}
	}
	writeJSON(w, 200, result)
}

func (h *Handler) startAssetExtraction(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var input assetextract.Input
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil || d.Decode(&struct{}{}) != io.EOF {
		writeError(w, 400, errors.New("提取参数格式无效"))
		return
	}
	userID := userIDFromRequest(r)
	if !h.authLimit.allow("asset-extract:"+userID, 4, 5*time.Minute) {
		writeError(w, 429, errors.New("提取请求较多，请稍后再试"))
		return
	}
	item, err := h.assetExtractor.Start(r.Context(), userID, input)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

type assetExtractionBatchInput struct {
	assetextract.Input
	Kinds []string `json:"kinds"`
}

type assetExtractionBatchError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func validateExtractionKinds(kinds []string) error {
	if len(kinds) == 0 || len(kinds) > 8 {
		return errors.New("请选择 1 到 8 种资产类型")
	}
	seen := map[string]bool{}
	for _, kind := range kinds {
		if !assetextract.ValidKind(kind) || seen[kind] {
			return errors.New("资产类型无效或重复")
		}
		seen[kind] = true
	}
	return nil
}

func (h *Handler) startAssetExtractionBatch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var input assetExtractionBatchInput
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil || d.Decode(&struct{}{}) != io.EOF {
		writeError(w, 400, errors.New("提取参数格式无效"))
		return
	}
	h.startExtractionBatch(w, r, input)
}

func (h *Handler) startUploadedAssetExtractionBatch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, storage.MaxAssetBytes+(1<<20))
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeError(w, 400, errors.New("参考视频需小于 100MB"))
		return
	}
	defer r.MultipartForm.RemoveAll()
	var input assetExtractionBatchInput
	if err := json.Unmarshal([]byte(r.FormValue("kinds")), &input.Kinds); err != nil {
		writeError(w, 400, errors.New("资产类型格式无效"))
		return
	}
	if err := validateExtractionKinds(input.Kinds); err != nil {
		writeError(w, 400, err)
		return
	}
	seconds, err := strconv.ParseFloat(r.FormValue("at_seconds"), 64)
	if err != nil {
		writeError(w, 400, errors.New("请指定有效的视频时间点"))
		return
	}
	file, header, err := r.FormFile("video")
	if err != nil {
		writeError(w, 400, errors.New("请选择参考视频"))
		return
	}
	defer file.Close()
	if header.Size > storage.MaxAssetBytes {
		writeError(w, 400, errors.New("参考视频需小于 100MB"))
		return
	}
	path, err := h.files.SaveUpload(file, header, "video")
	if err != nil {
		writeError(w, 400, err)
		return
	}
	defer os.Remove(path)
	input.Input = assetextract.Input{SourcePath: path, Label: r.FormValue("label"), AtSeconds: seconds}
	if raw := r.FormValue("image_options"); raw != "" {
		var options charactergen.Options
		d := json.NewDecoder(strings.NewReader(raw))
		d.DisallowUnknownFields()
		if d.Decode(&options) != nil {
			writeError(w, 400, errors.New("图片参数格式无效"))
			return
		}
		input.ImageOptions = options
	}
	h.startExtractionBatch(w, r, input)
}

func (h *Handler) startExtractionBatch(w http.ResponseWriter, r *http.Request, input assetExtractionBatchInput) {
	if err := validateExtractionKinds(input.Kinds); err != nil {
		writeError(w, 400, err)
		return
	}
	userID := userIDFromRequest(r)
	if !h.authLimit.allow("asset-extract:"+userID, 4, 5*time.Minute) {
		writeError(w, 429, errors.New("提取请求较多，请稍后再试"))
		return
	}
	items := []any{}
	failures := []assetExtractionBatchError{}
	for _, kind := range input.Kinds {
		in := input.Input
		in.Kind = kind
		if in.SourcePath != "" {
			// Each async task owns one link, so cleanup cannot remove another task's source.
			stub, err := os.CreateTemp(filepath.Dir(in.SourcePath), ".asset-batch-*")
			if err != nil {
				failures = append(failures, assetExtractionBatchError{kind, "暂时无法准备参考视频"})
				continue
			}
			link := stub.Name()
			stub.Close()
			os.Remove(link)
			if err := os.Link(in.SourcePath, link); err != nil {
				failures = append(failures, assetExtractionBatchError{kind, "暂时无法准备参考视频"})
				continue
			}
			in.SourcePath = link
		}
		item, err := h.assetExtractor.Start(r.Context(), userID, in)
		if err != nil {
			if in.SourcePath != "" {
				_ = os.Remove(in.SourcePath)
			}
			failures = append(failures, assetExtractionBatchError{kind, err.Error()})
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		writeError(w, 400, errors.New(strings.Join(batchFailureMessages(failures), "；")))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"items": items, "errors": failures})
}

func batchFailureMessages(items []assetExtractionBatchError) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.Kind+"："+item.Message)
	}
	return result
}

func (h *Handler) startUploadedAssetExtraction(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, storage.MaxAssetBytes+(1<<20))
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeError(w, 400, errors.New("参考视频需小于 100MB"))
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("video")
	if err != nil {
		writeError(w, 400, errors.New("请选择参考视频"))
		return
	}
	defer file.Close()
	if header.Size > storage.MaxAssetBytes {
		writeError(w, 400, errors.New("参考视频需小于 100MB"))
		return
	}
	path, err := h.files.SaveUpload(file, header, "video")
	if err != nil {
		writeError(w, 400, err)
		return
	}
	seconds, err := strconv.ParseFloat(r.FormValue("at_seconds"), 64)
	if err != nil {
		_ = os.Remove(path)
		writeError(w, 400, errors.New("请指定有效的视频时间点"))
		return
	}
	userID := userIDFromRequest(r)
	if !h.authLimit.allow("asset-extract:"+userID, 4, 5*time.Minute) {
		_ = os.Remove(path)
		writeError(w, 429, errors.New("提取请求较多，请稍后再试"))
		return
	}
	var options charactergen.Options
	if raw := r.FormValue("image_options"); raw != "" {
		d := json.NewDecoder(strings.NewReader(raw))
		d.DisallowUnknownFields()
		if d.Decode(&options) != nil {
			_ = os.Remove(path)
			writeError(w, 400, errors.New("图片参数格式无效"))
			return
		}
	}
	item, err := h.assetExtractor.Start(r.Context(), userID, assetextract.Input{SourcePath: path, Kind: r.FormValue("kind"), Label: r.FormValue("label"), AtSeconds: seconds, ImageOptions: options})
	if err != nil {
		_ = os.Remove(path)
		writeError(w, 400, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

func (h *Handler) importAssetExtraction(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var input struct {
		ProductID string `json:"product_id"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil || d.Decode(&struct{}{}) != io.EOF {
		writeError(w, 400, errors.New("导入参数格式无效"))
		return
	}
	item, err := h.assetExtractor.Import(r.Context(), userIDFromRequest(r), chi.URLParam(r, "id"), input.ProductID)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	writeJSON(w, 200, item)
}

func (h *Handler) getAssetExtraction(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetAssetExtraction(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 404, errors.New("资产提取任务不存在"))
		return
	}
	writeJSON(w, 200, item)
}

func (h *Handler) acceptAssetExtractionReview(w http.ResponseWriter, r *http.Request) {
	item, err := h.assetExtractor.ApproveReview(r.Context(), userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 400, err)
		return
	}
	writeJSON(w, 200, item)
}

func (h *Handler) getAssetExtractionFrame(w http.ResponseWriter, r *http.Request) {
	h.serveAssetExtractionImage(w, r, false)
}

func (h *Handler) getAssetExtractionImage(w http.ResponseWriter, r *http.Request) {
	h.serveAssetExtractionImage(w, r, true)
}

func (h *Handler) getAssetExtractionVariantFrame(w http.ResponseWriter, r *http.Request) {
	h.serveAssetExtractionVariant(w, r, false)
}

func (h *Handler) getAssetExtractionVariantImage(w http.ResponseWriter, r *http.Request) {
	h.serveAssetExtractionVariant(w, r, true)
}

func (h *Handler) getAssetExtractionVariantClip(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetAssetExtraction(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil || item.Status != "ready" {
		writeError(w, 404, errors.New("提取结果不存在"))
		return
	}
	index, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil || index < 0 || index >= len(item.Variants) {
		writeError(w, 404, errors.New("该片段不存在"))
		return
	}
	path := item.Variants[index].VideoPath
	if path == "" {
		writeError(w, 404, errors.New("该资产没有动态片段"))
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeError(w, 404, errors.New("视频文件不存在"))
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

func (h *Handler) serveAssetExtractionVariant(w http.ResponseWriter, r *http.Request, generated bool) {
	item, err := h.store.GetAssetExtraction(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil || (item.Status != "ready" && item.Status != "needs_review") {
		writeError(w, 404, errors.New("提取结果不存在"))
		return
	}
	index, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil || index < 0 || index >= len(item.Variants) {
		writeError(w, 404, errors.New("该视角不存在"))
		return
	}
	path := item.Variants[index].FramePath
	if generated {
		path = item.Variants[index].ImagePath
	}
	if path == "" {
		writeError(w, 404, errors.New("图片不存在"))
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeError(w, 404, errors.New("图片文件不存在"))
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

func (h *Handler) serveAssetExtractionImage(w http.ResponseWriter, r *http.Request, generated bool) {
	item, err := h.store.GetAssetExtraction(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 404, errors.New("资产提取任务不存在"))
		return
	}
	path := item.FramePath
	if generated {
		if item.Status != "ready" && item.Status != "needs_review" {
			writeError(w, 404, errors.New("资产图尚未就绪"))
			return
		}
		path = item.ImagePath
	}
	if path == "" {
		writeError(w, 404, errors.New("图片不存在"))
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeError(w, 404, errors.New("图片文件不存在"))
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

func (h *Handler) compactProductObservations(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	userID := userIDFromRequest(r)
	if _, err := h.store.GetUserProduct(userID, productID); err != nil {
		writeError(w, 404, errors.New("产品不存在"))
		return
	}
	items, err := h.store.ListAssetExtractions(userID)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	count := 0
	for _, item := range items {
		if item.ProductID != productID {
			continue
		}
		if err = h.store.WriteProductObservation(userID, productID, item.ID, assetextract.ProductAssetDescription(item)); err != nil {
			writeError(w, 500, err)
			return
		}
		count++
	}
	writeJSON(w, 200, map[string]int{"updated": count})
}
