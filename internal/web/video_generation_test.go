package web

import (
	"encoding/base64"
	"encoding/json"
	"github.com/tian1363/scriptagent/internal/auth"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVideoProductRouteCompletesAndSettlesQuotedPrice(t *testing.T) {
	s, err := jobs.OpenStore(filepath.Join(t.TempDir(), "video.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))); err != nil {
		t.Fatal(err)
	}
	u, err := s.CreateManagedUser(jobs.CreateUserInput{Email: "video@test.com", PasswordHash: "unused"}, time.Now().Add(time.Hour).Format(time.RFC3339), 1000, 1000, "owner")
	if err != nil {
		t.Fatal(err)
	}
	s.SetSoleAdministrator(u.ID)
	if err = s.SavePlatformModelProfile(jobs.PlatformModelInput{Capability: "video_generation", Provider: "zexapi", Model: "seedance-2.0", Endpoint: "https://zexapi.com/v1/videos", APIKey: "test-video-key"}); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveVideoPrice(jobs.ImagePrice{Model: "minimax-h3-max", Tier: "default:standard", Enabled: true, CostMicros: 100000, MultiplierBPS: 20000, PointMicros: 10000}, u.ID, "test supplier"); err != nil {
		t.Fatal(err)
	}
	session, err := s.CreateSession(jobs.CreateSessionInput{UserID: u.ID, Token: "video-token", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	http.DefaultTransport = poseRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "zexapi.com" && r.URL.Host != "media.example.com" {
			return original.RoundTrip(r)
		}
		response := `{"status":"completed","url":"https://media.example.com/result.mp4"}`
		if r.Method == "POST" {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
			}
			if r.FormValue("model") != "minimax-h3-max" || r.FormValue("seconds") != "5" || r.FormValue("aspect_ratio") != "21:9" {
				t.Error("wrong provider request")
			}
			response = `{"id":"video-upstream","status":"queued"}`
		}
		if r.URL.Host == "media.example.com" {
			response = "mock mp4 bytes"
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(response))}, nil
	})
	defer func() { http.DefaultTransport = original }()
	h := NewHandler(Config{AdminUserID: u.ID, AllowManagedMode: true}, s, storage.NewLocalStore(t.TempDir()), nil, nil, nil, nil)
	r := httptest.NewRequest("POST", "/api/videos", strings.NewReader(`{"prompt":"walk","mode":"text","model":"minimax-h3-max","resolution":"default","ratio":"21:9","duration":5,"price_version":2}`))
	r.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: session.Token})
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var item jobs.VideoGeneration
	if err = json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		current, e := s.GetUserVideoGeneration(u.ID, item.ID)
		if e != nil {
			t.Fatal(e)
		}
		if current.Status == "failed" {
			t.Fatal(current.ErrorMessage)
		}
		if current.Status == "completed" {
			metrics, err := s.UserAnalytics(u.ID, 7, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if metrics.Tasks != 1 || metrics.Completed != 1 || metrics.Requests != 1 || len(metrics.Models) != 1 {
				t.Fatalf("video flow analytics: %+v", metrics)
			}

			totals, e := s.UsageTotals(u.ID)
			if e != nil {
				t.Fatal(e)
			}
			if totals["consumed"] != 100 {
				time.Sleep(20 * time.Millisecond)
				continue
			}
			credit, _ := s.GetAccountCredit(u.ID)
			if credit.Balance != 900 {
				t.Fatal("wrong point charge", credit.Balance)
			}
			if current.LocalPath == "" || current.EstimatedCostCNY != 0.5 {
				t.Fatal("missing file or cost", current)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("video did not complete")
}
