package web

import (
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/userctx"
	"net/http"
)

// The view has its own GET-only allowlist. It never changes the admin session.
func (h *Handler) readOnlyUserView(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, 403, errors.New("用户工作台为只读模式，不允许修改或发起任务"))
			return
		}
		actor := userctx.UserID(r.Context())
		target, err := h.store.GetUser(chi.URLParam(r, "viewUserID"))
		if err != nil {
			writeError(w, 404, errors.New("用户不存在"))
			return
		}
		if r.URL.Path == "/api/owner/view/"+target.ID+"/profile" {
			if err := h.store.RecordAdminUserView(actor, target.ID); err != nil {
				writeError(w, 500, errors.New("无法记录此次查看"))
				return
			}
		}
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-User-View", "read-only")
		ctx := userctx.WithUser(r.Context(), userctx.User{ID: target.ID, Email: target.Email, Name: target.Name, Role: "member"})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (h *Handler) adminUserViewProfile(w http.ResponseWriter, r *http.Request) {
	target, _ := userctx.FromContext(r.Context())
	user, err := h.store.GetUser(target.ID)
	if err != nil {
		writeError(w, 404, errors.New("用户不存在"))
		return
	}
	writeJSON(w, 200, map[string]any{"user": map[string]string{"id": target.ID, "email": target.Email, "name": target.Name, "status": user.Status}, "read_only": true})
}
func (h *Handler) mountAdminUserView(r chi.Router) {
	r.Route("/owner/view/{viewUserID}", func(v chi.Router) {
		v.Use(h.requireAdmin)
		v.Use(h.readOnlyUserView)
		v.Get("/profile", h.adminUserViewProfile)
		v.Get("/products", h.listProducts)
		v.Get("/products/{id}/markdown", h.getProductMarkdown)
		v.Get("/products/{id}/assets", h.listProductAssets)
		v.Get("/products/{id}/observations", h.listProductObservations)
		v.Get("/artworks", h.listArtworks)
		v.Get("/characters/{id}/image", h.getCharacterImage)
		v.Get("/assets/{id}/file", h.getProductAssetFile)
		v.Get("/image-provider-jobs/{id}/image", h.myProviderImage)
		v.Get("/spaces", h.listSpaces)
		v.Get("/spaces/{id}/videos", h.listSpaceVideos)
		v.Get("/chats", h.listChats)
		v.Get("/chats/{id}", h.getChat)
		v.Get("/chats/{id}/attachments/{attachmentID}/file", h.getChatAttachmentFile)
		v.Get("/chats/{id}/attachments/{attachmentID}/poster", h.getChatAttachmentPoster)
		v.Get("/videos", h.listVideos)
		v.Get("/videos/{id}/file", h.getVideoFile)
		v.Get("/render-batches/{id}/variants/{variantID}/file", h.getRenderVariantFile)
		v.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
			writeError(w, 404, errors.New("此内容不在只读查看范围内"))
		})
	})
}
