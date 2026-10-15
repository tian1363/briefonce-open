package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
)

func TestFlaggedExtractionStaysPrivateUntilOwnerApproves(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.OpenStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := NewHandler(Config{RegistrationMode: "invite", InviteCodes: []string{"other"}, StaticDir: "../../web/app/public"}, store, storage.NewLocalStore(filepath.Join(dir, "uploads")), nil, nil, nil, nil)
	server := httptest.NewServer(h.Routes())
	defer server.Close()
	ownerClient := clientWithJar(t)
	owner := registerTestUser(t, ownerClient, server.URL, "review-owner@example.com", "")
	otherClient := clientWithJar(t)
	registerTestUser(t, otherClient, server.URL, "review-other@example.com", "other")
	framePath := filepath.Join(dir, "frame.jpg")
	imagePath := filepath.Join(dir, "candidate.jpg")
	for _, path := range []string{framePath, imagePath} {
		if err := os.WriteFile(path, []byte("test image"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	item, err := store.CreateAssetExtraction(jobs.AssetExtraction{UserID: owner.ID, Kind: "product", Label: "coat", SourceType: "product_asset", SourceID: "source", FramePath: framePath})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteAssetExtractionForReview(owner.ID, item.ID, framePath, imagePath, "待核对", "可能新增标识", []jobs.AssetExtractionVariant{{View: "main", AtSeconds: 1, FramePath: framePath, ImagePath: imagePath}}); err != nil {
		t.Fatal(err)
	}
	base := server.URL + "/api/asset-extractions/" + item.ID
	product, err := store.CreateProduct(jobs.CreateProductInput{Title: "test coat", MDPath: framePath, MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	mustClaim(t, store, owner.ID, "product", product.ID)
	response := doRequest(t, ownerClient, http.MethodPost, base+"/import", `{"product_id":"`+product.ID+`"}`)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("flagged image was importable before review: %d", response.StatusCode)
	}
	for _, path := range []string{"/image", "/variants/0/image"} {
		response = doRequest(t, otherClient, http.MethodGet, base+path, "")
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("another user read flagged candidate %s: %d", path, response.StatusCode)
		}
		response = doRequest(t, ownerClient, http.MethodGet, base+path, "")
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("owner could not review candidate %s: %d", path, response.StatusCode)
		}
	}
	response = doRequest(t, otherClient, http.MethodPost, base+"/accept-review", "")
	response.Body.Close()
	if response.StatusCode == http.StatusOK {
		t.Fatal("another user approved candidate")
	}
	response = doRequest(t, ownerClient, http.MethodPost, base+"/accept-review", "")
	var approved jobs.AssetExtraction
	if err := json.NewDecoder(response.Body).Decode(&approved); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || approved.Status != "ready" {
		t.Fatalf("approval result = %d %+v", response.StatusCode, approved)
	}
	person, err := store.CreateAssetExtraction(jobs.AssetExtraction{UserID: owner.ID, Kind: "person", Label: "model", SourceType: "product_asset", SourceID: "source", FramePath: framePath})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteAssetExtractionForReview(owner.ID, person.ID, framePath, imagePath, "待核对", "可能新增标识", nil); err != nil {
		t.Fatal(err)
	}
	response = doRequest(t, ownerClient, http.MethodPost, server.URL+"/api/asset-extractions/"+person.ID+"/accept-review", "")
	var approvedPerson jobs.AssetExtraction
	if err := json.NewDecoder(response.Body).Decode(&approvedPerson); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || approvedPerson.TargetID == "" {
		t.Fatalf("approved person not saved to role library: %d %+v", response.StatusCode, approvedPerson)
	}
	role, err := store.GetCharacter(owner.ID, approvedPerson.TargetID)
	if err != nil || role.Status != "ready" || role.Path != imagePath {
		t.Fatalf("reviewed role = %+v, %v", role, err)
	}
}
