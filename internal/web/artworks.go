package web

import (
	"net/http"
	"strconv"
)

func (h *Handler) listArtworks(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	items, err := h.store.ListArtworks(userIDFromRequest(r), r.URL.Query().Get("product_id"), offset)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	more := len(items) > 48
	if more {
		items = items[:48]
	}
	writeJSON(w, 200, map[string]any{"items": items, "has_more": more})
}
