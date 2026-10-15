package web

import (
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// Keep previews inline; explicit download links return a real attachment.
func setImageDownloadHeaders(w http.ResponseWriter, r *http.Request, name, path string) {
	if r.URL.Query().Get("download") != "1" {
		return
	}
	if requested := r.URL.Query().Get("filename"); requested != "" {
		name = requested
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(c rune) rune {
		if c < 32 || c == 127 {
			return -1
		}
		return c
	}, name)
	if name == "" || name == "." {
		name = "image"
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" && !strings.HasSuffix(strings.ToLower(name), ext) {
		name += ext
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Cache-Control", "private, no-store")
}
