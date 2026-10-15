package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/charactergen"
	"github.com/tian1363/scriptagent/internal/detailpage"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/userctx"
)

type detailPlanner interface {
	GenerateDetailPlan(context.Context, detailpage.Input) (*detailpage.Plan, error)
}

func detailDecode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		writeError(w, 400, errors.New("内容方案格式无效"))
		return false
	}
	return true
}
func (h *Handler) detailOwner(w http.ResponseWriter, r *http.Request) bool {
	if _, err := h.store.GetUserProduct(userIDFromRequest(r), chi.URLParam(r, "id")); err != nil {
		writeError(w, 404, errors.New("产品资料不存在"))
		return false
	}
	return true
}
func (h *Handler) detailAssets(productID string, in *detailpage.Input) error {
	if err := in.Validate(); err != nil {
		return err
	}
	for i, a := range in.Assets {
		asset, err := h.store.GetProductAsset(a.ID)
		if err != nil || asset.ProductID != productID || asset.Kind != "image" {
			return errors.New("商品素材不存在")
		}
		in.Assets[i].Name = asset.OriginalName
	}
	if in.ReferenceID != "" {
		a, err := h.store.GetProductAsset(in.ReferenceID)
		if err != nil || a.ProductID != productID || a.Kind != "reference" {
			return errors.New("视觉参考不存在")
		}
	}
	return nil
}
func (h *Handler) generateDetailPlan(w http.ResponseWriter, r *http.Request) {
	if !h.detailOwner(w, r) {
		return
	}
	var in detailpage.Input
	if !detailDecode(w, r, &in) {
		return
	}
	if err := h.detailAssets(chi.URLParam(r, "id"), &in); err != nil {
		writeError(w, 400, err)
		return
	}
	planner, ok := h.chat.(detailPlanner)
	if !ok {
		writeError(w, 503, errors.New("内容策划暂不可用"))
		return
	}
	if !h.authLimit.allow("detail-plan:"+userIDFromRequest(r), 6, 5*time.Minute) {
		writeError(w, 429, errors.New("策划请求较多，请稍后重试"))
		return
	}
	plan, err := planner.GenerateDetailPlan(r.Context(), in)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	writeJSON(w, 200, plan)
}
func (h *Handler) previewDetailPlan(w http.ResponseWriter, r *http.Request) {
	if !h.detailOwner(w, r) {
		return
	}
	var p detailpage.Plan
	if !detailDecode(w, r, &p) {
		return
	}
	if err := h.detailAssets(chi.URLParam(r, "id"), &p.Input); err != nil {
		writeError(w, 400, err)
		return
	}
	tasks, err := detailpage.Compile(p)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	cfg, _ := h.store.GetModelRuntimeConfig(r.Context(), "image_edit")
	points := h.store.CreditPrices()["image_edit"]
	if cfg.Provider == "zexapi" {
		_, n, e := h.store.ImageQuote(charactergen.Options{Model: cfg.Model})
		if e != nil {
			points = 0
		} else {
			points = n
		}
	}
	writeJSON(w, 200, map[string]any{"tasks": tasks, "scenes": detailpage.Scenes, "source": cfg.Source, "points_per_image": points})
}
func (h *Handler) listDetailImages(w http.ResponseWriter, r *http.Request) {
	if !h.detailOwner(w, r) {
		return
	}
	items, err := h.store.ListDetailImageJobs(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, items)
}
func (h *Handler) startDetailImages(w http.ResponseWriter, r *http.Request) {
	if !h.detailOwner(w, r) {
		return
	}
	userID := userIDFromRequest(r)
	productID := chi.URLParam(r, "id")
	var in struct {
		Plan         detailpage.Plan      `json:"plan"`
		RequestID    string               `json:"request_id"`
		RetryID      string               `json:"retry_job_id"`
		PlatformID   string               `json:"platform_id"`
		PageID       string               `json:"page_id"`
		Confirmed    bool                 `json:"confirmed"`
		ImageOptions charactergen.Options `json:"image_options"`
	}
	if !detailDecode(w, r, &in) {
		return
	}
	if !in.Confirmed {
		writeError(w, 400, errors.New("请先核对方案与费用"))
		return
	}
	var tasks []detailpage.Task
	var err error
	if in.RetryID != "" {
		old, e := h.store.GetDetailImageJob(userID, productID, in.RetryID)
		if e != nil || old.Status != "failed" {
			writeError(w, 400, errors.New("仅能重试本产品的失败任务"))
			return
		}
		tasks = []detailpage.Task{old.Task}
	} else {
		if err = h.detailAssets(productID, &in.Plan.Input); err != nil {
			writeError(w, 400, err)
			return
		}
		tasks, err = detailpage.Compile(in.Plan)
		if err != nil {
			writeError(w, 400, err)
			return
		}
		selected := []detailpage.Task{}
		for _, t := range tasks {
			if t.PlatformID == in.PlatformID && (in.PageID == "" || in.PageID == t.PageID) {
				for _, p := range in.Plan.Pages {
					if (p.ID == t.PageID || t.PageID == "whole" && detailTaskUsesPage(t, p.ID)) && len(p.Missing) > 0 {
						writeError(w, 400, errors.New("请先补充或核对本屏缺项"))
						return
					}
				}
				selected = append(selected, t)
			}
		}
		tasks = selected
	}
	if len(tasks) == 0 || len(tasks) > 5 {
		writeError(w, 400, errors.New("请选择一个平台或单屏生成"))
		return
	}
	images := map[string][]byte{}
	for _, t := range tasks {
		ids := append(append([]string{}, t.ProductAssetIDs()...), t.ReferenceID)
		for i, id := range ids {
			if id == "" {
				continue
			}
			kind := "image"
			if i == len(ids)-1 {
				kind = "reference"
			}
			a, e := h.store.GetProductAsset(id)
			if e != nil || a.ProductID != productID || a.Kind != kind {
				writeError(w, 400, errors.New("参考素材不可用"))
				return
			}
			if _, ok := images[id]; ok {
				continue
			}
			stat, e := os.Stat(a.Path)
			if e != nil || stat.Size() > 10<<20 {
				writeError(w, 400, errors.New("参考素材不可用或超过 10MB"))
				return
			}
			data, e := os.ReadFile(a.Path)
			mime := http.DetectContentType(data)
			if e != nil || len(data) == 0 || (mime != "image/png" && mime != "image/jpeg") {
				writeError(w, 400, errors.New("请使用 JPG 或 PNG 参考图"))
				return
			}
			images[id] = data
		}
	}
	cfg, options, err := h.imageRuntime(r, "image_edit", in.ImageOptions)
	if err != nil || cfg.Model == "" || cfg.APIKey == "" {
		writeError(w, 400, errors.New("请先连接图片编辑模型"))
		return
	}
	if cfg.Provider != "zexapi" {
		for _, task := range tasks {
			if len(detailTaskReferences(task, images)) > 2 {
				writeError(w, 400, errors.New("当前图片接口最多支持两张参考图，请减少商品图数量，或改用支持多图的模型"))
				return
			}
		}
	}
	if cfg.Source == "managed" && !h.cfg.AllowManagedMode {
		writeError(w, 403, errors.New("平台图片额度尚未开放"))
		return
	}
	if cfg.Provider == "zexapi" {
		options = normalizeImageOptions(options, "auto")
		for _, task := range tasks {
			refs := detailTaskReferences(task, images)
			if err = charactergen.ValidateInput(options, task.Prompt, refs); err != nil {
				writeError(w, 400, err)
				return
			}
		}
	}
	raw, _ := json.Marshal(in.Plan)
	items, fresh, err := h.store.CreateDetailImageBatch(userID, productID, in.RequestID, string(raw), tasks)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	if !fresh {
		for _, item := range items {
			linkAnalyticsTasks(r, item.ID)
		}
		writeJSON(w, 200, items)
		return
	}
	charges := make([]string, len(items))
	fail := func(message string) {
		for i, j := range items {
			_ = h.store.FinishUsage(charges[i], false)
			_ = h.store.UpdateDetailImageJob(userID, j.ID, "failed", "", message)
		}
	}
	if !h.authLimit.allow("detail-image:"+userID, 6, 5*time.Minute) {
		fail("请求较多，请稍后单张重试")
		writeError(w, 429, errors.New("生成请求较多，请稍后重试"))
		return
	}
	usage := []jobs.UsageTask{}
	for i, j := range items {
		if cfg.Source == "managed" {
			charges[i], err = h.reserveImage(r, "image_edit", cfg, options)
			if err != nil {
				fail("额度不足，未调用图片模型")
				writeError(w, 402, err)
				return
			}
		}
		usage = append(usage, jobs.UsageTask{UserID: userID, TaskID: j.ID, ChargeID: charges[i], Title: j.Task.PlatformName + " · " + j.Task.Headline, ProductID: productID, Model: cfg.Model, Source: cfg.Source, Capability: "image_edit"})
	}
	if err = h.store.RecordUsageTasks(usage); err != nil {
		fail("计费记录保存失败")
		writeError(w, 500, err)
		return
	}
	linkAnalyticsUsage(r, usage)
	if cfg.Provider == "zexapi" {
		for i, j := range items {
			refs := detailTaskReferences(j.Task, images)
			err = h.queueImage(jobs.ProviderImageJob{ID: j.ID, UserID: userID, Kind: "detail", ProductID: productID, ChargeID: charges[i], Capability: "image_edit"}, cfg, options, j.Task.Prompt, refs)
			if err != nil {
				for k := i; k < len(items); k++ {
					_ = h.store.FinishUsage(charges[k], false)
					_ = h.store.UpdateDetailImageJob(userID, items[k].ID, "failed", "", err.Error())
				}
				writeError(w, 500, err)
				return
			}
		}
		writeJSON(w, 202, items)
		return
	}
	user, _ := userctx.FromContext(r.Context())
	go func() {
		for i, j := range items {
			func() {
				success := false
				defer func() { _ = h.store.FinishUsage(charges[i], success) }()
				if h.store.UpdateDetailImageJob(userID, j.ID, "generating", "", "") != nil {
					return
				}
				ctx, cancel := context.WithTimeout(userctx.WithUser(context.Background(), user), 4*time.Minute)
				defer cancel()
				refs := detailTaskReferences(j.Task, images)
				data, e := h.characterImages.EditDetailReferences(ctx, cfg, j.Task.Prompt, detailpage.RenderSize(j.Task), refs)
				if e != nil {
					_ = h.store.UpdateDetailImageJob(userID, j.ID, "failed", "", e.Error())
					return
				}
				path, e := h.files.SaveProductImage(data)
				if e != nil {
					_ = h.store.UpdateDetailImageJob(userID, j.ID, "failed", "", "图片保存失败")
					return
				}
				asset, e := h.store.CreateProductAsset(jobs.ProductAsset{ProductID: productID, Kind: "artwork", Path: path, OriginalName: j.Task.PlatformName + "-" + j.Task.PageID + filepath.Ext(path), MimeType: http.DetectContentType(data), SizeBytes: int64(len(data))})
				if e != nil {
					_ = os.Remove(path)
					_ = h.store.UpdateDetailImageJob(userID, j.ID, "failed", "", "作品保存失败")
					return
				}
				if h.store.UpdateDetailImageJob(userID, j.ID, "ready", asset.ID, "") != nil {
					_ = h.store.DeleteProductAsset(asset.ID)
					_ = os.Remove(path)
					return
				}
				success = true
			}()
		}
	}()
	writeJSON(w, 202, items)
}

func detailTaskReferences(t detailpage.Task, images map[string][]byte) [][]byte {
	refs := [][]byte{}
	for _, id := range t.ProductAssetIDs() {
		refs = append(refs, images[id])
	}
	if t.ReferenceID != "" {
		refs = append(refs, images[t.ReferenceID])
	}
	return refs
}
func detailTaskUsesPage(t detailpage.Task, id string) bool {
	for _, platform := range detailpage.Platforms {
		if platform.ID == t.PlatformID {
			for _, section := range platform.Sections {
				if section == id {
					return true
				}
			}
		}
	}
	return false
}
