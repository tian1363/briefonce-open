package jobs

import (
	"path/filepath"
	"testing"
	"time"
)

func TestArtworkHistoryOwnershipFilteringAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "works.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	user, err := s.CreateManagedUser(CreateUserInput{Email: "works@example.test", PasswordHash: "hash"}, time.Now().Add(time.Hour).Format(time.RFC3339), 1000, 1000, "owner")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateProduct(CreateProductInput{Title: "外套"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ClaimResource(user.ID, "product", p.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 52; i++ {
		if _, err = s.CreateProductAsset(ProductAsset{ProductID: p.ID, Kind: "artwork", Path: "detail.jpg", OriginalName: "详情图.jpg", MimeType: "image/jpeg"}); err != nil {
			t.Fatal(err)
		}
	}
	role, err := s.CreateCharacter(user.ID, "姿势", "pose", "", "ready", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishCharacter(user.ID, role.ID, "pose.jpg", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateProductAsset(ProductAsset{ProductID: p.ID, Kind: "image", Path: "pose.jpg", OriginalName: "姿势.jpg"}); err != nil {
		t.Fatal(err)
	}
	// Ordinary uploaded sources must not appear as generated work.
	if _, err = s.CreateProductAsset(ProductAsset{ProductID: p.ID, Kind: "image", Path: "upload.jpg"}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.ListArtworks(user.ID, p.ID, 0)
	if err != nil || len(first) != 49 {
		t.Fatalf("first: %d %v", len(first), err)
	}
	rest, err := s.ListArtworks(user.ID, p.ID, 48)
	if err != nil || len(rest) != 5 {
		t.Fatalf("rest: %d %v", len(rest), err)
	}
	seen := map[string]bool{}
	poses := 0
	for _, a := range append(first[:48], rest...) {
		if seen[a.ID] {
			t.Fatal("duplicate work")
		}
		seen[a.ID] = true
		if a.Source == "pose" {
			poses++
			if a.AssetID == "" {
				t.Fatal("missing reusable pose asset")
			}
		}
	}
	if poses != 1 {
		t.Fatalf("pose duplicated or missing: %d", poses)
	}
	for _, filter := range []string{"other-product", "unassigned"} {
		items, err := s.ListArtworks(user.ID, filter, 0)
		if err != nil || len(items) != 0 {
			t.Fatalf("filter %s: %+v %v", filter, items, err)
		}
	}
	hidden, err := s.ListArtworks("other-user", "", 0)
	if err != nil || len(hidden) != 0 {
		t.Fatalf("foreign works exposed: %+v %v", hidden, err)
	}
}
