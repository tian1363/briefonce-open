package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/jobs"
)

func (h *Handler) listManagedAccounts(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	credits, err := h.store.ListAccountCredits()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	lookup := map[string]jobs.AccountCredit{}
	for _, credit := range credits {
		lookup[credit.UserID] = credit
	}
	usage, err := h.store.ListDailyCreditUsage()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	items := make([]map[string]any, 0, len(users))
	for _, user := range users {
		items = append(items, map[string]any{"user": user, "credit": lookup[user.ID], "daily_used": usage[user.ID]})
	}
	writeJSON(w, 200, map[string]any{"accounts": items, "prices": h.store.CreditPrices()})
}

func (h *Handler) createManagedAccount(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ModelMode == "mock" {
		writeError(w, 503, errors.New("当前为模拟运行，请切换到真实模型模式后创建试用账号"))
		return
	}
	key, keyErr := h.store.ManagedAPIKey()
	ready := key != ""
	profiles, profileErr := h.store.ListPublicPlatformModelProfiles()
	for _, p := range profiles {
		ready = ready || p.Configured
	}
	if !h.cfg.AllowManagedMode || keyErr != nil || profileErr != nil || !ready {
		writeError(w, 503, errors.New("请先在运营后台配置平台模型 Key"))
		return
	}
	var input struct {
		Email      string `json:"email"`
		Name       string `json:"name"`
		TrialDays  int    `json:"trial_days"`
		Credits    int64  `json:"credits"`
		DailyLimit int64  `json:"daily_limit"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, err)
		return
	}
	user, password, err := h.auth.ProvisionManagedAccount(input.Email, input.Name, input.TrialDays, input.Credits, input.DailyLimit, userIDFromRequest(r))
	if err != nil {
		writeError(w, 400, err)
		return
	}
	credit, _ := h.store.GetAccountCredit(user.ID)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 201, map[string]any{"user": user, "credit": credit, "temporary_password": password})
}

func (h *Handler) getPlatformProvider(w http.ResponseWriter, r *http.Request) {
	key, err := h.store.ManagedAPIKey()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	source, err := h.store.ManagedAPIKeySource()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	profiles, err := h.store.ListPublicPlatformModelProfiles()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{
		"configured": h.cfg.AllowManagedMode && key != "",
		"key_saved":  source == "database",
		"key_source": source,
		"mode":       h.cfg.ModelMode,
		"profiles":   profiles,
	})
}

func (h *Handler) savePlatformModelProfile(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.AllowManagedMode {
		writeError(w, 503, errors.New("请先在服务器启用平台托管模式"))
		return
	}
	var input jobs.PlatformModelInput
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, err)
		return
	}
	input.Capability = chi.URLParam(r, "capability")
	if err := h.store.SavePlatformModelProfile(input); err != nil {
		writeError(w, 400, err)
		return
	}
	h.getPlatformProvider(w, r)
}

func (h *Handler) savePlatformProvider(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.AllowManagedMode {
		writeError(w, 503, errors.New("请先在服务器启用平台托管模式"))
		return
	}
	var input struct {
		APIKey string `json:"api_key"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, err)
		return
	}
	if err := h.store.SetManagedAPIKey(strings.TrimSpace(input.APIKey)); err != nil {
		writeError(w, 400, err)
		return
	}
	h.getPlatformProvider(w, r)
}

func (h *Handler) updateManagedAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.store.GetUser(id)
	if err != nil || user.Role != "member" {
		writeError(w, 404, errors.New("普通账号不存在"))
		return
	}
	var input struct {
		TrialDays  *int   `json:"trial_days"`
		DailyLimit *int64 `json:"daily_limit"`
		Status     string `json:"status"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, err)
		return
	}
	if input.Status != "" && input.Status != "active" && input.Status != "suspended" {
		writeError(w, 400, errors.New("账号状态无效"))
		return
	}
	if input.TrialDays != nil || input.DailyLimit != nil {
		credit, err := h.store.GetAccountCredit(id)
		if err != nil || !credit.ManagedEnabled {
			writeError(w, 400, errors.New("该账号未开通平台积分"))
			return
		}
		if input.TrialDays != nil {
			if *input.TrialDays < 1 || *input.TrialDays > 365 {
				writeError(w, 400, errors.New("试用期限需为 1 到 365 天"))
				return
			}
			credit.TrialEndsAt = time.Now().UTC().Add(time.Duration(*input.TrialDays) * 24 * time.Hour).Format(time.RFC3339)
		}
		if input.DailyLimit != nil {
			credit.DailyLimit = *input.DailyLimit
		}
		if credit.DailyLimit < 1 || credit.DailyLimit > 1000000 {
			writeError(w, 400, errors.New("每日额度需在 1 到 1000000 之间"))
			return
		}
		if err := h.store.SetAccountCredit(id, credit.TrialEndsAt, true, credit.DailyLimit); err != nil {
			writeError(w, 500, err)
			return
		}
	}
	if input.Status != "" {
		if err := h.store.SetUserStatus(id, input.Status); err != nil {
			writeError(w, 400, err)
			return
		}
	}
	user, _ = h.store.GetUser(id)
	credit, _ := h.store.GetAccountCredit(id)
	writeJSON(w, 200, map[string]any{"user": user, "credit": credit})
}

func (h *Handler) adjustManagedCredits(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var input struct {
		Amount int64  `json:"amount"`
		Note   string `json:"note"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, err)
		return
	}
	credit, err := h.store.AdjustCredits(id, userIDFromRequest(r), input.Amount, input.Note)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	writeJSON(w, 200, credit)
}

func (h *Handler) resetManagedPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.store.GetUser(id)
	if err != nil || user.Role != "member" {
		writeError(w, 404, errors.New("普通账号不存在"))
		return
	}
	password, err := h.auth.ResetPassword(id)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]string{"temporary_password": password})
}

func (h *Handler) adminCreditLedger(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := h.store.GetUser(id); err != nil {
		writeError(w, 404, err)
		return
	}
	items, err := h.store.ListCreditLedger(id, 100)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, items)
}

func (h *Handler) updateCreditPrice(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Points int64 `json:"points"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, err)
		return
	}
	if err := h.store.SetCreditPrice(chi.URLParam(r, "capability"), input.Points, userIDFromRequest(r)); err != nil {
		writeError(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"prices": h.store.CreditPrices()})
}

func (h *Handler) myCredits(w http.ResponseWriter, r *http.Request) {
	credit, err := h.store.GetAccountCredit(userIDFromRequest(r))
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 200, map[string]any{"managed_enabled": false, "prices": h.store.CreditPrices()})
		return
	}
	if err != nil {
		writeError(w, 500, err)
		return
	}
	used, err := h.store.DailyCreditUsage(userIDFromRequest(r))
	if err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"account": credit, "daily_used": used, "prices": h.store.CreditPrices()})
}

func (h *Handler) myCreditLedger(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListCreditLedger(userIDFromRequest(r), 50)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, items)
}
