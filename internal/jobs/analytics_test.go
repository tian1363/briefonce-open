package jobs

import (
	"context"
	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/userctx"
	"path/filepath"
	"testing"
	"time"
)

func TestAnalyticsIsolationDedupActivityAndBusinessTruth(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	makeUser := func(email string) *User {
		u, e := s.CreateUser(CreateUserInput{Email: email, PasswordHash: "hash", Role: "member", Status: "active"})
		if e != nil {
			t.Fatal(e)
		}
		return u
	}
	u, other := makeUser("a@example.com"), makeUser("b@example.com")
	e := AnalyticsEvent{ID: "event-1", Name: "page_view", Feature: "home", SessionID: "session", Entry: "navigation"}
	if err = s.RecordAnalytics(u.ID, e); err != nil {
		t.Fatal(err)
	}
	_ = s.RecordAnalytics(u.ID, e)
	e.ID = "effective"
	e.Name = "material_saved"
	e.Effective = true
	_ = s.RecordAnalytics(u.ID, e)
	_ = s.RecordAnalytics(other.ID, e)
	for _, status := range []string{"ready", "failed", "generating"} {
		if _, err = s.CreateCharacter(u.ID, "private title", "pose", "", status, nil); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = s.CreateCharacter(other.ID, "other", "pose", "", "ready", nil)
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: u.ID})
	if err = s.RecordModelCall(ctx, model.CallRecord{Model: "test-model", LatencyMS: 20, InputJSON: `{"secret":"private"}`}); err != nil {
		t.Fatal(err)
	}
	a, err := s.UserAnalytics(u.ID, 7, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if a.Visits != 1 || a.ActiveDays != 1 || a.Tasks != 3 || a.Completed != 1 || a.Failed != 1 || a.Pending != 1 || len(a.Models) != 1 || a.Models[0].Count != 1 || len(a.Timeline) != 7 {
		t.Fatalf("wrong own statistics: %+v", a)
	}
	if len(a.Features) != 1 || a.Features[0].Key != "pose" || a.Features[0].Count != 3 {
		t.Fatal(a.Features)
	}
	global, err := s.AnalyticsActivity(time.Now())
	if err != nil || global["dau"] != 2 {
		t.Fatal(global, err)
	}
	if _, err = s.UserAnalytics(u.ID, 100000, time.Now()); err == nil {
		t.Fatal("unbounded period accepted")
	}
}
func TestAnalyticsTimezoneAndExclusions(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	u, err := s.CreateUser(CreateUserInput{Email: "timezone@example.com", PasswordHash: "hash", Role: "member", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	e := AnalyticsEvent{ID: "boundary", Name: "material_saved", Feature: "products", Effective: true, Environment: "production"}
	_ = s.RecordAnalytics(u.ID, e)
	// 16:00 UTC belongs to the following Beijing day, including fractional seconds.
	_, err = s.db.Exec(`UPDATE analytics_events SET created_at='2026-10-14T16:00:00.123Z'`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 15, 4, 0, 0, 0, time.UTC)
	a, err := s.UserAnalytics(u.ID, 7, now)
	if err != nil {
		t.Fatal(err)
	}
	if a.Timeline[6].Date != "2026-10-15" || a.Timeline[6].Actions != 1 {
		t.Fatal(a.Timeline)
	}
	_, _ = s.db.Exec(`UPDATE analytics_events SET environment='development'`)
	activity, err := s.AnalyticsActivity(now)
	if err != nil || activity["dau"] != 0 {
		t.Fatal(activity, err)
	}
	_, _ = s.db.Exec(`UPDATE analytics_events SET environment='production'; UPDATE users SET role='admin'`)
	activity, err = s.AnalyticsActivity(now)
	if err != nil || activity["dau"] != 0 {
		t.Fatal(activity, err)
	}
}

func TestAnalyticsModelRetryDedupAndMatureRetention(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	u, err := s.CreateUser(CreateUserInput{Email: "retained@example.com", PasswordHash: "hash", Role: "member", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -7).Format(time.RFC3339)
	for _, id := range []string{"first", "returned"} {
		if err = s.RecordAnalytics(u.ID, AnalyticsEvent{ID: id, Name: "material_saved", Feature: "products", Effective: true}); err != nil {
			t.Fatal(err)
		}
	}
	_, err = s.db.Exec(`UPDATE analytics_events SET created_at=? WHERE id='first'`, old)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.AnalyticsActivity(now)
	if err != nil || a["retention_d7_eligible"] != 1 || a["retention_d7_returned"] != 1 || a["retention_d30_eligible"] != 0 {
		t.Fatal(a, err)
	}
	v, err := s.CreateVideoGeneration(CreateVideoGenerationInput{UserID: u.ID, Mode: "text", Prompt: "private", Model: "video-model", Resolution: "720p", Ratio: "9:16", Duration: 5})
	if err != nil {
		t.Fatal(err)
	}
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: u.ID})
	if err = s.RecordModelCall(ctx, model.CallRecord{Model: "video-model", RefID: v.ID, LatencyMS: 100}); err != nil {
		t.Fatal(err)
	}
	summary, err := s.UserAnalytics(u.ID, 30, now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Tasks != 1 || len(summary.Models) != 1 || summary.Models[0].Count != 1 {
		t.Fatalf("counted provider task twice: %+v", summary)
	}
}

func TestOperationsAnalyticsAggregatesCustomersAndExcludesInternalActivity(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "analytics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, tc := range []struct{ email, role, environment string }{
		{"first@example.com", "member", "production"},
		{"second@example.com", "member", "production"},
		{"owner@example.com", "admin", "production"},
		{"fixture@example.test", "member", "production"},
		{"development@example.com", "member", "development"},
	} {
		u, err := s.CreateUser(CreateUserInput{Email: tc.email, Role: tc.role, PasswordHash: "hash", Status: "active"})
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"page_view", "task_accepted"} {
			err = s.RecordAnalytics(u.ID, AnalyticsEvent{ID: name, Name: name, Feature: "pose", Entry: "canvas", SessionID: "same-session", Environment: tc.environment, Effective: name == "task_accepted"})
			if err != nil {
				t.Fatal(err)
			}
		}
		if tc.environment == "production" {
			if _, err = s.CreateCharacter(u.ID, "private product", "pose", "", "ready", nil); err != nil {
				t.Fatal(err)
			}
			ctx := userctx.WithUser(context.Background(), userctx.User{ID: u.ID})
			if err = s.RecordModelCall(ctx, model.CallRecord{Model: "shared-model", LatencyMS: 100, InputJSON: `{"private":"content"}`}); err != nil {
				t.Fatal(err)
			}
		}
	}
	a, err := s.OperationsAnalytics(7, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if a.Scope != "platform" || a.Visits != 2 || a.ActiveUsers != 2 || a.Tasks != 2 || a.Completed != 2 || a.Requests != 2 || a.Activity["dau"] != 2 {
		t.Fatalf("wrong platform summary: %+v", a)
	}
	if len(a.Models) != 1 || a.Models[0].Count != 2 || a.LatencySamples != 2 {
		t.Fatal("internal model usage leaked", a.Models)
	}
	if a.Timeline[6].ActiveUsers != 2 {
		t.Fatal(a.Timeline)
	}
	for _, r := range a.Journey {
		if r.Count != 2 {
			t.Fatal("same session IDs from different users collapsed", a.Journey)
		}
	}
	if _, err = s.UserAnalytics("", 7, time.Now()); err == nil {
		t.Fatal("empty user scope must not grant platform access")
	}
}
