package web

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/tian1363/scriptagent/internal/jobs"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/charactergen"
	"github.com/tian1363/scriptagent/internal/userctx"
)

func (h *Handler) listCharacters(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListCharacters(userIDFromRequest(r))
	if err != nil {
		writeError(w, 500, errors.New("角色库读取失败"))
		return
	}
	writeJSON(w, 200, items)
}
func (h *Handler) getCharacterImage(w http.ResponseWriter, r *http.Request) {
	role, err := h.store.GetCharacter(userIDFromRequest(r), chi.URLParam(r, "id"))
	if err != nil || role.Status != "ready" || role.Path == "" {
		writeError(w, 404, errors.New("角色图不存在或尚未就绪"))
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	setImageDownloadHeaders(w, r, role.Name, role.Path)
	http.ServeFile(w, r, role.Path)
}
func (h *Handler) uploadCharacter(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, (10<<20)+(64<<10))
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, 400, errors.New("请选择 10MB 以内的角色图"))
		return
	}
	defer r.MultipartForm.RemoveAll()
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" || utf8.RuneCountInString(name) > 80 {
		writeError(w, 400, errors.New("请填写 1–80 字角色名称"))
		return
	}
	f, _, err := r.FormFile("image")
	if err != nil {
		writeError(w, 400, errors.New("请上传角色图"))
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (10<<20)+1))
	if err != nil {
		writeError(w, 400, errors.New("图片读取失败"))
		return
	}
	path, err := h.files.SaveCharacterImage(data)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	userID := userIDFromRequest(r)
	role, err := h.store.CreateCharacter(userID, name, "upload", "", "ready", nil)
	if err != nil {
		_ = os.Remove(path)
		writeError(w, 400, errors.New("角色保存失败或角色库已满"))
		return
	}
	if err = h.store.FinishCharacter(userID, role.ID, path, ""); err != nil {
		_ = os.Remove(path)
		_ = h.store.FinishCharacter(userID, role.ID, "", "保存失败，请重新上传")
		writeError(w, 500, errors.New("角色保存失败"))
		return
	}
	writeJSON(w, 201, role)
}
func (h *Handler) generateCharacter(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name         string                `json:"name"`
		ReferenceID  string                `json:"reference_id"`
		Controls     charactergen.Controls `json:"controls"`
		ImageOptions charactergen.Options  `json:"image_options"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(&struct{}{}) != io.EOF {
		writeError(w, 400, errors.New("角色参数格式无效"))
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 80 {
		writeError(w, 400, errors.New("请填写 1–80 字角色名称"))
		return
	}
	if err := in.Controls.Validate(); err != nil {
		writeError(w, 400, err)
		return
	}
	userID := userIDFromRequest(r)
	capability := "image_generation"
	var reference []byte
	if in.ReferenceID != "" {
		role, err := h.store.GetCharacter(userID, in.ReferenceID)
		if err != nil || role.Status != "ready" {
			writeError(w, 404, errors.New("参考角色不存在或尚未就绪"))
			return
		}
		reference, err = os.ReadFile(role.Path)
		if err != nil {
			writeError(w, 400, errors.New("参考角色图读取失败"))
			return
		}
		capability = "image_edit"
	}
	cfg, options, err := h.imageRuntime(r, capability, in.ImageOptions)
	if err != nil || cfg.Model == "" {
		writeError(w, 400, errors.New("请先在设置中配置对应的图片生成或图片编辑模型"))
		return
	}
	if cfg.Provider == "zexapi" {
		ratio := map[string]string{"1024*1024": "1:1", "720*1280": "9:16", "960*1280": "3:4"}[in.Controls.Size]
		options = normalizeImageOptions(options, ratio)
		refs := [][]byte{}
		if len(reference) > 0 {
			refs = append(refs, reference)
		}
		prompt := in.Controls.Prompt(len(reference) > 0) + " 避免：多人、模糊、畸形、水印。" + in.Controls.Negative
		if err = charactergen.ValidateInput(options, prompt, refs); err != nil {
			writeError(w, 400, err)
			return
		}
	}
	user, _ := userctx.FromContext(r.Context())
	if cfg.Source == "managed" {
		if !h.cfg.AllowManagedMode {
			writeError(w, 403, errors.New("当前账号不能使用平台图片额度"))
			return
		}
	}
	if cfg.APIKey == "" {
		writeError(w, 400, errors.New("请先连接图片模型 API Key"))
		return
	}
	if !h.authLimit.allow("character:"+userID, 3, 5*time.Minute) {
		writeError(w, 429, errors.New("生成请求较多，请稍后再试"))
		return
	}
	chargeID := ""
	if cfg.Source == "managed" {
		chargeID, err = h.reserveImage(r, capability, cfg, options)
		if err != nil {
			writeError(w, http.StatusPaymentRequired, err)
			return
		}
	}
	controls, _ := json.Marshal(in.Controls)
	source := "ai"
	if len(reference) > 0 {
		source = "edit"
	}
	role, err := h.store.CreateCharacter(userID, in.Name, source, in.ReferenceID, "generating", controls)
	if err != nil {
		_ = h.store.FinishUsage(chargeID, false)
		writeError(w, 400, errors.New("无法创建角色，角色库可能已满"))
		return
	}
	if err := h.store.RecordUsageTasks([]jobs.UsageTask{{UserID: userID, TaskID: role.ID, ChargeID: chargeID, Title: role.Name, Model: cfg.Model, Source: cfg.Source, Capability: capability}}); err != nil {
		_ = h.store.FinishUsage(chargeID, false)
		_ = h.store.FinishCharacter(userID, role.ID, "", "计费记录保存失败")
		writeError(w, 500, err)
		return
	}
	if cfg.Provider == "zexapi" {
		refs := [][]byte{}
		if len(reference) > 0 {
			refs = append(refs, reference)
		}
		prompt := in.Controls.Prompt(len(reference) > 0) + " 避免：多人、模糊、畸形、水印。" + in.Controls.Negative
		err = h.queueImage(jobs.ProviderImageJob{ID: role.ID, UserID: userID, Kind: "character", ChargeID: chargeID, Capability: capability}, cfg, options, prompt, refs)
		if err != nil {
			_ = h.store.FinishUsage(chargeID, false)
			_ = h.store.FinishCharacter(userID, role.ID, "", err.Error())
			writeError(w, 400, err)
			return
		}
		writeJSON(w, http.StatusAccepted, role)
		return
	}
	go func() {
		success := false
		defer func() { _ = h.store.FinishUsage(chargeID, success) }()
		ctx, cancel := context.WithTimeout(userctx.WithUser(context.Background(), user), 4*time.Minute)
		defer cancel()
		data, err := h.characterImages.Generate(ctx, cfg, in.Controls, reference)
		if err != nil {
			_ = h.store.FinishCharacter(userID, role.ID, "", err.Error())
			return
		}
		path, err := h.files.SaveCharacterImage(data)
		if err != nil {
			_ = h.store.FinishCharacter(userID, role.ID, "", err.Error())
			return
		}
		if err = h.store.FinishCharacter(userID, role.ID, path, ""); err != nil {
			_ = os.Remove(path)
			return
		}
		success = true
	}()
	writeJSON(w, http.StatusAccepted, role)
}
