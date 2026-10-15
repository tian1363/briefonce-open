package jobs

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tian1363/scriptagent/internal/userctx"
)

func TestManagedAPIKeyIsEncryptedAndOverridesEnvironment(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "environment-secret")
	store, err := OpenStore(filepath.Join(t.TempDir(), "provider.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SetManagedAPIKey("database-secret-key"); err == nil {
		t.Fatal("saved secret without encryption key")
	}
	if err := store.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))); err != nil {
		t.Fatal(err)
	}
	if err := store.SetManagedAPIKey("database-secret-key"); err != nil {
		t.Fatal(err)
	}
	key, err := store.ManagedAPIKey()
	if err != nil || key != "database-secret-key" {
		t.Fatalf("key lookup failed: %v", err)
	}
	var raw string
	if err := store.db.QueryRow(`SELECT encrypted_key FROM platform_credentials WHERE provider='dashscope'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, encryptedSecretPrefix) || strings.Contains(raw, key) {
		t.Fatal("platform key stored as plaintext")
	}
}

func TestManagedCreditsReserveRefundAndLimits(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "credits.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.CreateManagedUser(CreateUserInput{Email: "trial@example.com", PasswordHash: "hash"}, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), 10, 5, "owner")
	if err != nil {
		t.Fatal(err)
	}
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: user.ID})
	charge, err := store.ReserveUsage(ctx, "multimodal", 1)
	if err != nil {
		t.Fatal(err)
	}
	credit, _ := store.GetAccountCredit(user.ID)
	if credit.Balance != 7 {
		t.Fatalf("balance after reserve = %d", credit.Balance)
	}
	if used, err := store.DailyCreditUsage(user.ID); err != nil || used != 3 {
		t.Fatalf("daily usage after reserve = %d, %v", used, err)
	}
	if _, err := store.ReserveUsage(ctx, "multimodal", 1); err == nil || !strings.Contains(err.Error(), "今日") {
		t.Fatalf("daily limit not applied: %v", err)
	}
	if err := store.FinishUsage(charge, false); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishUsage(charge, false); err != nil {
		t.Fatal(err)
	}
	credit, _ = store.GetAccountCredit(user.ID)
	if credit.Balance != 10 {
		t.Fatalf("double refund changed balance: %d", credit.Balance)
	}
	if used, err := store.DailyCreditUsage(user.ID); err != nil || used != 0 {
		t.Fatalf("daily usage after refund = %d, %v", used, err)
	}
	entries, err := store.ListCreditLedger(user.ID, 20)
	if err != nil || len(entries) != 3 {
		t.Fatalf("ledger = %v, %v", entries, err)
	}
	if err := store.SetAccountCredit(user.ID, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339), true, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReserveUsage(ctx, "text", 1); err == nil || !strings.Contains(err.Error(), "试用期") {
		t.Fatalf("expired account accepted: %v", err)
	}
}

func TestLocalUnlimitedAdminSkipsPointAccounting(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "local-admin.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	admin, err := store.CreateUser(CreateUserInput{Email: "admin@example.com", PasswordHash: "hash", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	member, err := store.CreateUser(CreateUserInput{Email: "member@example.com", PasswordHash: "hash", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetSoleAdministrator(admin.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.SetLocalUnlimitedAdmin(member.ID); err == nil {
		t.Fatal("member accepted as unlimited admin")
	}
	if err := store.SetLocalUnlimitedAdmin(admin.ID); err != nil {
		t.Fatal(err)
	}
	adminCtx := userctx.WithUser(context.Background(), userctx.User{ID: admin.ID})
	if charge, err := store.ReserveUsage(adminCtx, "image_edit", 1); err != nil || charge != "" {
		t.Fatalf("admin reserve = %q, %v", charge, err)
	}
	credit, err := store.GetAccountCredit(admin.ID)
	if err != nil || !credit.Unlimited || !credit.ManagedEnabled {
		t.Fatalf("admin credit = %+v, %v", credit, err)
	}
	if _, err := store.ReserveUsage(adminCtx, "unknown", 1); err == nil {
		t.Fatal("unsupported capability accepted")
	}
	memberCtx := userctx.WithUser(context.Background(), userctx.User{ID: member.ID})
	if _, err := store.ReserveUsage(memberCtx, "image_edit", 1); err == nil {
		t.Fatal("ordinary account bypassed credits")
	}
}

func TestManagedCreditConcurrentReservationsCannotOverspend(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "concurrent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.CreateManagedUser(CreateUserInput{Email: "many@example.com", PasswordHash: "hash"}, "", 3, 100, "owner")
	if err != nil {
		t.Fatal(err)
	}
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: user.ID})
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := store.ReserveUsage(ctx, "text", 1)
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
				_ = store.FinishUsage(id, true)
			}
		}()
	}
	wg.Wait()
	credit, err := store.GetAccountCredit(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if success != 3 || credit.Balance != 0 {
		t.Fatalf("success=%d balance=%d", success, credit.Balance)
	}
}

func TestManagedCreditPriceChangesApplyToNextReservation(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "price.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.CreateManagedUser(CreateUserInput{Email: "price@example.com", PasswordHash: "hash"}, "", 10, 10, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetCreditPrice("text", 4, "owner"); err != nil {
		t.Fatal(err)
	}
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: user.ID})
	id, err := store.ReserveUsage(ctx, "text", 1)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.FinishUsage(id, true)
	credit, _ := store.GetAccountCredit(user.ID)
	if credit.Balance != 6 || store.CreditPrices()["text"] != 4 {
		t.Fatalf("price not applied: %+v", credit)
	}
}
