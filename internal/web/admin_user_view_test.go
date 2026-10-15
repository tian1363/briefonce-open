package web

import (
	"bytes"
	"github.com/tian1363/scriptagent/internal/auth"
	"github.com/tian1363/scriptagent/internal/jobs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAdministratorUserViewIsIsolatedAndReadOnly(t *testing.T) {
	dir := t.TempDir()
	s, err := jobs.OpenStore(filepath.Join(dir, "view.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	admin, _ := s.CreateUser(jobs.CreateUserInput{Email: "admin@test.com", Status: "active", PasswordHash: "admin-secret"})
	a, _ := s.CreateUser(jobs.CreateUserInput{Email: "one@test.com", Status: "active", PasswordHash: "one-secret"})
	b, _ := s.CreateUser(jobs.CreateUserInput{Email: "two@test.com", Status: "active"})
	s.SetSoleAdministrator(admin.ID)
	for _, u := range []*jobs.User{admin, a, b} {
		_, err = s.CreateSession(jobs.CreateSessionInput{UserID: u.ID, Token: u.ID, ExpiresAt: time.Now().Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
	}
	products := map[string]*jobs.Product{}
	assets := map[string]*jobs.ProductAsset{}
	for _, u := range []*jobs.User{admin, a, b} {
		path := filepath.Join(dir, u.ID+".md")
		os.WriteFile(path, []byte("private "+u.Email), 0600)
		p, e := s.CreateProduct(jobs.CreateProductInput{Title: "product-" + u.Email, MDPath: path, MDName: "product.md"})
		if e != nil {
			t.Fatal(e)
		}
		if e = s.ClaimResource(u.ID, "product", p.ID); e != nil {
			t.Fatal(e)
		}
		products[u.ID] = p
		asset, e := s.CreateProductAsset(jobs.ProductAsset{ProductID: p.ID, Kind: "image", Path: path, OriginalName: "image.png", MimeType: "image/png"})
		if e != nil {
			t.Fatal(e)
		}
		assets[u.ID] = asset
	}
	h := NewHandler(Config{AdminUserID: admin.ID}, s, nil, nil, nil, nil, nil)
	routes := h.Routes()
	request := func(method, path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(`{"title":"changed","prompt":"bill me"}`))
		if token != "" {
			r.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: token})
		}
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, r)
		return w
	}
	base := "/api/owner/view/" + a.ID
	for _, tc := range []struct {
		token  string
		status int
	}{{"", 401}, {a.ID, 403}, {b.ID, 403}, {admin.ID, 200}} {
		w := request("GET", base+"/products", tc.token)
		if w.Code != tc.status {
			t.Fatalf("access %d %s", w.Code, w.Body.String())
		}
		if tc.token == admin.ID && (!strings.Contains(w.Body.String(), a.Email) || strings.Contains(w.Body.String(), b.Email) || strings.Contains(w.Body.String(), admin.Email)) {
			t.Fatal("wrong user data", w.Body.String())
		}
	}
	w := request("GET", base+"/profile", admin.ID)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"read_only":true`) || strings.Contains(w.Body.String(), "one-secret") || len(w.Result().Cookies()) > 0 {
		t.Fatal("profile or session leak", w.Body.String())
	}
	w = request("GET", base+"/products/"+products[a.ID].ID+"/markdown", admin.ID)
	if w.Code != 200 || !strings.Contains(w.Body.String(), a.Email) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("GET", base+"/assets/"+assets[a.ID].ID+"/file", admin.ID)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(a.Email)) {
		t.Fatal("file not readable", w.Code)
	}
	for _, path := range []string{"/products/" + products[b.ID].ID + "/markdown", "/products/" + products[b.ID].ID + "/assets", "/assets/" + assets[b.ID].ID + "/file", "/settings/model", "/owner/provider", "/auth/me"} {
		if w = request("GET", base+path, admin.ID); w.Code != 404 {
			t.Fatal("read scope bypass", path, w.Code)
		}
	}
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		for _, path := range []string{"/products", "/products/" + products[a.ID].ID, "/assets/" + assets[a.ID].ID, "/assets/" + assets[a.ID].ID + "/replace", "/videos", "/pose-generations", "/asset-extractions", "/auth/logout"} {
			if w = request(method, base+path, admin.ID); w.Code != 403 {
				t.Fatal("write allowed", method, path, w.Code)
			}
		}
	}
	if w = request("GET", "/api/owner/view/missing/profile", admin.ID); w.Code != 404 {
		t.Fatal("missing user accepted")
	}
	w = request("GET", "/api/auth/me", admin.ID)
	if w.Code != 200 || !strings.Contains(w.Body.String(), admin.ID) {
		t.Fatal("admin session changed")
	}
	w = request("GET", "/api/products", admin.ID)
	if w.Code != 200 || !strings.Contains(w.Body.String(), admin.Email) || strings.Contains(w.Body.String(), a.Email) {
		t.Fatal("global user context changed")
	}
	doc, e := os.ReadFile(products[a.ID].MDPath)
	if e != nil || string(doc) != "private "+a.Email {
		t.Fatal("user document modified")
	}
	audits, e := s.OperationsAudit()
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, entry := range audits {
		if entry["kind"] == "user_read_only_view" && entry["actor"] == admin.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("view missing from administrator audit")
	}
}
