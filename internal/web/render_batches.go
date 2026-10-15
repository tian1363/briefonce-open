package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/jobs"
)

type renderVariantInput struct {
	Title         string                `json:"title"`
	Copy          string                `json:"copy"`
	Segments      []jobs.CaptionSegment `json:"segments"`
	CoverExisting bool                  `json:"cover_existing"`
}

var renderSlots = make(chan struct{}, 2)

func (h *Handler) listRenderBatches(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListRenderBatches(userIDFromRequest(r))
	if err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, items)
}

func (h *Handler) getRenderBatch(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.GetRenderBatch(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 404, errors.New("批次不存在"))
		return
	}
	writeJSON(w, 200, b)
}

func (h *Handler) createRenderBatch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, 400, err)
		return
	}
	userID := userIDFromRequest(r)
	productID, sourceID, sourceVideoID := r.FormValue("product_id"), r.FormValue("source_asset_id"), r.FormValue("source_video_id")
	sourceBatchID, sourceVariantID := r.FormValue("source_batch_id"), r.FormValue("source_variant_id")
	if _, err := h.store.GetUserProduct(userID, productID); err != nil {
		writeError(w, 400, errors.New("请选择自己的产品资料"))
		return
	}
	sourcePath, err := h.renderSourcePath(userID, productID, sourceID, sourceVideoID, sourceBatchID, sourceVariantID)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	var inputs []renderVariantInput
	if err := json.Unmarshal([]byte(r.FormValue("variants")), &inputs); err != nil || len(inputs) < 1 || len(inputs) > 5 {
		writeError(w, 400, errors.New("首版每批支持 1 至 5 个字幕版本"))
		return
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		writeError(w, 503, errors.New("服务器缺少 FFmpeg，暂时无法合成"))
		return
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		writeError(w, 503, errors.New("服务器缺少 ffprobe，暂时无法检查原片"))
		return
	}
	seconds, err := probeRenderDuration(r.Context(), sourcePath)
	if err != nil || seconds <= 0 || seconds > 30 {
		writeError(w, 400, errors.New("原视频需可读取且时长不超过 30 秒"))
		return
	}
	variants := make([]jobs.RenderVariant, 0, len(inputs))
	var saved []string
	defer func() {
		if saved != nil {
			for _, path := range saved {
				_ = os.Remove(path)
			}
		}
	}()
	for i, input := range inputs {
		input.Title = strings.TrimSpace(input.Title)
		input.Copy = strings.TrimSpace(input.Copy)
		if len(input.Segments) == 0 && (input.Copy == "" || len([]rune(input.Copy)) > 60) {
			writeError(w, 400, errors.New("每版字幕需为 1 至 60 字"))
			return
		}
		variant := jobs.RenderVariant{Title: input.Title, Copy: input.Copy, CoverExisting: input.CoverExisting}
		if len(input.Segments) > 0 {
			if len(input.Segments) > 20 {
				writeError(w, 400, errors.New("每版最多 20 句字幕"))
				return
			}
			lastEnd := 0.0
			for j, segment := range input.Segments {
				segment.Text = strings.TrimSpace(segment.Text)
				if segment.Text == "" || len([]rune(segment.Text)) > 60 || segment.Start < lastEnd || segment.End <= segment.Start || segment.End > seconds+0.05 {
					writeError(w, 400, errors.New("字幕时间或文案无效，请检查逐句时间轴"))
					return
				}
				path, err := h.saveCaptionOverlay(r, fmt.Sprintf("overlay_%d_%d", i, j))
				if err != nil {
					writeError(w, 400, err)
					return
				}
				saved = append(saved, path)
				segment.OverlayPath = path
				variant.Segments = append(variant.Segments, segment)
				lastEnd = segment.End
			}
			variant.Copy = "逐句字幕"
		} else {
			path, err := h.saveCaptionOverlay(r, fmt.Sprintf("overlay_%d", i))
			if err != nil {
				writeError(w, 400, err)
				return
			}
			saved = append(saved, path)
			variant.OverlayPath = path
		}
		variants = append(variants, variant)
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = "广告视频批次"
	}
	if len([]rune(title)) > 80 {
		writeError(w, 400, errors.New("批次名称过长"))
		return
	}
	b, err := h.store.CreateRenderBatchWithSource(userID, productID, sourceID, sourceVideoID, sourceBatchID, sourceVariantID, title, variants)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	saved = nil
	go h.runRenderBatch(userID, b.ID)
	writeJSON(w, http.StatusAccepted, b)
}

func (h *Handler) saveCaptionOverlay(r *http.Request, field string) (string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		return "", errors.New("缺少字幕图层")
	}
	defer file.Close()
	if header.Size <= 0 || header.Size > 2<<20 || strings.ToLower(filepath.Ext(header.Filename)) != ".png" {
		return "", errors.New("字幕图层需为不超过 2MB 的 PNG")
	}
	path, err := h.files.SaveUpload(file, header, "asset")
	if err != nil {
		return "", err
	}
	imageFile, err := os.Open(path)
	if err != nil {
		_ = os.Remove(path)
		return "", errors.New("无法读取字幕图层")
	}
	config, decodeErr := png.DecodeConfig(imageFile)
	imageFile.Close()
	if decodeErr != nil || config.Width != 720 || config.Height != 1280 {
		_ = os.Remove(path)
		return "", errors.New("字幕图层需为 720×1280 PNG")
	}
	return path, nil
}

func (h *Handler) renderSourcePath(userID, productID, sourceID, sourceVideoID, sourceBatchID, sourceVariantID string) (string, error) {
	count := 0
	if sourceID != "" {
		count++
	}
	if sourceVideoID != "" {
		count++
	}
	if sourceBatchID != "" && sourceVariantID != "" {
		count++
	}
	if count != 1 || (sourceBatchID == "") != (sourceVariantID == "") {
		return "", errors.New("请选择一条原视频")
	}
	if sourceBatchID != "" {
		batch, err := h.store.GetRenderBatch(userID, sourceBatchID)
		if err != nil {
			return "", errors.New("所选批次成片不存在")
		}
		for _, variant := range batch.Variants {
			if variant.ID == sourceVariantID && variant.Status == "completed" && variant.OutputPath != "" {
				if _, err := os.Stat(variant.OutputPath); err == nil {
					return variant.OutputPath, nil
				}
			}
		}
		return "", errors.New("所选批次成片不可用")
	}
	if sourceVideoID != "" {
		video, err := h.store.GetUserVideoGeneration(userID, sourceVideoID)
		if err != nil || video.Status != "completed" || video.LocalPath == "" {
			return "", errors.New("所选生成视频不存在或尚未完成")
		}
		if _, err := os.Stat(video.LocalPath); err != nil {
			return "", errors.New("所选生成视频文件不可用")
		}
		return video.LocalPath, nil
	}
	asset, err := h.store.GetProductAsset(sourceID)
	if err != nil || asset.ProductID != productID || asset.Kind != "video" {
		return "", errors.New("请选择该产品的原视频")
	}
	return asset.Path, nil
}

func probeRenderDuration(ctx context.Context, path string) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

func (h *Handler) runRenderBatch(userID, batchID string) {
	b, err := h.store.GetRenderBatch(userID, batchID)
	if err != nil {
		return
	}
	sourcePath, err := h.renderSourcePath(userID, b.ProductID, b.SourceAssetID, b.SourceVideoID, b.SourceBatchID, b.SourceVariantID)
	if err != nil {
		for _, v := range b.Variants {
			if v.Status == "pending" || v.Status == "running" {
				_ = h.store.SetRenderVariant(v.ID, "failed", "", "原视频已不可用")
			}
		}
		_ = h.store.SetRenderBatchStatus(batchID, "needs_review")
		return
	}
	_ = h.store.SetRenderBatchStatus(batchID, "running")
	for _, v := range b.Variants {
		if v.Status == "pending" || v.Status == "running" {
			h.runRenderVariant(sourcePath, v)
		}
	}
	h.updateRenderBatchStatus(userID, batchID)
}

func (h *Handler) runRenderVariant(source string, v jobs.RenderVariant) {
	renderSlots <- struct{}{}
	defer func() { <-renderSlots }()
	_ = h.store.SetRenderVariant(v.ID, "running", "", "")
	outDir := filepath.Join(h.cfg.UploadDir, "generated", "batches")
	if err := os.MkdirAll(outDir, 0700); err != nil {
		_ = h.store.SetRenderVariant(v.ID, "failed", "", err.Error())
		return
	}
	path := filepath.Join(outDir, v.ID+".mp4")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var err error
	if len(v.Segments) > 0 {
		err = renderTimedCaptionVideo(ctx, source, v.Segments, v.CoverExisting, path)
	} else {
		err = renderCaptionVideo(ctx, source, v.OverlayPath, path)
	}
	if err != nil {
		_ = os.Remove(path)
		_ = h.store.SetRenderVariant(v.ID, "failed", "", err.Error())
		return
	}
	_ = h.store.SetRenderVariant(v.ID, "completed", path, "")
}

func renderTimedCaptionVideo(ctx context.Context, source string, segments []jobs.CaptionSegment, coverExisting bool, path string) error {
	base := "[0:v]scale=720:1280:force_original_aspect_ratio=decrease,pad=720:1280:(ow-iw)/2:(oh-ih)/2"
	if coverExisting {
		base += ",drawbox=x=0:y=990:w=720:h=290:color=black:t=fill"
	}
	base += "[base]"
	filters := []string{base}
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", source}
	previous := "base"
	for i, segment := range segments {
		args = append(args, "-loop", "1", "-i", segment.OverlayPath)
		next := fmt.Sprintf("caption%d", i)
		filters = append(filters, fmt.Sprintf("[%s][%d:v]overlay=0:0:enable='between(t,%.3f,%.3f)':eof_action=pass:format=auto[%s]", previous, i+1, segment.Start, segment.End, next))
		previous = next
	}
	args = append(args, "-filter_complex", strings.Join(filters, ";"), "-map", "["+previous+"]", "-map", "0:a?", "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-movflags", "+faststart", "-shortest", path)
	output, err := exec.CommandContext(ctx, "ffmpeg", args...).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if len(message) > 500 {
			message = message[:500]
		}
		if message == "" {
			message = err.Error()
		}
		return errors.New(message)
	}
	return nil
}

func renderCaptionVideo(ctx context.Context, source, overlay, path string) error {
	filter := "[0:v]scale=720:1280:force_original_aspect_ratio=decrease,pad=720:1280:(ow-iw)/2:(oh-ih)/2[base];[base][1:v]overlay=0:0:shortest=1:format=auto[v]"
	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", source, "-loop", "1", "-i", overlay, "-filter_complex", filter, "-map", "[v]", "-map", "0:a?", "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-movflags", "+faststart", "-shortest", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(output))
		if len(message) > 500 {
			message = message[:500]
		}
		if message == "" {
			message = err.Error()
		}
		return errors.New(message)
	}
	return nil
}

func (h *Handler) updateRenderBatchStatus(userID, batchID string) {
	b, err := h.store.GetRenderBatch(userID, batchID)
	if err != nil {
		return
	}
	failed, running := false, false
	for _, v := range b.Variants {
		if v.Status == "failed" {
			failed = true
		}
		if v.Status == "pending" || v.Status == "running" {
			running = true
		}
	}
	status := "completed"
	if running {
		status = "running"
	} else if failed {
		status = "needs_review"
	}
	_ = h.store.SetRenderBatchStatus(batchID, status)
}

func (h *Handler) getRenderVariantFile(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.GetRenderBatch(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 404, errors.New("批次不存在"))
		return
	}
	for _, v := range b.Variants {
		if v.ID == chi.URLParam(r, "variantID") && v.Status == "completed" && v.OutputPath != "" {
			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="ad-%s.mp4"`, v.ID))
			http.ServeFile(w, r, v.OutputPath)
			return
		}
	}
	writeError(w, 404, errors.New("成片尚未完成"))
}

func (h *Handler) retryRenderVariant(w http.ResponseWriter, r *http.Request) {
	userID, batchID := userIDFromRequest(r), chi.URLParam(r, "id")
	v, err := h.store.RetryRenderVariant(userID, batchID, chi.URLParam(r, "variantID"))
	if err != nil {
		writeError(w, 400, err)
		return
	}
	go h.runRenderBatch(userID, batchID)
	writeJSON(w, http.StatusAccepted, v)
}

func (h *Handler) ResumeRenderBatches() {
	users, err := h.store.ListUsers()
	if err != nil {
		return
	}
	for _, user := range users {
		ids, err := h.store.ListUnfinishedRenderBatchIDs(user.ID)
		if err != nil {
			continue
		}
		for _, id := range ids {
			go h.runRenderBatch(user.ID, id)
		}
	}
}
