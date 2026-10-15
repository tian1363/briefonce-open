package jobs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductObservationAppendIsOwnedAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(filepath.Join(dir, "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner, err := store.CreateUser(CreateUserInput{Email: "doc-owner@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateUser(CreateUserInput{Email: "doc-other@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "product.md")
	if err := os.WriteFile(path, []byte("# Verified product facts\n"), 0600); err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(CreateProductInput{Title: "Camera", MDPath: path, MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimResource(owner.ID, "product", product.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteProductObservation(other.ID, product.ID, "ex-1", "unauthorized"); err == nil {
		t.Fatal("other user wrote document")
	}
	for i := 0; i < 2; i++ {
		if err := store.WriteProductObservation(owner.ID, product.ID, "ex-1", "## Visible\nBlue camera"); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# Verified product facts") || strings.Contains(string(data), "Blue camera") {
		t.Fatalf("unexpected document: %s", data)
	}
	if err := store.WriteProductDocument(owner.ID, product.ID, "# Revised product facts"); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# Revised product facts") || strings.Contains(string(data), "Blue camera") {
		t.Fatalf("observation lost after stale edit: %s", data)
	}
	observation, err := store.ReadProductObservation(owner.ID, product.ID, "ex-1")
	if err != nil || !strings.Contains(observation, "Blue camera") {
		t.Fatalf("observation missing: %v %s", err, observation)
	}
	if err := store.WriteProductDocument(other.ID, product.ID, "clobber"); err == nil {
		t.Fatal("other user replaced document")
	}
}
