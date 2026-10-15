package web

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/storage"
	"net/http"
)

func (h *Handler) appendProductReference(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := h.store.GetUserProduct(userIDFromRequest(r), id); err != nil {
		writeError(w, 404, err)
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, int64(storage.MaxMarkdownBytes)+4096)).Decode(&input); err != nil {
		writeError(w, 400, err)
		return
	}
	if err := h.store.AppendProductReference(userIDFromRequest(r), id, input.Content); err != nil {
		writeError(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"saved": true})
}
