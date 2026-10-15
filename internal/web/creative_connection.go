package web

import (
	"encoding/json"
	"github.com/tian1363/scriptagent/internal/creative"
	"net/http"
)

func (h *Handler) xhsConnection(w http.ResponseWriter, r *http.Request) {
	ctx := creative.WithCollectorOwner(r.Context(), userIDFromRequest(r))
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodPost:
		writeJSON(w, 200, creative.Connect(ctx))
	case http.MethodDelete:
		if err := creative.Disconnect(ctx); err != nil {
			writeError(w, 400, err)
			return
		}
		writeJSON(w, 200, creative.Connection{Status: "login_required", Message: "已断开小红书连接"})
	default:
		writeJSON(w, 200, creative.ConnectionStatus(ctx))
	}
}
func (h *Handler) readCreativeNote(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SourceURL string `json:"source_url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input); err != nil {
		writeError(w, 400, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, creative.ReadNote(creative.WithCollectorOwner(r.Context(), userIDFromRequest(r)), input.SourceURL))
}

func (h *Handler) creativeNoteImage(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("url")
	if len(raw) > 8192 {
		writeError(w, 400, &publicError{"图片地址过长"})
		return
	}
	data, kind, err := creative.FetchNoteImage(r.Context(), raw)
	if err != nil {
		writeError(w, 502, err)
		return
	}
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
