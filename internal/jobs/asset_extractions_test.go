package jobs

import (
	"path/filepath"
	"testing"
)

func TestAssetExtractionLifecycleIsOwnerScoped(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "assets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner, err := store.CreateUser(CreateUserInput{Email: "asset-owner@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateUser(CreateUserInput{Email: "asset-other@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateAssetExtraction(AssetExtraction{UserID: owner.ID, Kind: "person", Label: "reference", SourceType: "chat_attachment", SourceID: "source", FramePath: "/private/frame.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAssetExtraction(other.ID, item.ID); err == nil {
		t.Fatal("other user read task")
	}
	if list, err := store.ListAssetExtractions(other.ID); err != nil || len(list) != 0 {
		t.Fatalf("other user list = %v, %v", list, err)
	}
	if list, err := store.ListAssetExtractions(owner.ID); err != nil || len(list) != 1 || list[0].ID != item.ID {
		t.Fatalf("owner task list = %v, %v", list, err)
	}
	if err := store.FinishAssetExtraction(other.ID, item.ID, "ready", item.FramePath, "/private/image.png", "target", ""); err == nil {
		t.Fatal("other user finished task")
	}
	if err := store.FinishAssetExtraction(owner.ID, item.ID, "ready", item.FramePath, "/private/image.png", "target", ""); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetAssetExtraction(owner.ID, item.ID)
	if err != nil || got.Status != "ready" || got.TargetID != "target" {
		t.Fatalf("finished task = %+v, %v", got, err)
	}
	if err := store.FinishAssetExtraction(owner.ID, item.ID, "failed", "", "", "", ""); err == nil {
		t.Fatal("finished task changed again")
	}
}

func TestAssetExtractionReviewRequiresOwnerAndExplicitApproval(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "assets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner, err := store.CreateUser(CreateUserInput{Email: "review-owner@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateUser(CreateUserInput{Email: "review-other@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateAssetExtraction(AssetExtraction{UserID: owner.ID, Kind: "product", Label: "reference", SourceType: "product_asset", SourceID: "source", FramePath: "/private/frame.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	variants := []AssetExtractionVariant{{View: "main", AtSeconds: 2.5, FramePath: "/private/frame.jpg", ImagePath: "/private/candidate.png"}}
	if err := store.CompleteAssetExtractionForReview(owner.ID, item.ID, variants[0].FramePath, variants[0].ImagePath, "待核对", "可能新增标识", variants); err != nil {
		t.Fatal(err)
	}
	if err := store.ApproveAssetExtractionReview(other.ID, item.ID); err == nil {
		t.Fatal("another user approved a candidate")
	}
	got, err := store.GetAssetExtraction(owner.ID, item.ID)
	if err != nil || got.Status != "needs_review" || got.Error != "可能新增标识" || len(got.Variants) != 1 {
		t.Fatalf("review task = %+v, %v", got, err)
	}
	if err := store.ApproveAssetExtractionReview(owner.ID, item.ID); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetAssetExtraction(owner.ID, item.ID)
	if err != nil || got.Status != "ready" || got.Error != "" || len(got.Variants) != 1 {
		t.Fatalf("approved task = %+v, %v", got, err)
	}
	if err := store.ApproveAssetExtractionReview(owner.ID, item.ID); err == nil {
		t.Fatal("review was approved twice")
	}
}
