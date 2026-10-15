package jobs

import (
	"context"
	"github.com/tian1363/scriptagent/internal/detailpage"
	"github.com/tian1363/scriptagent/internal/userctx"
	"path/filepath"
	"testing"
)

func TestDetailRecoveryRefundsOnlyUndispatchedJobs(t *testing.T) {
	s, e := OpenStore(filepath.Join(t.TempDir(), "detail.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	user, e := s.CreateManagedUser(CreateUserInput{Email: "recovery@example.com", PasswordHash: "hash"}, "", 100, 100, "owner")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.SetCreditPrice("image_edit", 5, "owner"); e != nil {
		t.Fatal(e)
	}
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: user.ID})
	items, _, e := s.CreateDetailImageBatch(user.ID, "product", "request", "{}", []detailpage.Task{{PageID: "hero"}, {PageID: "specs"}})
	if e != nil {
		t.Fatal(e)
	}
	for _, j := range items {
		charge, e := s.ReserveUsage(ctx, "image_edit", 1)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.RecordUsageTasks([]UsageTask{{UserID: user.ID, TaskID: j.ID, ChargeID: charge, Capability: "image_edit", Source: "managed"}}); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.UpdateDetailImageJob(user.ID, items[0].ID, "generating", "", ""); e != nil {
		t.Fatal(e)
	}
	if e = s.InterruptDetailImageJobs(); e != nil {
		t.Fatal(e)
	}
	credit, e := s.GetAccountCredit(user.ID)
	if e != nil || credit.Balance != 95 {
		t.Fatalf("refund wrongly settled provider call: %+v %v", credit, e)
	}
	for _, j := range items {
		got, e := s.GetDetailImageJob(user.ID, "product", j.ID)
		if e != nil || got.Status != "failed" {
			t.Fatal("not interrupted", e)
		}
	}
	if e = s.InterruptDetailImageJobs(); e != nil {
		t.Fatal(e)
	}
	credit, _ = s.GetAccountCredit(user.ID)
	if credit.Balance != 95 {
		t.Fatal("repeated recovery refunded twice")
	}
}
