package jobs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/tian1363/scriptagent/internal/charactergen"
	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/userctx"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func opsStore(t *testing.T) (*Store, *User, context.Context) {
	t.Helper()
	s, e := OpenStore(filepath.Join(t.TempDir(), "ops.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	u, e := s.CreateManagedUser(CreateUserInput{Email: "ops@example.com", PasswordHash: "hash"}, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), 1000, 10000, "owner")
	if e != nil {
		t.Fatal(e)
	}
	return s, u, userctx.WithUser(context.Background(), userctx.User{ID: u.ID})
}
func validPrice() ImagePrice {
	return ImagePrice{Model: "gpt-image-2", Tier: "1K", Enabled: true, CostMicros: 300000, MultiplierBPS: 20000, PointMicros: 10000}
}
func TestImageQuoteVersionsAndRefundLots(t *testing.T) {
	s, u, ctx := opsStore(t)
	if err := s.SaveImagePrice(validPrice(), "owner", "账单成本"); err != nil {
		t.Fatal(err)
	}
	o := charactergen.Options{Model: "gpt-image-2"}
	p, n, e := s.ImageQuote(o)
	if e != nil || n != 60 {
		t.Fatalf("quote %v %d", e, n)
	}
	id, e := s.ReserveImageUsage(ctx, "image_edit", o)
	if e != nil {
		t.Fatal(e)
	}
	p.CostMicros = 900000
	if e = s.SaveImagePrice(p, "owner", "涨价"); e != nil {
		t.Fatal(e)
	}
	var points int64
	var raw string
	if e = s.db.QueryRow(`SELECT points,pricing_json FROM credit_charges WHERE id=?`, id).Scan(&points, &raw); e != nil {
		t.Fatal(e)
	}
	var snap ImagePrice
	_ = json.Unmarshal([]byte(raw), &snap)
	if points != 60 || snap.CostMicros != 300000 {
		t.Fatal("price snapshot changed")
	}
	if e = s.FinishUsage(id, false); e != nil {
		t.Fatal(e)
	}
	if e = s.FinishUsage(id, false); e != nil {
		t.Fatal(e)
	}
	credit, _ := s.GetAccountCredit(u.ID)
	if credit.Balance != 1000 {
		t.Fatal("refund duplicated or missing")
	}
	var sum int64
	_ = s.db.QueryRow(`SELECT SUM(remaining) FROM credit_lots WHERE user_id=?`, u.ID).Scan(&sum)
	if sum != 1000 {
		t.Fatalf("lots %d", sum)
	}
}
func TestPurchaseConfirmationReceiptUniquenessAndRefund(t *testing.T) {
	s, u, _ := opsStore(t)
	p := Package{ID: "p", Name: "100元套餐", AmountMicros: 100000000, PaidPoints: 10000, BonusPoints: 2000, Active: true}
	if e := s.SavePackage(p, "owner", "初始套餐"); e != nil {
		t.Fatal(e)
	}
	id, e := s.CreatePurchase(u.ID, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := s.ConfirmPurchase(id, "receipt-1", "owner"); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	credit, _ := s.GetAccountCredit(u.ID)
	if credit.Balance != 13000 {
		t.Fatalf("duplicated issuance %d", credit.Balance)
	}
	id2, e := s.CreatePurchase(u.ID, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.ConfirmPurchase(id2, "receipt-1", "owner"); e == nil {
		t.Fatal("duplicate receipt accepted")
	}
	if e = s.RefundPurchase(id, "owner", "refund-receipt"); e != nil {
		t.Fatal(e)
	}
	if e = s.RefundPurchase(id, "owner", "refund-receipt"); e != nil {
		t.Fatal(e)
	}
	credit, _ = s.GetAccountCredit(u.ID)
	if credit.Balance != 1000 {
		t.Fatal("refund changed unrelated balance")
	}
}
func TestPriceInputValidation(t *testing.T) {
	p := validPrice()
	p.MultiplierBPS = 20000
	for _, mutate := range []func(*ImagePrice){func(p *ImagePrice) { p.CostMicros = -1 }, func(p *ImagePrice) { p.PointMicros = 0 }, func(p *ImagePrice) { p.MultiplierBPS = 1000001 }, func(p *ImagePrice) { p.FixedPoints = -1 }} {
		bad := p
		mutate(&bad)
		if _, e := bad.Points(); e == nil {
			t.Fatal("invalid price accepted")
		}
	}
	p.CostMicros = 300001
	if n, e := p.Points(); e != nil || n != 61 {
		t.Fatalf("ceiling %d %v", n, e)
	}
}
func TestProviderQueueEncryptionAndAmbiguousBilling(t *testing.T) {
	s, u, ctx := opsStore(t)
	if e := s.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))); e != nil {
		t.Fatal(e)
	}
	if e := s.SaveImagePrice(validPrice(), "owner", "test"); e != nil {
		t.Fatal(e)
	}
	o := charactergen.Options{Model: "gpt-image-2"}
	id, e := s.ReserveImageUsage(ctx, "image_edit", o)
	if e != nil {
		t.Fatal(e)
	}
	e = s.QueueProviderImage(ProviderImageJob{ID: "j", UserID: u.ID, Kind: "character", ChargeID: id, Capability: "image_edit"}, model.RuntimeConfig{APIKey: "secret-test", Source: "managed", Provider: "zexapi", Model: o.Model, Endpoint: "https://zexapi.com/v1/videos"}, map[string]any{"prompt": "private input"}, o)
	if e != nil {
		t.Fatal(e)
	}
	var key, body string
	_ = s.db.QueryRow(`SELECT credential,request_json FROM image_provider_jobs WHERE id='j'`).Scan(&key, &body)
	if strings.Contains(key, "secret-test") || strings.Contains(body, "private input") {
		t.Fatal("sensitive queue plaintext")
	}
	if e = s.FinishUsage(id, false); e != nil {
		t.Fatal(e)
	}
	var status string
	_ = s.db.QueryRow(`SELECT status FROM credit_charges WHERE id=?`, id).Scan(&status)
	if status != "reserved" {
		t.Fatal("pending refunded")
	}
	_ = s.UpdateProviderImage("j", "failed", "", 0, "", "")
	if e = s.FinishUsage(id, false); e != nil {
		t.Fatal(e)
	}
	credit, _ := s.GetAccountCredit(u.ID)
	if credit.Balance != 1000 {
		t.Fatal("definite failure not refunded")
	}
	lines := []StatementLine{{Account: "acct", ExternalID: "bill", TaskID: "task", AmountMicros: 300000, Evidence: "csv"}}
	if n, e := s.ImportStatements(lines, "owner"); e != nil || n != 1 {
		t.Fatalf("import %v %d", e, n)
	}
	if n, e := s.ImportStatements(lines, "owner"); e != nil || n != 0 {
		t.Fatal("import duplicated")
	}
}
func TestPurchasedLotsRevenueAndRestart(t *testing.T) {
	s, u, ctx := opsStore(t)
	if _, e := s.AdjustCredits(u.ID, "owner", -1000, "clear trial"); e != nil {
		t.Fatal(e)
	}
	p := Package{ID: "p", Name: "p", AmountMicros: 10000000, PaidPoints: 1000, BonusPoints: 200, Active: true}
	if e := s.SavePackage(p, "owner", "test"); e != nil {
		t.Fatal(e)
	}
	id, e := s.CreatePurchase(u.ID, "p")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.ConfirmPurchase(id, "r", "owner"); e != nil {
		t.Fatal(e)
	}
	if e = s.SaveImagePrice(validPrice(), "owner", "test"); e != nil {
		t.Fatal(e)
	}
	charge, e := s.ReserveImageUsage(ctx, "image_edit", charactergen.Options{Model: "gpt-image-2"})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.FinishUsage(charge, true); e != nil {
		t.Fatal(e)
	}
	summary, e := s.RevenueSummary()
	if e != nil {
		t.Fatal(e)
	}
	if summary["usage_revenue_micros"].(int64) != 600000 {
		t.Fatalf("revenue %v", summary)
	}
	if e = s.RefundPurchase(id, "owner", "refund"); e != nil {
		t.Fatal(e)
	}
	var refund int64
	if e = s.db.QueryRow(`SELECT refund_micros FROM purchase_orders WHERE id=?`, id).Scan(&refund); e != nil || refund != 9400000 {
		t.Fatalf("partial refund %d %v", refund, e)
	}
	if e = s.migrateOperations(); e != nil {
		t.Fatal(e)
	}
	var sum int64
	_ = s.db.QueryRow(`SELECT SUM(remaining) FROM credit_lots WHERE user_id=?`, u.ID).Scan(&sum)
	credit, _ := s.GetAccountCredit(u.ID)
	if sum != credit.Balance {
		t.Fatalf("migration duplicated lot %d %d", sum, credit.Balance)
	}
}
func TestProviderResultAssetIsIdempotent(t *testing.T) {
	s, u, _ := opsStore(t)
	product, err := s.CreateProduct(CreateProductInput{Title: "商品"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveImagePrice(validPrice(), "owner", "test"); err != nil {
		t.Fatal(err)
	}
	o := charactergen.Options{Model: "gpt-image-2"}
	cfg := model.RuntimeConfig{Provider: "zexapi", Model: o.Model, APIKey: "test", Source: "managed"}
	if err = s.QueueProviderImage(ProviderImageJob{ID: "asset-job", UserID: u.ID, ProductID: product.ID, Kind: "pose", Capability: "image_edit"}, cfg, map[string]any{"prompt": "test"}, o); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveProviderImagePath("asset-job", "/test.png", ""); err != nil {
		t.Fatal(err)
	}
	input := ProductAsset{ProductID: product.ID, Kind: "image", Path: "/test.png", OriginalName: "test.png", MimeType: "image/png", SizeBytes: 1}
	a, err := s.CreateProviderImageAsset("asset-job", input)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateProviderImageAsset("asset-job", input)
	if err != nil || a.ID != b.ID {
		t.Fatalf("asset duplicated %v", err)
	}
	rows, _ := s.ListProductAssets(product.ID)
	if len(rows) != 1 {
		t.Fatal("duplicate assets")
	}
}
