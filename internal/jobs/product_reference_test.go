package jobs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestAppendProductReferencePreservesFactsAndRetries(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(filepath.Join(dir, "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	owner, err := s.CreateUser(CreateUserInput{Email: "reference-owner@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateUser(CreateUserInput{Email: "reference-other@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "product.md")
	os.WriteFile(path, []byte("# Confirmed product facts\n"), 0600)
	p, err := s.CreateProduct(CreateProductInput{Title: "coat", MDPath: path, MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	s.ClaimResource(owner.ID, "product", p.ID)
	if s.AppendProductReference(other.ID, p.ID, "unauthorized") == nil {
		t.Fatal("foreign owner wrote reference")
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for retry := 0; retry < 2; retry++ {
				if err := s.AppendProductReference(owner.ID, p.ID, fmt.Sprintf("## AI reference %d", i)); err != nil {
					t.Error(err)
				}
			}
		}(i)
	}
	wg.Wait()
	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), "# Confirmed product facts\n") || strings.Count(string(data), "## AI reference") != 4 {
		t.Fatalf("lost original or duplicated reference: %s", data)
	}
}
