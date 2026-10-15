package jobs

import (
	"context"
	"github.com/tian1363/scriptagent/internal/userctx"
	"path/filepath"
	"testing"
	"time"
)

func TestUsageActivityLinksChargesAndIsolatesAccounts(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "usage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	user, err := s.CreateManagedUser(CreateUserInput{Email: "usage@example.com", PasswordHash: "hash"}, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), 1000, 1000, "owner")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateManagedUser(CreateUserInput{Email: "other@example.com", PasswordHash: "hash"}, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), 1000, 1000, "owner")
	if err != nil {
		t.Fatal(err)
	}
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: user.ID})
	charge, err := s.ReserveUsage(ctx, "image_edit", 1)
	if err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(user.ID, "外套姿势", "pose", "", "generating", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = s.RecordUsageTasks([]UsageTask{{UserID: user.ID, TaskID: role.ID, ChargeID: charge, Title: role.Name, Model: "test-image", Source: "managed", Capability: "image_edit"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordUsageTasks([]UsageTask{{UserID: other.ID, TaskID: "foreign", ChargeID: charge, Title: "foreign", Source: "managed"}}); err == nil {
		t.Fatal("foreign charge accepted")
	}
	items, err := s.ListUsageActivity(user.ID, 0)
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %+v %v", items, err)
	}
	if items[0].BillingStatus != "reserved" || items[0].TaskID != role.ID || items[0].Model != "test-image" {
		t.Fatalf("missing task details: %+v", items[0])
	}
	if err := s.FinishUsage(charge, false); err != nil {
		t.Fatal(err)
	}
	item, err := s.ImageUsage(user.ID, "", role.ID)
	if err != nil || item == nil || item.BillingStatus != "refunded" {
		t.Fatalf("refund: %+v %v", item, err)
	}
	hidden, err := s.ImageUsage(other.ID, "", role.ID)
	if err != nil || hidden != nil {
		t.Fatalf("foreign image usage exposed: %+v %v", hidden, err)
	}
	totals, err := s.UsageTotals(user.ID)
	if err != nil || totals["consumed"] != 0 || totals["refunded"] != item.Points {
		t.Fatalf("totals: %+v %v", totals, err)
	}
	err = s.RecordUsageTasks([]UsageTask{{UserID: user.ID, TaskID: "byok-task", Title: "自带 API", Model: "own-model", Source: "byok", Capability: "image_edit"}})
	if err != nil {
		t.Fatal(err)
	}
	items, err = s.ListUsageActivity(user.ID, 0)
	if err != nil || len(items) != 2 {
		t.Fatalf("byok missing: %+v %v", items, err)
	}
	if items[0].Points != 0 || items[0].Source != "byok" || items[0].BillingStatus != "unbilled" {
		t.Fatalf("wrong byok cost: %+v", items[0])
	}
}
