package web

import (
	"bytes"
	"encoding/json"
	"github.com/tian1363/scriptagent/internal/auth"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAnalyticsRoutesPrivacyIsolationAndServerEvents(t *testing.T) {
	s, err := jobs.OpenStore(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	u, err := s.CreateUser(jobs.CreateUserInput{Email: "analytics@example.com", PasswordHash: "hash", Role: "member", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateUser(jobs.CreateUserInput{Email: "other@example.com", PasswordHash: "hash", Role: "member", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.CreateSession(jobs.CreateSessionInput{UserID: u.ID, Token: "analytics-token", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(Config{}, s, storage.NewLocalStore(t.TempDir()), nil, nil, nil, nil)
	router := h.Routes()
	call := func(method, path, body string, cookie bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if cookie {
			r.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: session.Token})
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Creative-Session", "testsession")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/api/owner/analytics", "", false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := call("POST", "/api/analytics/events", `{"id":"e","name":"page_view","feature":"home","session_id":"s"}`, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, body := range []string{`{"name":"task_accepted","feature":"pose"}`, `{"name":"page_view","feature":"home","user_id":"` + other.ID + `"}`, `{"name":"page_view","feature":"home","prompt":"private"}`, `{"name":"page_view","feature":"home","effective":true}`} {
		if w := call("POST", "/api/analytics/events", body, true); w.Code != 400 {
			t.Fatal("client forged server data", w.Code, w.Body.String())
		}
	}
	if w := call("POST", "/api/analytics/events", `{"id":"e","name":"page_view","feature":"home","session_id":"s"}`, true); w.Code != 204 {
		t.Fatal(w.Code)
	}
	// A real application route is recorded after it succeeds, not from a fabricated client success.
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	_ = form.WriteField("title", "Analytics product")
	file, _ := form.CreateFormFile("product_md", "product.md")
	_, _ = file.Write([]byte("# private product facts"))
	_ = form.Close()
	req := httptest.NewRequest("POST", "/api/products", &buf)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("X-Creative-Session", "testsession")
	req.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: session.Token})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code < 200 || w.Code >= 300 {
		t.Fatal(w.Code, w.Body.String())
	}
	a, err := s.UserAnalytics(u.ID, 7, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if a.Visits != 1 || a.ActiveDays != 1 {
		t.Fatal(a)
	}
	if strings.Contains(string(encoded), "private product facts") || strings.Contains(string(encoded), other.Email) {
		t.Fatal("sensitive data leaked")
	}
	b, err := s.UserAnalytics(other.ID, 7, time.Now())
	if err != nil || b.Visits != 0 || b.ActiveDays != 0 {
		t.Fatal(b, err)
	}
	if w = call("GET", "/api/owner/analytics", "", true); w.Code != 403 {
		t.Fatal("ordinary user accessed global data", w.Code)
	}
	if w = call("GET", "/api/analytics", "", true); w.Code != 404 {
		t.Fatal(w.Code)
	}
}

func TestOperationsAnalyticsOnlyBoundAdminCanRead(t *testing.T) {
	s, err := jobs.OpenStore(filepath.Join(t.TempDir(), "analytics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	create := func(email, role string) *jobs.User {
		u, err := s.CreateUser(jobs.CreateUserInput{Email: email, PasswordHash: "hash", Role: role, Status: "active"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.CreateSession(jobs.CreateSessionInput{UserID: u.ID, Token: u.ID, ExpiresAt: time.Now().Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	admin := create("owner@example.com", "admin")
	unbound := create("unbound@example.com", "admin")
	member := create("member@example.com", "member")
	other := create("second@example.com", "member")
	for _, u := range []*jobs.User{admin, member, other} {
		if err = s.RecordAnalytics(u.ID, jobs.AnalyticsEvent{ID: "saved", Name: "material_saved", Feature: "products", Effective: true}); err != nil {
			t.Fatal(err)
		}
	}
	router := NewHandler(Config{AdminUserID: admin.ID}, s, nil, nil, nil, nil, nil).Routes()
	for _, tc := range []struct {
		token string
		want  int
	}{{"", 401}, {member.ID, 403}, {unbound.ID, 403}, {admin.ID, 200}} {
		r := httptest.NewRequest("GET", "/api/owner/analytics?days=7&user_id="+member.ID, nil)
		if tc.token != "" {
			r.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: tc.token})
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("access got %d want %d: %s", w.Code, tc.want, w.Body.String())
		}
		if tc.want == 200 {
			var a jobs.AnalyticsSummary
			if err = json.Unmarshal(w.Body.Bytes(), &a); err != nil {
				t.Fatal(err)
			}
			if a.Scope != "platform" || a.ActiveUsers != 2 || a.Activity["dau"] != 2 {
				t.Fatalf("not platform-scoped: %+v", a)
			}
			if w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("analytics must not be cached")
			}
		}
	}
	for _, days := range []string{"999999", "invalid"} {
		r := httptest.NewRequest("GET", "/api/owner/analytics?days="+days, nil)
		r.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: admin.ID})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal("invalid range accepted", w.Code)
		}
	}
}
