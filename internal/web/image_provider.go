package web

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/charactergen"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func (h *Handler) imageRuntime(r *http.Request, capability string, o charactergen.Options) (model.RuntimeConfig, charactergen.Options, error) {
	cfg, err := h.store.GetModelRuntimeConfig(r.Context(), capability)
	if err != nil {
		return cfg, o, err
	}
	if cfg.Provider != "zexapi" {
		if o.Model != "" || o.AspectRatio != "" || o.Size != "" || o.ImageSize != "" {
			return cfg, o, errors.New("当前图片能力尚未切换至章鱼哥接口")
		}
		return cfg, o, nil
	}
	if o.Model == "" {
		o.Model = cfg.Model
	}
	if err = o.Validate(); err != nil {
		return cfg, o, err
	}
	if _, _, err = h.store.ImageQuote(o); err != nil {
		return cfg, o, err
	}
	cfg.Model = o.Model
	return cfg, o, nil
}
func (h *Handler) reserveImage(r *http.Request, capability string, cfg model.RuntimeConfig, o charactergen.Options) (string, error) {
	if cfg.Source != "managed" {
		return "", nil
	}
	if cfg.Provider == "zexapi" {
		return h.store.ReserveImageUsage(r.Context(), capability, o)
	}
	return h.store.ReserveUsage(r.Context(), capability, 1)
}
func normalizeImageOptions(o charactergen.Options, ratio string) charactergen.Options {
	if o.AspectRatio == "" && o.Size == "" {
		o.AspectRatio = ratio
	}
	if o.Model == "nano-banana-2.1" && o.ImageSize == "" {
		o.ImageSize = "1K"
	}
	return o
}
func (h *Handler) queueImage(j jobs.ProviderImageJob, cfg model.RuntimeConfig, o charactergen.Options, prompt string, refs [][]byte) error {
	body, err := charactergen.RequestBody(o, prompt, refs)
	if err != nil {
		return err
	}
	if err = h.store.QueueProviderImage(j, cfg, body, o); err != nil {
		return err
	}
	return nil
}

// One server-side worker owns provider polling. Persistent submitting is ambiguous
// after a process crash: never submit it again without operator reconciliation.
func (h *Handler) ResumeImages() {
	h.imageWorkerOnce.Do(func() {
		go func() {
			items, err := h.store.PendingProviderImages()
			if err == nil {
				for _, j := range items {
					if j.Status == "submitting" && j.UpstreamID == "" {
						_ = h.store.UpdateProviderImage(j.ID, "uncertain", "", 0, "", "提交中断，需核对供应商是否受理")
					}
				}
			}
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				h.processProviderImages()
				<-ticker.C
			}
		}()
	})
}
func (h *Handler) processProviderImages() {
	items, err := h.store.PendingProviderImages()
	if err != nil {
		return
	}
	for _, j := range items {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
			defer cancel()
			if j.Status == "pending" {
				if err = h.store.UpdateProviderImage(j.ID, "submitting", "", 0, "", ""); err != nil {
					return
				}
				result, callErr := h.characterImages.ZexRequest(ctx, j.Endpoint, j.Credential, "", []byte(j.RequestJSON))
				_ = h.store.RecordProviderRequest(j.ID, "submit", result.HTTPStatus, result.RequestID, safeCallError(callErr))
				if callErr != nil {
					if result.HTTPStatus >= 400 && result.HTTPStatus < 500 && result.HTTPStatus != 408 {
						h.failProviderImage(j, safeCallError(callErr))
					} else {
						_ = h.store.UpdateProviderImage(j.ID, "uncertain", "", 0, "", safeCallError(callErr))
					}
					return
				}
				if result.Task.ID == "" {
					_ = h.store.UpdateProviderImage(j.ID, "uncertain", "", 0, "", "供应商未返回任务 ID，请核对账单")
					return
				}
				j.UpstreamID = result.Task.ID
				j.Status = result.Task.Status
				if j.Status == "completed" {
					j.Status = "saving"
				}
				if j.Status == "" {
					j.Status = "queued"
				}
				_ = h.store.UpdateProviderImage(j.ID, j.Status, j.UpstreamID, result.Task.Progress, result.Task.ResultURL(), "")
				if j.Status == "failed" {
					h.failProviderImage(j, "供应商图片任务失败")
					return
				}
				if j.Status != "saving" {
					return
				}
				j.ResultURL = result.Task.ResultURL()
				j.Status = "saving"
				_ = h.store.UpdateProviderImage(j.ID, "saving", j.UpstreamID, 100, j.ResultURL, "")
			} else if j.Status != "saving" {
				result, callErr := h.characterImages.ZexRequest(ctx, j.Endpoint, j.Credential, j.UpstreamID, nil)
				_ = h.store.RecordProviderRequest(j.ID, "query", result.HTTPStatus, result.RequestID, safeCallError(callErr))
				if callErr != nil {
					_ = h.store.UpdateProviderImage(j.ID, j.Status, j.UpstreamID, j.Progress, "", safeCallError(callErr))
					return
				}
				if result.Task.Status == "failed" {
					h.failProviderImage(j, "供应商图片任务失败，请在后台核对成本")
					return
				}
				if result.Task.Status != "completed" {
					status := result.Task.Status
					if status != "queued" && status != "in_progress" {
						status = "in_progress"
					}
					_ = h.store.UpdateProviderImage(j.ID, status, j.UpstreamID, result.Task.Progress, "", "")
					return
				}
				j.ResultURL = result.Task.ResultURL()
				j.Status = "saving"
				_ = h.store.UpdateProviderImage(j.ID, "saving", j.UpstreamID, 100, j.ResultURL, "")
			}
			if j.ResultURL == "" {
				_ = h.store.UpdateProviderImage(j.ID, "saving", j.UpstreamID, 100, "", "已生成但缺少结果地址，需要核对")
				return
			}
			if j.SavedPath == "" {
				data, downloadErr := h.imageDownload(ctx, j.ResultURL)
				_ = h.store.RecordProviderRequest(j.ID, "download", 0, "", safeCallError(downloadErr))
				if downloadErr != nil {
					_ = h.store.UpdateProviderImage(j.ID, "saving", j.UpstreamID, 100, "", safeCallError(downloadErr))
					return
				}
				var saveErr error
				if j.Kind == "transform" || j.Kind == "detail" {
					j.SavedPath, saveErr = h.files.SaveProductImage(data)
				} else {
					j.SavedPath, saveErr = h.files.SaveCharacterImage(data)
				}
				if saveErr != nil {
					_ = h.store.UpdateProviderImage(j.ID, "saving", j.UpstreamID, 100, "", saveErr.Error())
					return
				}
				if err = h.store.SaveProviderImagePath(j.ID, j.SavedPath, ""); err != nil {
					_ = os.Remove(j.SavedPath)
					return
				}
			}
			if j.ProductID != "" && j.AssetID == "" {
				info, statErr := os.Stat(j.SavedPath)
				if statErr != nil {
					return
				}
				kind := "image"
				if j.Kind == "detail" {
					kind = "artwork"
				}
				asset, saveErr := h.store.CreateProviderImageAsset(j.ID, jobs.ProductAsset{ProductID: j.ProductID, Kind: kind, Path: j.SavedPath, OriginalName: "生成图片-" + j.ID + filepath.Ext(j.SavedPath), MimeType: imageMime(j.SavedPath), SizeBytes: info.Size()})
				if saveErr != nil {
					return
				}
				j.AssetID = asset.ID
			}
			if j.Kind == "transform" {
				err = h.store.FinishImageTransformJob(j.UserID, j.ID, j.AssetID, "")
			} else if j.Kind == "detail" {
				err = h.store.UpdateDetailImageJob(j.UserID, j.ID, "ready", j.AssetID, "")
			} else if j.Kind != "generated_only" {
				err = h.store.FinishCharacter(j.UserID, j.ID, j.SavedPath, "")
			}
			if err != nil {
				return
			}
			if err = h.store.FinishUsage(j.ChargeID, true); err != nil {
				return
			}
			_ = h.store.UpdateProviderImage(j.ID, "completed", j.UpstreamID, 100, "", "")
		}()
	}
}
func safeCallError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
func imageMime(path string) string {
	if filepath.Ext(path) == "png" || filepath.Ext(path) == ".png" {
		return "image/png"
	}
	if filepath.Ext(path) == ".webp" {
		return "image/webp"
	}
	return "image/jpeg"
}
func (h *Handler) failProviderImage(j jobs.ProviderImageJob, message string) {
	if j.Kind == "transform" {
		_ = h.store.FinishImageTransformJob(j.UserID, j.ID, "", message)
	} else if j.Kind == "detail" {
		_ = h.store.UpdateDetailImageJob(j.UserID, j.ID, "failed", "", message)
	} else if j.Kind != "generated_only" {
		_ = h.store.FinishCharacter(j.UserID, j.ID, "", message)
	}
	_ = h.store.UpdateProviderImage(j.ID, "failed", j.UpstreamID, 0, "", message)
	_ = h.store.FinishUsage(j.ChargeID, false)
}
func (h *Handler) imageCatalog(w http.ResponseWriter, r *http.Request) {
	prices, err := h.store.ImagePrices()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	models := []map[string]any{}
	for _, base := range charactergen.ImageModels {
		m, _ := charactergen.FindModel(base.ID)
		tiers := []map[string]any{}
		for _, p := range prices {
			if p.Model == m.ID {
				if n, e := p.Points(); e == nil {
					tiers = append(tiers, map[string]any{"tier": p.Tier, "points": n, "version": p.Version})
				}
			}
		}
		models = append(models, map[string]any{"model": m, "quotes": tiers})
	}
	defaults := map[string]any{}
	for _, cap := range []string{"image_generation", "image_edit"} {
		cfg, e := h.store.GetModelRuntimeConfig(r.Context(), cap)
		if e == nil {
			defaults[cap] = map[string]string{"provider": cfg.Provider, "model": cfg.Model, "source": cfg.Source}
		}
	}
	writeJSON(w, 200, map[string]any{"models": models, "pixel_sizes": charactergen.PixelSizes, "defaults": defaults})
}
func (h *Handler) imageQuote(w http.ResponseWriter, r *http.Request) {
	var o charactergen.Options
	if !decodeOps(w, r, &o) {
		return
	}
	p, n, err := h.store.ImageQuote(o)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"points": n, "version": p.Version, "model": p.Model, "tier": p.Tier})
}
func (h *Handler) myProviderImages(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.UserProviderImages(userIDFromRequest(r))
	if err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, records)
}
func (h *Handler) myProviderImage(w http.ResponseWriter, r *http.Request) {
	status, path, _, err := h.store.ProviderImageResult(chi.URLParam(r, "id"), userIDFromRequest(r))
	if err != nil || status != "completed" || path == "" {
		writeError(w, 404, errors.New("图片不存在或尚未保存"))
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeFile(w, r, path)
}

// Extraction workflows can wait for a durable image task without owning its polling.
func (h *Handler) generateGatewayImage(ctx context.Context, cfg model.RuntimeConfig, prompt, negative, size string, seed int64, refs [][]byte) ([]byte, error) {
	o := charactergen.Options{PriceVersion: cfg.ImagePriceVersion, Model: cfg.Model, AspectRatio: cfg.ImageAspectRatio, ImageSize: cfg.ImageSize, Size: cfg.ImagePixelSize}
	o = normalizeImageOptions(o, "1:1")
	if negative != "" {
		prompt += " 避免：" + negative
	}
	userID := cfg.ImageUserID
	if userID == "" {
		return nil, errors.New("图片任务缺少账号关联")
	}
	id := fmt.Sprintf("image-%d", time.Now().UnixNano())
	if err := h.queueImage(jobs.ProviderImageJob{ID: id, UserID: userID, Kind: "generated_only", ChargeID: cfg.ImageChargeID, Capability: "image_edit"}, cfg, o, prompt, refs); err != nil {
		return nil, err
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		status, path, message, err := h.store.ProviderImageResult(id, userID)
		if err != nil {
			return nil, err
		}
		if status == "completed" {
			return os.ReadFile(path)
		}
		if status == "failed" || status == "uncertain" {
			return nil, errors.New(message)
		}
		select {
		case <-ctx.Done():
			return nil, errors.New("图片任务仍在生成，可在任务历史查询结果")
		case <-ticker.C:
		}
	}
}
