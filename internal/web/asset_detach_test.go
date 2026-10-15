package web

import (
	"bytes"
	"encoding/json"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDetachAssetPreservesWorkAndFile(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.OpenStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := NewHandler(Config{RegistrationMode: "invite", InviteCodes: []string{"other"}}, store, storage.NewLocalStore(filepath.Join(dir, "uploads")), nil, nil, nil, nil)
	server := httptest.NewServer(h.Routes())
	defer server.Close()
	ownerClient := clientWithJar(t)
	owner := registerTestUser(t, ownerClient, server.URL, "detach-owner@example.com", "")
	otherClient := clientWithJar(t)
	registerTestUser(t, otherClient, server.URL, "detach-other@example.com", "other")
	path := filepath.Join(dir, "work.png")
	var data bytes.Buffer
	_ = png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 10, 10)))
	if err = os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(jobs.CreateProductInput{Title: "coat", MDPath: path, MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	mustClaim(t, store, owner.ID, "product", product.ID)
	asset, err := store.CreateProductAsset(jobs.ProductAsset{ProductID: product.ID, Kind: "image", Path: path, OriginalName: "work.png", MimeType: "image/png"})
	if err != nil {
		t.Fatal(err)
	}
	work, err := store.CreateCharacter(owner.ID, "work", "pose", "", "generating", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.FinishCharacter(owner.ID, work.ID, path, ""); err != nil {
		t.Fatal(err)
	}
	endpoint := server.URL + "/api/assets/" + asset.ID
	response := doRequest(t, otherClient, http.MethodPost, endpoint+"/detach", "")
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatal("another account detached asset")
	}
	var upload bytes.Buffer
	writer := multipart.NewWriter(&upload)
	part, _ := writer.CreateFormFile("asset", "replacement.png")
	var replacement bytes.Buffer
	_ = png.Encode(&replacement, image.NewRGBA(image.Rect(0, 0, 20, 20)))
	_, _ = part.Write(replacement.Bytes())
	_ = writer.Close()
	response, err = otherClient.Post(endpoint+"/replace", writer.FormDataContentType(), bytes.NewReader(upload.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatal("another account replaced image")
	}
	response, err = ownerClient.Post(endpoint+"/replace", writer.FormDataContentType(), bytes.NewReader(upload.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	var updated jobs.ProductAsset
	if err = json.NewDecoder(response.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 || updated.ID == asset.ID || updated.OriginalName != "replacement.png" {
		t.Fatalf("replace failed: %d %+v", response.StatusCode, updated)
	}
	current, err := store.ListProductAssets(product.ID)
	if err != nil || len(current) != 1 || current[0].ID != updated.ID {
		t.Fatal("replacement duplicated or lost product attachment")
	}
	if store.CurrentProductAssetID(asset.ID) != updated.ID {
		t.Fatal("observation replacement mapping lost")
	}
	original, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, data.Bytes()) {
		t.Fatal("replacement overwrote historical source")
	}
	response, err = ownerClient.Post(endpoint+"/replace", writer.FormDataContentType(), bytes.NewReader(upload.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 400 {
		t.Fatal("stale replacement accepted")
	}
	endpoint = server.URL + "/api/assets/" + updated.ID
	for i := 0; i < 2; i++ {
		response = doRequest(t, ownerClient, http.MethodPost, endpoint+"/detach", "")
		response.Body.Close()
		if response.StatusCode != 204 {
			t.Fatalf("detach: %d", response.StatusCode)
		}
	}
	response = doRequest(t, ownerClient, http.MethodGet, server.URL+"/api/products/"+product.ID+"/assets", "")
	var listed []jobs.ProductAsset
	if err = json.NewDecoder(response.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if len(listed) != 0 {
		t.Fatal("asset still attached")
	}
	for _, url := range []string{endpoint + "/file", server.URL + "/api/characters/" + work.ID + "/image"} {
		response = doRequest(t, ownerClient, http.MethodGet, url, "")
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatalf("historical file lost: %s %d", url, response.StatusCode)
		}
	}
	preserved, err := store.GetCharacter(owner.ID, work.ID)
	if err != nil || preserved.Status != "ready" {
		t.Fatal("work record changed")
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal("source file removed")
	}
}
