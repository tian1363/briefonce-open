package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/tian1363/scriptagent/internal/charactergen"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/storage"
	"github.com/tian1363/scriptagent/internal/userctx"
	"image"
	"image/png"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type opsTransport func(*http.Request) (*http.Response, error)

func (f opsTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestDurableImagePollingBillingAndDownload(t *testing.T) {
	dir := t.TempDir()
	s, e := jobs.OpenStore(filepath.Join(dir, "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	_ = s.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012")))
	u, e := s.CreateManagedUser(jobs.CreateUserInput{Email: "image@example.com", PasswordHash: "hash"}, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), 1000, 1000, "a")
	if e != nil {
		t.Fatal(e)
	}
	price := jobs.ImagePrice{Model: "gpt-image-2", Tier: "1K", Enabled: true, CostMicros: 300000, MultiplierBPS: 20000, PointMicros: 10000}
	if e = s.SaveImagePrice(price, "a", "test"); e != nil {
		t.Fatal(e)
	}
	o := charactergen.Options{Model: price.Model, AspectRatio: "1:1"}
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: u.ID})
	charge, e := s.ReserveImageUsage(ctx, "image_generation", o)
	if e != nil {
		t.Fatal(e)
	}
	role, e := s.CreateCharacter(u.ID, "test", "ai", "", "generating", nil)
	if e != nil {
		t.Fatal(e)
	}
	h := NewHandler(Config{}, s, storage.NewLocalStore(filepath.Join(dir, "uploads")), nil, nil, nil, nil)
	if e = h.queueImage(jobs.ProviderImageJob{ID: role.ID, UserID: u.ID, Kind: "character", ChargeID: charge, Capability: "image_generation"}, model.RuntimeConfig{Provider: "zexapi", APIKey: "test-key", Source: "managed", Model: o.Model, Endpoint: "https://zexapi.com/v1/videos"}, o, "create", nil); e != nil {
		t.Fatal(e)
	}
	submits, queries := 0, 0
	h.characterImages.HTTP = &http.Client{Transport: opsTransport(func(r *http.Request) (*http.Response, error) {
		raw := `{"id":"task-1","status":"queued"}`
		if r.Method == "POST" {
			submits++
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Fatal("wrong credential")
			}
		} else {
			queries++
			raw = `{"id":"task-1","status":"completed","image_url":"https://cdn.example.com/a.png"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(raw)), Header: make(http.Header)}, nil
	})}
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 384, 384)))
	h.imageDownload = func(ctx context.Context, url string) ([]byte, error) {
		if url != "https://cdn.example.com/a.png" {
			t.Fatal("nano URL ignored")
		}
		return b.Bytes(), nil
	}
	h.processProviderImages()
	if submits != 1 || queries != 0 {
		t.Fatal("unexpected polling")
	}
	h.processProviderImages()
	h.processProviderImages()
	if submits != 1 || queries != 1 {
		t.Fatal("duplicate provider request")
	}
	got, _ := s.GetCharacter(u.ID, role.ID)
	if got.Status != "ready" {
		t.Fatalf("result not saved: %+v", got)
	}
	credit, _ := s.GetAccountCredit(u.ID)
	if credit.Balance != 940 {
		t.Fatal("billing changed")
	}
	records, e := s.ProviderImageRecords()
	if e != nil || len(records) != 1 || records[0]["status"] != "completed" || records[0]["requests"].(int64) != 3 {
		t.Fatalf("missing audit %v %v", records, e)
	}
	p := records[0]["parameters"].(map[string]any)
	if p["aspect_ratio"] != "1:1" || p["prompt"] != "create" {
		t.Fatalf("admin controls missing: %+v", p)
	}
	if _, ok := records[0]["request_snapshot"]; ok {
		t.Fatal("encrypted payload exposed")
	}
	activity, e := s.ListUsageActivity(u.ID, 0)
	if e != nil || len(activity) != 1 || activity[0].Model != o.Model || activity[0].Parameters["aspect_ratio"] != "1:1" {
		t.Fatalf("user controls missing: %+v %v", activity, e)
	}
}
func TestOperationsCSVFormulaNeutralized(t *testing.T) {
	for _, v := range []string{"=HYPERLINK(\"x\")", " +SUM(1)", "@abc", "-cmd"} {
		if !strings.HasPrefix(csvSafe(v), "'") {
			t.Fatal("CSV formula not neutralized")
		}
	}
	if csvSafe(nil) != "" || csvSafe("regular") != "regular" {
		t.Fatal("CSV values corrupted")
	}
}
