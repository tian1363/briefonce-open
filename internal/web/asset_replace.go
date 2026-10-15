package web

import (
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/jobs"
	"net/http"
	"os"
)

func (h *Handler) replaceProductAsset(w http.ResponseWriter, r *http.Request) {
	old, err := h.store.GetProductAsset(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 404, errors.New("图片资产不存在"))
		return
	}
	if _, err = h.store.GetUserProduct(userIDFromRequest(r), old.ProductID); err != nil {
		writeError(w, 404, errors.New("图片资产不存在"))
		return
	}
	if old.Kind != "image" && old.Kind != "reference" && old.Kind != "artwork" {
		writeError(w, 400, errors.New("仅支持替换图片资产"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 21<<20)
	if err = r.ParseMultipartForm(20 << 20); err != nil {
		writeError(w, 400, errors.New("请选择 20MB 以内的图片"))
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("asset")
	if err != nil {
		writeError(w, 400, errors.New("请选择替换图片"))
		return
	}
	defer file.Close()
	if header.Size > 20<<20 {
		writeError(w, 400, errors.New("图片最大 20MB"))
		return
	}
	path, err := h.files.SaveUpload(file, header, "asset")
	if err != nil {
		writeError(w, 400, err)
		return
	}
	data, err := os.ReadFile(path)
	mime := http.DetectContentType(data)
	if err != nil || (mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" && mime != "image/gif") {
		os.Remove(path)
		writeError(w, 400, errors.New("请上传 JPG、PNG、WEBP 或 GIF 图片"))
		return
	}
	kind := old.Kind
	if kind == "artwork" {
		kind = "image"
	}
	asset, err := h.store.ReplaceProductAsset(old.ID, jobs.ProductAsset{ProductID: old.ProductID, Kind: kind, Path: path, OriginalName: header.Filename, MimeType: mime, SizeBytes: int64(len(data))})
	if err != nil {
		os.Remove(path)
		writeError(w, 400, errors.New("图片已移除或替换，请刷新后重试"))
		return
	}
	writeJSON(w, 200, asset)
}
