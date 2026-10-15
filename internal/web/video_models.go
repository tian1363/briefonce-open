package web

import (
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/videogen"
	"net/http"
	"strconv"
)

func (h *Handler) videoModels(w http.ResponseWriter, r *http.Request) {
	seconds, _ := strconv.Atoi(r.URL.Query().Get("seconds"))
	if seconds == 0 {
		seconds = 5
	}
	resolution := r.URL.Query().Get("resolution")
	if resolution == "" {
		resolution = "default"
	}
	videoRef := r.URL.Query().Get("video_reference") == "true"
	type entry struct {
		Model     videogen.Model `json:"model"`
		Points    int64          `json:"points"`
		Version   int64          `json:"version"`
		Available bool           `json:"available"`
		Message   string         `json:"message"`
	}
	runtime, runtimeErr := h.store.GetModelRuntimeConfig(r.Context(), "video_generation")
	byok := runtimeErr == nil && runtime.Source == "byok" && runtime.Provider == "zexapi" && runtime.APIKey != ""
	items := []entry{}
	for _, m := range videogen.Models {
		full, _ := videogen.FindModel(m.ID)
		price, points, err := h.store.VideoQuote(m.ID, resolution, seconds, videoRef, 0)
		e := entry{Model: full, Points: points, Version: price.Version, Available: err == nil}
		if err != nil {
			e.Message = err.Error()
		}
		if byok {
			e.Available = true
			e.Points = 0
			e.Message = "自有 API Key，由供应商直接结算"
		}
		items = append(items, e)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, items)
}
func (h *Handler) opsVideoPrice(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Price jobs.ImagePrice `json:"price"`
		Note  string          `json:"note"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	if err := h.store.SaveVideoPrice(in.Price, userIDFromRequest(r), in.Note); err != nil {
		writeError(w, 400, err)
		return
	}
	h.opsOverview(w, r)
}
