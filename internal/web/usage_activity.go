package web

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

func (h *Handler) myUsageActivity(w http.ResponseWriter, r *http.Request) {
	h.usageActivity(w, r, userIDFromRequest(r))
}

func (h *Handler) adminUsageActivity(w http.ResponseWriter, r *http.Request) {
	h.usageActivity(w, r, chi.URLParam(r, "id"))
}

func (h *Handler) usageActivity(w http.ResponseWriter, r *http.Request, userID string) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	items, err := h.store.ListUsageActivity(userID, offset)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	more := len(items) > 50
	if more {
		items = items[:50]
	}
	totals, err := h.store.UsageTotals(userID)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "has_more": more, "totals": totals})
}
func (h *Handler) myImageUsage(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromRequest(r)
	assetID := r.URL.Query().Get("asset_id")
	characterID := r.URL.Query().Get("character_id")
	if assetID != "" {
		asset, err := h.store.GetProductAsset(assetID)
		if err != nil {
			writeError(w, 404, err)
			return
		}
		if _, err = h.store.GetUserProduct(userID, asset.ProductID); err != nil {
			writeError(w, 404, err)
			return
		}
	}
	if characterID != "" {
		if _, err := h.store.GetCharacter(userID, characterID); err != nil {
			writeError(w, 404, err)
			return
		}
	}
	item, err := h.store.ImageUsage(userID, assetID, characterID)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, item)
}
