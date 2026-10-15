package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tian1363/scriptagent/internal/jobs"
)

type analyticsRequestKey struct{}
type analyticsRequestInfo struct{ TaskIDs []string }

func linkAnalyticsTasks(r *http.Request, ids ...string) {
	if info, ok := r.Context().Value(analyticsRequestKey{}).(*analyticsRequestInfo); ok {
		info.TaskIDs = append(info.TaskIDs, ids...)
	}
}
func linkAnalyticsUsage(r *http.Request, tasks []jobs.UsageTask) {
	for _, task := range tasks {
		linkAnalyticsTasks(r, task.TaskID)
	}
}

func (h *Handler) analyticsEnvironment() string {
	if h.cfg.ModelMode == "mock" {
		return "development"
	}
	return "production"
}
func (h *Handler) analyticsSummary(w http.ResponseWriter, r *http.Request) {
	days := 30
	if r.URL.Query().Has("days") {
		var err error
		days, err = strconv.Atoi(r.URL.Query().Get("days"))
		if err != nil {
			writeError(w, 400, errors.New("时间范围无效"))
			return
		}
	}
	data, err := h.store.OperationsAnalytics(days, time.Now())
	if err != nil {
		writeError(w, 400, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, data)
}
func (h *Handler) analyticsCollect(w http.ResponseWriter, r *http.Request) {
	var e jobs.AnalyticsEvent
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		writeError(w, 400, errors.New("事件格式无效"))
		return
	}
	if dec.Decode(new(any)) != io.EOF {
		writeError(w, 400, errors.New("事件格式无效"))
		return
	}
	// Clients can report intent and navigation, never task success, charges or downloads.
	if e.Name != "page_view" && e.Name != "feature_open" && e.Name != "result_preview" {
		writeError(w, 400, errors.New("不支持的客户端事件"))
		return
	}
	if e.TaskID != "" || e.RefID != "" || e.RequestID != "" {
		writeError(w, 400, errors.New("客户端不能写入业务关联标识"))
		return
	}
	e.Environment = h.analyticsEnvironment()
	e.Effective = false
	if !h.authLimit.allow("analytics:"+userIDFromRequest(r), 120, time.Minute) {
		writeError(w, 429, errors.New("事件请求过于频繁"))
		return
	}
	if err := h.store.RecordAnalytics(userIDFromRequest(r), e); err != nil {
		writeError(w, 400, err)
		return
	}
	if e.SessionID != "" && len(e.SessionID) <= 100 {
		http.SetCookie(w, &http.Cookie{Name: "briefonce_analytics_session", Value: e.SessionID, Path: "/api", HttpOnly: true, Secure: h.cfg.SecureCookies || r.TLS != nil, SameSite: http.SameSiteLaxMode, MaxAge: 1800})
	}
	w.WriteHeader(204)
}

// Runs after authentication. No body, query, URL, prompt or supplied user ID is stored.
// The original response and streaming interfaces are preserved by chi's wrapper.
func (h *Handler) analyticsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/owner/") || strings.HasPrefix(r.URL.Path, "/api/analytics") {
			next.ServeHTTP(w, r)
			return
		}
		info := &analyticsRequestInfo{}
		r = r.WithContext(context.WithValue(r.Context(), analyticsRequestKey{}, info))
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		pattern := chi.RouteContext(r.Context()).RoutePattern()
		name, feature := analyticsRoute(r.Method, pattern)
		if name == "" {
			if r.Method == "GET" && r.URL.Query().Get("download") == "1" && strings.HasSuffix(pattern, "/image") {
				name, feature = "download_served", "other"
			} else if r.Method == "GET" && r.URL.Query().Get("download") == "1" && (strings.HasSuffix(pattern, "/file") || strings.HasSuffix(pattern, "/clip") || strings.HasSuffix(pattern, "/frame")) {
				name, feature = "download_served", "other"
			} else {
				return
			}
		}
		code := ww.Status()
		if code == 0 {
			code = 200
		}
		ok := code >= 200 && code < 300
		if !ok {
			if name != "task_accepted" {
				return
			}
			name = "request_failed"
		}
		e := jobs.AnalyticsEvent{Name: name, Feature: feature, Entry: r.Header.Get("X-Creative-Entry"), SessionID: r.Header.Get("X-Creative-Session"), RequestID: middleware.GetReqID(r.Context()), Environment: h.analyticsEnvironment(), Effective: ok && (name == "material_saved" || name == "task_accepted" || name == "asset_imported" || name == "download_served")}
		if e.SessionID == "" {
			if cookie, err := r.Cookie("briefonce_analytics_session"); err == nil {
				e.SessionID = cookie.Value
			}
		}
		if e.Entry == "" || e.Entry == "navigation" {
			switch feature {
			case "pose", "refine", "detail":
				e.Entry = "canvas"
			case "extract", "interpret":
				e.Entry = "toolbox"
			case "chat", "video":
				e.Entry = "composer"
			}
		}
		if ok && name == "task_accepted" && len(info.TaskIDs) > 0 {
			sort.Strings(info.TaskIDs)
			e.ID = fmt.Sprintf("accepted_%x", sha256.Sum256([]byte(strings.Join(info.TaskIDs, ","))))
			for _, id := range info.TaskIDs {
				linked := e
				linked.ID = "link_" + id
				linked.Name = "task_linked"
				linked.TaskID = id
				linked.Effective = false
				_ = h.store.RecordAnalytics(userIDFromRequest(r), linked)
			}
		}
		_ = h.store.RecordAnalytics(userIDFromRequest(r), e)
	})
}
func analyticsRoute(method, route string) (string, string) {
	if method == "POST" {
		switch route {
		case "/api/pose-generations":
			return "task_accepted", "pose"
		case "/api/products/{id}/image-transforms":
			return "task_accepted", "refine"
		case "/api/products/{id}/detail-images":
			return "task_accepted", "detail"
		case "/api/videos", "/api/videos/{id}/retry":
			return "task_accepted", "video"
		case "/api/asset-extractions", "/api/asset-extractions/batch", "/api/asset-extractions/upload", "/api/asset-extractions/batch/upload":
			return "task_accepted", "extract"
		case "/api/creative/interpret":
			return "task_accepted", "interpret"
		case "/api/products", "/api/products/{id}/assets", "/api/assets/{id}/replace":
			return "material_saved", "products"
		case "/api/asset-extractions/{id}/import":
			return "asset_imported", "extract"
		case "/api/chats/messages", "/api/chats/{id}/messages":
			return "task_accepted", "chat"
		}
	}
	if method == "PUT" && route == "/api/products/{id}" {
		return "material_saved", "products"
	}
	return "", ""
}
