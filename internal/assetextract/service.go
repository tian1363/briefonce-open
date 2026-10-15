package assetextract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/tian1363/scriptagent/internal/charactergen"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/storage"
	"github.com/tian1363/scriptagent/internal/userctx"
)

type Input struct {
	ProductID       string               `json:"product_id"`
	SourcePath      string               `json:"-"`
	Kind            string               `json:"kind"`
	Label           string               `json:"label"`
	SourceAssetID   string               `json:"source_asset_id"`
	SourceVideoID   string               `json:"source_video_id"`
	SourceBatchID   string               `json:"source_batch_id"`
	SourceVariantID string               `json:"source_variant_id"`
	ConversationID  string               `json:"conversation_id"`
	AttachmentID    string               `json:"attachment_id"`
	AtSeconds       float64              `json:"at_seconds"`
	ImageOptions    charactergen.Options `json:"image_options"`
}

type Service struct {
	Store        *jobs.Store
	Files        *storage.LocalStore
	Images       *charactergen.Client
	Vision       *model.DashScopeClient
	AllowManaged bool
}

var slots = make(chan struct{}, 8)

func ValidKind(kind string) bool {
	switch kind {
	case "person", "product", "scene", "pose", "action", "camera", "brand_logo", "video_watermark":
		return true
	}
	return false
}

var importMu sync.Mutex

func New(store *jobs.Store, files *storage.LocalStore, allowManaged bool) *Service {
	return &Service{Store: store, Files: files, Images: charactergen.New(), Vision: model.NewDashScopeClient(model.DashScopeConfig{APIKey: os.Getenv("DASHSCOPE_API_KEY"), Provider: store, Recorder: store, AllowManaged: allowManaged}), AllowManaged: allowManaged}
}

func (s *Service) Start(ctx context.Context, userID string, input Input) (*jobs.AssetExtraction, error) {
	input.ProductID = strings.TrimSpace(input.ProductID)
	input.Label = strings.TrimSpace(input.Label)
	if !ValidKind(input.Kind) {
		return nil, errors.New("请选择有效的资产类型")
	}
	if math.IsNaN(input.AtSeconds) || math.IsInf(input.AtSeconds, 0) || input.AtSeconds < 0 {
		return nil, errors.New("请指定有效的视频时间点")
	}
	if utf8.RuneCountInString(input.Label) > 80 {
		return nil, errors.New("资产名称最多 80 字")
	}
	path, sourceType, sourceID, sourceAuxID, err := s.sourcePath(userID, input)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		return nil, errors.New("参考视频文件不可用")
	}
	seconds, err := probeDuration(ctx, path)
	if err != nil || seconds <= 0 || seconds > 1800 {
		return nil, errors.New("参考视频需可读取且时长不超过 30 分钟")
	}
	if input.AtSeconds >= seconds {
		return nil, fmt.Errorf("时间点需小于视频时长 %.1f 秒", seconds)
	}
	var cfg model.RuntimeConfig
	if input.Kind == "person" || input.Kind == "product" || input.Kind == "scene" || input.Kind == "brand_logo" || input.Kind == "video_watermark" {
		cfg, err = s.Store.GetModelRuntimeConfig(ctx, "image_edit")
		if err != nil || cfg.Model == "" {
			return nil, errors.New("请先在设置中配置图片编辑模型")
		}
		if cfg.Source == "managed" {
			if !s.AllowManaged {
				return nil, errors.New("当前账号不能使用平台图片额度")
			}
		}
		if cfg.APIKey == "" {
			return nil, errors.New("请先连接图片编辑模型 API Key")
		}
	}
	if cfg.Provider == "zexapi" {
		options := input.ImageOptions
		if options.Model == "" {
			options.Model = cfg.Model
		}
		if options.AspectRatio == "" && options.Size == "" {
			options.AspectRatio = "1:1"
		}
		if options.Model == "nano-banana-2.1" && options.ImageSize == "" {
			options.ImageSize = "1K"
		}
		if _, _, err = s.Store.ImageQuote(options); err != nil {
			return nil, err
		}
		cfg.Model = options.Model
		cfg.ImageAspectRatio = options.AspectRatio
		cfg.ImagePixelSize = options.Size
		cfg.ImageSize = options.ImageSize
		cfg.ImageUserID = userID
		cfg.ImagePriceVersion = options.PriceVersion
	}
	visionCfg, err := s.Store.GetModelRuntimeConfig(userctx.WithUser(ctx, userctx.User{ID: userID}), "multimodal")
	if err != nil || visionCfg.Model == "" || (visionCfg.Source == "managed" && !s.AllowManaged) || visionCfg.APIKey == "" {
		return nil, errors.New("请先在设置中配置可用的多模态模型，以生成资产视觉解析")
	}
	frame, err := extractFrame(ctx, path, input.AtSeconds)
	if err != nil {
		return nil, err
	}
	framePath, err := s.Files.SaveCharacterImage(frame)
	if err != nil {
		return nil, err
	}
	label := input.Label
	if label == "" {
		label = map[string]string{"person": "视频人物", "product": "视频商品", "scene": "视频场景", "pose": "角色姿势", "action": "角色动作", "camera": "镜头运动", "brand_logo": "品牌 Logo", "video_watermark": "视频水印"}[input.Kind]
	}
	item, err := s.Store.CreateAssetExtraction(jobs.AssetExtraction{UserID: userID, Kind: input.Kind, Label: label, SourceType: sourceType, SourceID: sourceID, SourceAuxID: sourceAuxID, AtSeconds: input.AtSeconds, FramePath: framePath})
	if err != nil {
		_ = os.Remove(framePath)
		return nil, err
	}
	chargeID := ""
	if cfg.Source == "managed" && (input.Kind == "person" || input.Kind == "product" || input.Kind == "scene" || input.Kind == "brand_logo" || input.Kind == "video_watermark") {
		chargeID, err = s.reserveImage(ctx, cfg)
		if err != nil {
			_ = s.Store.FinishAssetExtraction(userID, item.ID, "failed", framePath, "", "", err.Error())
			return nil, err
		}
	}
	go func() {
		slots <- struct{}{}
		defer func() { <-slots }()
		if sourceType == "upload" && s.Files.OwnsPath(path) {
			defer os.Remove(path)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		var err error
		if input.Kind == "person" || input.Kind == "product" {
			err = s.process(ctx, userID, item.ID, input.Kind, label, sourceType, sourceID, path, seconds, input.AtSeconds, cfg, chargeID, framePath)
		} else if input.Kind == "scene" {
			err = s.processScene(ctx, userID, item.ID, path, seconds, input.AtSeconds, cfg, chargeID, framePath)
		} else if input.Kind == "brand_logo" || input.Kind == "video_watermark" {
			err = s.processMark(ctx, userID, item.ID, input.Kind, path, seconds, input.AtSeconds, cfg, chargeID, framePath)
		} else {
			err = s.processReference(ctx, userID, item.ID, input.Kind, path, seconds, input.AtSeconds, framePath)
		}
		if err != nil {
			_ = s.Store.FinishAssetExtraction(userID, item.ID, "failed", framePath, "", "", err.Error())
		}
	}()
	return item, nil
}

func (s *Service) process(ctx context.Context, userID, extractionID, kind, label, sourceType, sourceID, path string, duration, at float64, cfg model.RuntimeConfig, firstChargeID, initialFramePath string) error {
	frames, plan, err := s.planFrames(ctx, userID, extractionID, path, kind, duration, at)
	if err != nil {
		_ = s.Store.FinishUsage(firstChargeID, false)
		return fmt.Errorf("关键帧筛选失败：%w", err)
	}
	views := plan.Views
	if kind == "person" {
		views = []plannedView{{Index: plan.Main, View: "main"}}
	}
	variants := make([]jobs.AssetExtractionVariant, 0, len(views))
	var analysisParts []string
	analysisParts = append(analysisParts, fmt.Sprintf("关键帧选择：%s。视频叠加层：%s。已确认的衣物实体标识：%s。不确定处：%s。", plan.Reason, notesOrNone(plan.Overlays), notesOrNone(plan.GarmentMarks), notesOrNone(plan.Uncertain)))
	createdPaths := []string{}
	defer func() {
		if len(createdPaths) > 0 {
			for _, name := range createdPaths {
				_ = os.Remove(name)
			}
		}
	}()
	for index, view := range views {
		frame := frames[view.Index]
		cleanReference := frame.Image
		if len(plan.Overlays) > 0 {
			cleanReference = removeScreenOverlays(frame.Image, plan.OverlayBoxes)
		}
		prompt, negative := extractionPrompt(kind)
		prompt = assetSystemPrompt(kind) + "\n" + prompt
		prompt += " 当前输出视角：" + viewName(view.View) + "。仅呈现此帧真实可见的主体角度与局部，不补造未出现的背面或细节。"
		prompt += " 视频后期叠加的字幕、水印、平台角标、悬浮文字和贴纸必须全部移除，即使覆盖在衣服胸前，也不得变成商品印花或 Logo。"
		// Never repeat overlay wording in a generation prompt: image models
		// tend to draw the quoted caption or watermark back into the image.
		prompt += " 只保留参考图中清晰贴合商品的实体标识；无法确认的文字或图案不要复制。"
		if len(plan.GarmentMarks) == 0 {
			prompt += " 原帧没有经过确认的实体标识；衣服和商品表面不要新增字母、徽章、商标或图案。"
		}
		var image []byte
		var lastCandidate []byte
		var lastReview imageReview
		var lastErr error
		accepted := false
		maxAttempts := 2
		if view.View == "main" {
			maxAttempts = 3
		}
		for attempt := 0; attempt < maxAttempts; attempt++ {
			chargeID := ""
			if index == 0 && attempt == 0 {
				chargeID = firstChargeID
			} else if cfg.Source == "managed" {
				chargeID, err = s.reserveImage(userctx.WithUser(ctx, userctx.User{ID: userID}), cfg)
				if err != nil {
					lastErr = err
					break
				}
			}
			instruction := prompt
			retryNote := ""
			contextNote := ""
			useDualReference := false
			if attempt > 0 {
				retryNote = retryExtractionInstruction(lastReview)
				if len(lastCandidate) > 0 && len(plan.GarmentMarks) == 0 && lastReview.markOnlyFailure() {
					useDualReference = true
					contextNote = "第一张图片是原始视频帧，是人物与商品事实依据；第二张是上次生成图，只参考其白底构图。"
				}
			}

			instruction = contextNote + instruction + retryNote
			cfg.ImageChargeID = chargeID
			cfg.ImageRefID = extractionID
			if useDualReference {
				image, err = s.Images.EditReferences(ctx, cfg, instruction, negative, "1024*1024", [][]byte{cleanReference, lastCandidate})
			} else {
				image, err = s.Images.EditReference(ctx, cfg, instruction, negative, cleanReference)
			}
			_ = s.Store.FinishUsage(chargeID, err == nil)
			if err != nil {
				lastErr = err
				if ctx.Err() != nil || nonRetryableImageError(err) {
					break
				}
				continue
			}
			image = removeScreenOverlays(image, safeGeneratedOverlayBoxes(plan.OverlayBoxes))
			lastCandidate = image
			review, reviewErr := s.reviewImage(ctx, userID, extractionID, kind, frame.Image, image, plan)
			if reviewErr != nil {
				lastErr = fmt.Errorf("图片质检暂时不可用：%w", reviewErr)
				lastReview = imageReview{}
				continue
			}
			lastErr = nil
			lastReview = review
			if !review.acceptable() {
				continue
			}
			accepted = true
			analysisParts = append(analysisParts, fmt.Sprintf("### %s（%.1f 秒）\n%s", viewName(view.View), frame.AtSeconds, review.Analysis))
			break
		}
		if !accepted {
			if view.View != "main" {
				analysisParts = append(analysisParts, fmt.Sprintf("### %s\n自动质检未通过，已跳过这一附加视角。", viewName(view.View)))
				continue
			}
			if len(lastCandidate) == 0 {
				if lastErr != nil {
					return fmt.Errorf("主视角生成失败：%w", lastErr)
				}
				return errors.New("主视角没有生成可核对的图片")
			}
			frameFile, saveErr := s.Files.SaveCharacterImage(frame.Image)
			if saveErr != nil {
				return saveErr
			}
			createdPaths = append(createdPaths, frameFile)
			imageFile, saveErr := s.Files.SaveCharacterImage(lastCandidate)
			if saveErr != nil {
				return saveErr
			}
			createdPaths = append(createdPaths, imageFile)
			message := reviewFailureSummary(lastReview, lastErr)
			analysis := "## 待人工核对\n" + message + "。请对照原视频帧检查生成图中的人物、商品和文字；确认前不会导入素材库。"
			if strings.TrimSpace(lastReview.Analysis) != "" {
				analysis += "\n\n" + lastReview.Analysis
			}
			variant := jobs.AssetExtractionVariant{View: "main", AtSeconds: frame.AtSeconds, FramePath: frameFile, ImagePath: imageFile}
			if err := s.Store.CompleteAssetExtractionForReview(userID, extractionID, frameFile, imageFile, analysis, message, []jobs.AssetExtractionVariant{variant}); err != nil {
				return err
			}
			createdPaths = nil
			if initialFramePath != frameFile {
				_ = os.Remove(initialFramePath)
			}
			return nil
		}
		frameFile, saveErr := s.Files.SaveCharacterImage(frame.Image)
		if saveErr != nil {
			return saveErr
		}
		createdPaths = append(createdPaths, frameFile)
		imageFile, saveErr := s.Files.SaveCharacterImage(image)
		if saveErr != nil {
			return saveErr
		}
		createdPaths = append(createdPaths, imageFile)
		variants = append(variants, jobs.AssetExtractionVariant{View: view.View, AtSeconds: frame.AtSeconds, FramePath: frameFile, ImagePath: imageFile})
	}
	if len(variants) == 0 {
		return errors.New("没有可用的商品画面")
	}
	targetID := ""
	if kind == "person" {
		metadata, _ := json.Marshal(map[string]any{"extraction_id": extractionID, "source_type": sourceType, "source_id": sourceID, "at_seconds": variants[0].AtSeconds})
		role, createErr := s.Store.CreateCharacter(userID, label, "video_extract", "", "ready", metadata)
		if createErr == nil {
			createErr = s.Store.FinishCharacter(userID, role.ID, variants[0].ImagePath, "")
		}
		if createErr == nil {
			targetID = role.ID
		}
	}
	analysis := strings.Join(analysisParts, "\n\n")
	if utf8.RuneCountInString(analysis) > 5000 {
		analysis = string([]rune(analysis)[:5000])
	}
	if err := s.Store.CompleteAssetExtractionWithVariants(userID, extractionID, variants[0].FramePath, variants[0].ImagePath, targetID, "", analysis, variants); err != nil {
		return err
	}
	createdPaths = nil
	if initialFramePath != variants[0].FramePath {
		_ = os.Remove(initialFramePath)
	}
	return nil
}

func (r imageReview) markOnlyFailure() bool {
	return (r.WatermarkTransferred || r.OverlayUncertain || r.UnsupportedDetails) && !r.ProductMismatch && !r.PersonMismatch
}

func retryExtractionInstruction(review imageReview) string {
	if review.markOnlyFailure() {
		return " 上一次结果出现原帧未证实的标识、文字或细节。只保留原帧可见的人物面貌、服装版型和商品结构；不要添加任何字母、徽章、Logo、字幕或不可见细节。"
	}
	return " 上一次结果与原帧可能不一致。严格保持原帧中的人物面貌和商品颜色、衣长、轮廓、拼接、拉链及口袋，不得换款或补造不可见细节。"
}

func reviewFailureSummary(review imageReview, reviewErr error) string {
	issues := []string{}
	if review.ProductMismatch {
		issues = append(issues, "商品外观可能与原画面不一致")
	}
	if review.PersonMismatch {
		issues = append(issues, "人物外观可能改变")
	}
	if review.WatermarkTransferred || review.OverlayUncertain {
		issues = append(issues, "生成图可能残留字幕或水印")
	}
	if review.UnsupportedDetails {
		issues = append(issues, "生成图可能增加原画面没有的标识或细节")
	}
	if len(issues) == 0 {
		if reviewErr != nil {
			return "生成图已保留，但自动质检或重试未完成"
		}
		return "生成图需要人工核对"
	}
	return strings.Join(issues, "；")
}

func nonRetryableImageError(err error) bool {
	if errors.Is(err, charactergen.ErrAccessDenied) || errors.Is(err, charactergen.ErrModelUnavailable) {
		return true
	}
	message := err.Error()
	for _, fragment := range []string{"API Key 无效", "额度不足", "尚未开通", "账单不可用", "请为角色生成配置", "请配置与 API Key"} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func safeGeneratedOverlayBoxes(boxes []overlayBox) []overlayBox {
	result := make([]overlayBox, 0, len(boxes))
	for _, box := range boxes {
		// A screen watermark in the outer top strip can be erased after
		// generation without touching the garment or the person's face.
		if validTargetBox(box) && box.Y+box.H <= 180 && box.W <= 350 && box.H <= 120 {
			result = append(result, box)
		}
	}
	return result
}

func viewName(view string) string {
	switch view {
	case "side":
		return "侧面图"
	case "back":
		return "背面图"
	case "detail":
		return "细节图"
	default:
		return "主视角白底图"
	}
}

func assetVariantName(kind, view string) string {
	switch kind {
	case "scene":
		return "场景参考图"
	case "pose":
		return "姿势参考图"
	case "action":
		return "动作片段"
	case "camera":
		return "镜头片段"
	case "brand_logo":
		return "品牌Logo裁切"
	case "video_watermark":
		return "视频水印裁切"
	}
	return viewName(view)
}

func notesOrNone(items []string) string {
	if len(items) == 0 {
		return "未确认"
	}
	return strings.Join(items, "；")
}

// ApproveReview records the user's visual decision. Flagged candidates are
// never imported or added to the role library before this explicit action.
func (s *Service) ApproveReview(ctx context.Context, userID, extractionID string) (*jobs.AssetExtraction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	item, err := s.Store.GetAssetExtraction(userID, extractionID)
	if err != nil || item.Status != "needs_review" || item.ImagePath == "" {
		return nil, errors.New("没有可确认的待核对图片")
	}
	if _, err := os.Stat(item.ImagePath); err != nil {
		return nil, errors.New("待核对图片文件不可用")
	}
	if err := s.Store.ApproveAssetExtractionReview(userID, extractionID); err != nil {
		return nil, err
	}
	if item.Kind == "person" {
		metadata, _ := json.Marshal(map[string]any{"extraction_id": extractionID, "source_type": item.SourceType, "source_id": item.SourceID, "at_seconds": item.AtSeconds, "user_reviewed": true})
		role, createErr := s.Store.CreateCharacter(userID, item.Label, "video_extract", "", "ready", metadata)
		if createErr == nil {
			createErr = s.Store.FinishCharacter(userID, role.ID, item.ImagePath, "")
		}
		if createErr == nil {
			_ = s.Store.SetAssetExtractionTarget(userID, extractionID, role.ID)
		}
	}
	return s.Store.GetAssetExtraction(userID, extractionID)
}

func (s *Service) Import(ctx context.Context, userID, extractionID, productID string) (*jobs.AssetExtraction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	importMu.Lock()
	defer importMu.Unlock()
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, errors.New("请选择目标产品")
	}
	if _, err := s.Store.GetUserProduct(userID, productID); err != nil {
		return nil, errors.New("目标产品不存在")
	}
	item, err := s.Store.GetAssetExtraction(userID, extractionID)
	if err != nil {
		return nil, errors.New("提取结果不存在")
	}
	if item.Status != "ready" || item.ImagePath == "" || item.Analysis == "" {
		return nil, errors.New("提取结果尚未完成")
	}
	if item.ProductID != "" {
		if item.ProductID == productID {
			return item, nil
		}
		return nil, errors.New("该资产已导入其他产品")
	}
	variants := item.Variants
	if len(variants) == 0 {
		variants = []jobs.AssetExtractionVariant{{View: "main", AtSeconds: item.AtSeconds, ImagePath: item.ImagePath, FramePath: item.FramePath}}
	}
	assetIDs := make([]string, 0, len(variants))
	copyPaths := make([]string, 0, len(variants))
	committed := false
	defer func() {
		if committed {
			return
		}
		for _, id := range assetIDs {
			_ = s.Store.DeleteProductAsset(id)
		}
		for _, path := range copyPaths {
			_ = os.Remove(path)
		}
	}()
	for index, variant := range variants {
		isClip := (item.Kind == "action" || item.Kind == "camera") && variant.VideoPath != ""
		sourcePath := variant.ImagePath
		if isClip {
			sourcePath = variant.VideoPath
		}
		content, readErr := os.ReadFile(sourcePath)
		if readErr != nil {
			return nil, errors.New("提取资产文件不可用")
		}
		copyPath := ""
		var saveErr error
		if isClip {
			copyPath, saveErr = s.Files.SaveGeneratedVideo(fmt.Sprintf("%s-import-%d-%s", item.ID, index, productID), bytes.NewReader(content))
		} else {
			copyPath, saveErr = s.Files.SaveCharacterImage(content)
		}
		if saveErr != nil {
			return nil, saveErr
		}
		copyPaths = append(copyPaths, copyPath)
		mime := http.DetectContentType(content)
		ext := ".jpg"
		if mime == "image/png" {
			ext = ".png"
		}
		assetKind := "image"
		if isClip {
			assetKind, mime, ext = "video", "video/mp4", ".mp4"
		}
		asset, createErr := s.Store.CreateProductAsset(jobs.ProductAsset{ProductID: productID, Kind: assetKind, Path: copyPath, OriginalName: item.Label + "-" + assetVariantName(item.Kind, variant.View) + ext, MimeType: mime, SizeBytes: int64(len(content))})
		if createErr != nil {
			return nil, createErr
		}
		assetIDs = append(assetIDs, asset.ID)
	}
	section := ProductAssetDescription(*item)
	if err = s.Store.WriteProductObservation(userID, productID, item.ID, section); err != nil {
		return nil, err
	}
	if len(item.Variants) > 0 {
		err = s.Store.SetAssetExtractionVariantAssets(userID, item.ID, productID, assetIDs)
	} else {
		err = s.Store.SetAssetExtractionProduct(userID, item.ID, productID, assetIDs[0])
	}
	if err != nil {
		return nil, err
	}
	committed = true
	return s.Store.GetAssetExtraction(userID, item.ID)
}

func (s *Service) sourcePath(userID string, in Input) (path, sourceType, sourceID, sourceAuxID string, err error) {
	count := 0
	for _, id := range []string{in.SourcePath, in.SourceAssetID, in.SourceVideoID, in.SourceBatchID, in.AttachmentID} {
		if id != "" {
			count++
		}
	}
	if count != 1 {
		return "", "", "", "", errors.New("请选择一条参考视频")
	}
	if in.SourcePath != "" {
		return in.SourcePath, "upload", filepath.Base(in.SourcePath), "", nil
	}
	if in.SourceAssetID != "" {
		asset, e := s.Store.GetProductAsset(in.SourceAssetID)
		if e != nil || asset.Kind != "video" {
			return "", "", "", "", errors.New("参考视频不存在")
		}
		if _, e = s.Store.GetUserProduct(userID, asset.ProductID); e != nil {
			return "", "", "", "", errors.New("参考视频不存在")
		}
		return asset.Path, "product_asset", asset.ID, "", nil
	}
	if in.SourceVideoID != "" {
		video, e := s.Store.GetUserVideoGeneration(userID, in.SourceVideoID)
		if e != nil || video.Status != "completed" || video.LocalPath == "" {
			return "", "", "", "", errors.New("生成视频不存在或尚未完成")
		}
		return video.LocalPath, "generated_video", video.ID, "", nil
	}
	if in.SourceBatchID != "" {
		batch, e := s.Store.GetRenderBatch(userID, in.SourceBatchID)
		if e != nil {
			return "", "", "", "", errors.New("批次视频不存在")
		}
		for _, variant := range batch.Variants {
			if variant.ID == in.SourceVariantID && variant.Status == "completed" && variant.OutputPath != "" {
				return variant.OutputPath, "render_variant", batch.ID, variant.ID, nil
			}
		}
		return "", "", "", "", errors.New("批次成片不存在或尚未完成")
	}
	if in.ConversationID == "" {
		return "", "", "", "", errors.New("聊天附件需要对话 ID")
	}
	if _, e := s.Store.GetUserChatThread(userID, in.ConversationID); e != nil {
		return "", "", "", "", errors.New("参考视频不存在")
	}
	attachment, e := s.Store.GetChatAttachment(in.ConversationID, in.AttachmentID)
	if e != nil || attachment.Kind != "video" {
		return "", "", "", "", errors.New("参考视频不存在")
	}
	return attachment.Path, "chat_attachment", attachment.ID, in.ConversationID, nil
}

func extractionPrompt(kind string) (string, string) {
	if kind == "person" {
		return "严格以输入视频画面中最清晰的单个人物为参考，提取同一人物角色，保留可见的五官、脸型、肤色、发型、服装版型、颜色和配饰。生成真实摄影风格的人物角色白底图，纯白背景、均匀柔光、人物完整居中。若原画面只显示半身，只做半身图，不臆造不可见的身体或服装细节。只保留跨帧证实属于衣服本体的标识；视频后期叠加的文字、水印和平台图标全部移除。", "多人、不同人物、换脸、换装、改色、额外配饰、场景背景、视频字幕、平台水印、肢体畸形"
	}
	return "严格提取视频中同一件实物商品，保留可见的颜色、轮廓、长度比例、裁片拼接、领口、袖口、拉链、口袋和已证实属于商品的图案。若商品是模特身上的衣服，保留整件衣服原有版型，只移除人体和背景；不能把衣服换款、改成短款或另造内搭。若商品不是衣服，不把模特服装或道具误当商品。纯白背景、真实棚拍光线。不可见的背面和细节不得补造。视频字幕、水印、平台图标和悬浮字绝不是商品 Logo。", "另一种商品、衣长改变、版型改变、拼接错误、伪造文字、视频水印、平台角标、字幕、额外商品、人物、手、场景背景"
}

func probeDuration(ctx context.Context, path string) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

func extractFrame(ctx context.Context, path string, at float64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-ss", strconv.FormatFloat(at, 'f', 3, 64), "-i", path, "-frames:v", "1", "-vf", "scale=1024:1024:force_original_aspect_ratio=decrease,pad=1024:1024:(ow-iw)/2:(oh-ih)/2:white", "-q:v", "2", "-f", "image2pipe", "-vcodec", "mjpeg", "pipe:1").Output()
	if err != nil || len(out) == 0 || len(out) > 10<<20 {
		return nil, errors.New("无法读取该时间点的画面，请选择另一帧")
	}
	return out, nil
}

func (s *Service) reserveImage(ctx context.Context, cfg model.RuntimeConfig) (string, error) {
	if cfg.Provider == "zexapi" {
		return s.Store.ReserveImageUsage(ctx, "image_edit", charactergen.Options{PriceVersion: cfg.ImagePriceVersion, Model: cfg.Model, AspectRatio: cfg.ImageAspectRatio, Size: cfg.ImagePixelSize, ImageSize: cfg.ImageSize})
	}
	return s.Store.ReserveUsage(ctx, "image_edit", 1)
}
