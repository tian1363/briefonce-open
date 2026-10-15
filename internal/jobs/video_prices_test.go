package jobs

import (
	"encoding/base64"
	"github.com/tian1363/scriptagent/internal/model"
	"strings"
	"testing"
)

func TestVideoPricingCombinationsAndReservation(t *testing.T) {
	s, u, ctx := opsStore(t)
	rows, e := s.VideoPrices()
	if e != nil || len(rows) != 24 {
		t.Fatal(len(rows), e)
	}
	if _, _, e = s.VideoQuote("seedance-2.0", "default", 5, false, 0); e == nil {
		t.Fatal("unpriced model allowed")
	}
	p := ImagePrice{Model: "seedance-2.0", Tier: "default:standard", Enabled: true, CostMicros: 123456, MultiplierBPS: 20000, PointMicros: 10000}
	if e = s.SaveVideoPrice(p, "owner", "supplier pricing"); e != nil {
		t.Fatal(e)
	}
	total, points, e := s.VideoQuote(p.Model, "default", 7, false, 0)
	if e != nil || points != 173 || total.CostMicros != 864192 {
		t.Fatal(total, points, e)
	}
	if _, _, e = s.VideoQuote(p.Model, "default", 7, true, 0); e == nil {
		t.Fatal("video reference borrowed standard price")
	}
	if _, _, e = s.VideoQuote(p.Model, "default", 7, false, 1); e == nil || !strings.Contains(e.Error(), "更新") {
		t.Fatal(e)
	}
	id, _, e := s.ReserveVideoUsage(ctx, p.Model, "default", 7, false, total.Version)
	if e != nil {
		t.Fatal(e)
	}
	credit, _ := s.GetAccountCredit(u.ID)
	if credit.Balance != 827 {
		t.Fatal(credit.Balance)
	}
	if e = s.FinishUsage(id, false); e != nil {
		t.Fatal(e)
	}
	credit, _ = s.GetAccountCredit(u.ID)
	if credit.Balance != 1000 {
		t.Fatal(credit.Balance)
	}
}

func TestVideoRuntimeEncryptedAndIsolated(t *testing.T) {
	s, u, _ := opsStore(t)
	if err := s.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))); err != nil {
		t.Fatal(err)
	}
	cfg := model.RuntimeConfig{Provider: "zexapi", Model: "minimax-h3", Endpoint: "https://zexapi.com/v1/videos", APIKey: "private-video-key"}
	if err := s.SaveVideoRuntime("video", u.ID, cfg); err != nil {
		t.Fatal(err)
	}
	var stored string
	s.db.QueryRow(`SELECT config FROM video_runtime_snapshots WHERE video_id='video'`).Scan(&stored)
	if strings.Contains(stored, cfg.APIKey) {
		t.Fatal("plaintext key")
	}
	got, err := s.VideoRuntime("video", u.ID)
	if err != nil || got.APIKey != cfg.APIKey {
		t.Fatal("snapshot not restored", err)
	}
	if _, err = s.VideoRuntime("video", "other"); err == nil {
		t.Fatal("cross user snapshot exposed")
	}
}
