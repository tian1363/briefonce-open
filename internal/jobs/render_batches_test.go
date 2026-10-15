package jobs

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderBatchPersistsAndScopesVariants(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.CreateUser(CreateUserInput{Email: "render@example.com", PasswordHash: "x", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(CreateProductInput{Title: "商品", MDPath: "/tmp/product.md", MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimResource(user.ID, "product", product.ID); err != nil {
		t.Fatal(err)
	}
	asset, err := store.CreateProductAsset(ProductAsset{ProductID: product.ID, Kind: "video", Path: "/tmp/source.mp4", OriginalName: "source.mp4", MimeType: "video/mp4"})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := store.CreateRenderBatch(user.ID, product.ID, asset.ID, "测试批次", []RenderVariant{{Title: "A", Copy: "文案 A", OverlayPath: "/tmp/a.png"}, {Title: "B", Copy: "文案 B", OverlayPath: "/tmp/b.png"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Variants) != 2 {
		t.Fatalf("variants: %d", len(batch.Variants))
	}
	if _, err := store.GetRenderBatch("other-user", batch.ID); err == nil {
		t.Fatal("cross-user batch read succeeded")
	}
	if err := store.SetRenderVariant(batch.Variants[0].ID, "completed", "/tmp/a.mp4", ""); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetRenderBatch(user.ID, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Variants[0].Status != "completed" || got.Variants[1].Status != "pending" {
		t.Fatalf("unexpected variants: %+v", got.Variants)
	}
	if _, err := store.RetryRenderVariant(user.ID, batch.ID, batch.Variants[0].ID); err == nil {
		t.Fatal("completed variant was overwritten")
	}
	if err := store.SetRenderVariant(batch.Variants[1].ID, "failed", "", "temporary"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRenderBatchStatus(batch.ID, "needs_review"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RetryRenderVariant(user.ID, batch.ID, batch.Variants[1].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RetryRenderVariant(user.ID, batch.ID, batch.Variants[1].ID); err == nil {
		t.Fatal("duplicate retry accepted")
	}
}

func TestRenderBatchPersistsTimedSegmentsWithoutExposingPaths(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.CreateUser(CreateUserInput{Email: "timed@example.com", PasswordHash: "x", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(CreateProductInput{Title: "商品", MDPath: "/tmp/product.md", MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := store.CreateRenderBatchWithSource(user.ID, product.ID, "", "video-1", "", "", "逐句", []RenderVariant{{Title: "A", Copy: "逐句字幕", Segments: []CaptionSegment{{Start: 0.2, End: 0.8, Text: "第一句", OverlayPath: "/private/one.png"}}, CoverExisting: true}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetRenderBatch(user.ID, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceVideoID != "video-1" || !got.Variants[0].CoverExisting || len(got.Variants[0].Segments) != 1 || got.Variants[0].Segments[0].OverlayPath != "/private/one.png" {
		t.Fatalf("not persisted: %+v", got)
	}
	data, _ := json.Marshal(got)
	if strings.Contains(string(data), "/private/one.png") {
		t.Fatal("local overlay path leaked in API JSON")
	}
}
