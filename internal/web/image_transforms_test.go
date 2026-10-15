package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
)

func TestProductImageTransformBatchOwnershipAndResults(t *testing.T) {
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
	owner := registerTestUser(t, ownerClient, server.URL, "image-owner@example.com", "")
	otherClient := clientWithJar(t)
	registerTestUser(t, otherClient, server.URL, "image-other@example.com", "other")
	var source bytes.Buffer
	if err := png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 384, 384))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "source.png")
	if err := os.WriteFile(path, source.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(jobs.CreateProductInput{Title: "hanging coat", MDPath: path, MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	mustClaim(t, store, owner.ID, "product", product.ID)
	asset, err := store.CreateProductAsset(jobs.ProductAsset{ProductID: product.ID, Kind: "image", Path: path, OriginalName: "source.png", MimeType: "image/png", SizeBytes: int64(source.Len())})
	if err != nil {
		t.Fatal(err)
	}
	endpoint := server.URL + "/api/products/" + product.ID + "/image-transforms"
	body := `{"source_asset_id":"` + asset.ID + `","modes":["studio_white","visible_detail"],"detail_focus":"领口"}`
	for _, target := range []string{endpoint, endpoint + "?x=1"} {
		r := doRequest(t, otherClient, http.MethodGet, target, "")
		r.Body.Close()
		if r.StatusCode != http.StatusNotFound {
			t.Fatalf("foreign product jobs visible: %d", r.StatusCode)
		}
	}
	r := doRequest(t, otherClient, http.MethodPost, endpoint, body)
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("foreign product generation accepted: %d", r.StatusCode)
	}
	r = doRequest(t, ownerClient, http.MethodPost, endpoint, `{"source_asset_id":"`+asset.ID+`","modes":["studio_white","studio_white"]}`)
	r.Body.Close()
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("duplicate modes accepted: %d", r.StatusCode)
	}
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	if err := store.ConfigureSecretEncryption(key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveUserModelSettings(owner.ID, jobs.ModelSettings{Capability: "image_edit", Mode: "byok", APIKey: "test-key", Model: "qwen-image-edit-plus", Endpoint: "https://workspace.cn-beijing.maas.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"}); err != nil {
		t.Fatal(err)
	}
	var providerCalls atomic.Int32
	h.characterImages.HTTP = &http.Client{Transport: poseRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(source.Bytes())), Header: make(http.Header)}, nil
		}
		providerCalls.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"output":{"choices":[{"message":{"content":[{"image":"https://result.aliyuncs.com/product.png"}]}}]}}`)), Header: make(http.Header)}, nil
	})}
	r = doRequest(t, ownerClient, http.MethodPost, endpoint, body)
	var created []jobs.ImageTransformJob
	if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusAccepted || len(created) != 2 || created[0].BatchID != created[1].BatchID {
		t.Fatalf("create batch: %d %+v", r.StatusCode, created)
	}
	waitImageJobs(t, store, owner.ID, product.ID, created, "ready")
	if providerCalls.Load() != 2 {
		t.Fatalf("provider calls = %d", providerCalls.Load())
	}
	jobsList, err := store.ListImageTransformJobs(owner.ID, product.ID)
	if err != nil || len(jobsList) != 2 {
		t.Fatalf("saved jobs = %d, %v", len(jobsList), err)
	}
	for _, item := range jobsList {
		if item.OutputAssetID == "" {
			t.Fatal("generated result missing product asset")
		}
		output, err := store.GetProductAsset(item.OutputAssetID)
		if err != nil || output.ProductID != product.ID {
			t.Fatalf("generated asset missing or wrong product: %v", err)
		}
		usageURL := server.URL + "/api/credits/image?asset_id=" + item.OutputAssetID
		res := doRequest(t, ownerClient, http.MethodGet, usageURL, "")
		var usage jobs.UsageActivity
		if err := json.NewDecoder(res.Body).Decode(&usage); err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK || usage.TaskID != item.ID || usage.Source != "byok" || usage.Points != 0 || usage.Model == "" {
			t.Fatalf("image billing details: %+v", usage)
		}
		res = doRequest(t, otherClient, http.MethodGet, usageURL, "")
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("foreign billing details visible: %d", res.StatusCode)
		}
	}
	visualRef, err := store.CreateProductAsset(jobs.ProductAsset{ProductID: product.ID, Kind: "reference", Path: path, OriginalName: "style.png", MimeType: "image/png", SizeBytes: int64(source.Len())})
	if err != nil {
		t.Fatal(err)
	}
	r = doRequest(t, ownerClient, http.MethodPost, endpoint, `{"source_asset_id":"`+asset.ID+`","modes":["studio_white"],"reference_asset_id":"`+asset.ID+`","reference_focus":"参考留白"}`)
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("product photo accepted as visual reference: %d", r.StatusCode)
	}
	var referenceInputs atomic.Int32
	h.characterImages.HTTP = &http.Client{Transport: poseRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(source.Bytes())), Header: make(http.Header)}, nil
		}
		var payload struct {
			Input struct {
				Messages []struct {
					Content []map[string]string `json:"content"`
				} `json:"messages"`
			} `json:"input"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		content := payload.Input.Messages[0].Content
		if len(content) != 3 || !strings.HasPrefix(content[0]["image"], "data:image/png;base64,") || !strings.HasPrefix(content[1]["image"], "data:image/png;base64,") || !strings.Contains(content[2]["text"], "参考留白") {
			t.Errorf("visual reference did not reach provider: %+v", content)
		}
		referenceInputs.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"output":{"choices":[{"message":{"content":[{"image":"https://result.aliyuncs.com/product.png"}]}}]}}`)), Header: make(http.Header)}, nil
	})}
	r = doRequest(t, ownerClient, http.MethodPost, endpoint, `{"source_asset_id":"`+asset.ID+`","modes":["studio_white"],"reference_asset_id":"`+visualRef.ID+`","reference_focus":"参考留白"}`)
	var referenced []jobs.ImageTransformJob
	if err := json.NewDecoder(r.Body).Decode(&referenced); err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusAccepted || len(referenced) != 1 {
		t.Fatalf("reference batch: %d %+v", r.StatusCode, referenced)
	}
	waitImageJobs(t, store, owner.ID, product.ID, referenced, "ready")
	if referenceInputs.Load() != 1 || referenced[0].ReferenceAssetID != visualRef.ID || referenced[0].ReferenceFocus != "参考留白" {
		t.Fatalf("reference was not saved or sent: %+v", referenced)
	}
	providerCalls.Store(0)
	h.characterImages.HTTP = &http.Client{Transport: poseRoundTripper(func(req *http.Request) (*http.Response, error) {
		providerCalls.Add(1)
		return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(`{"code":"Model.AccessDenied"}`)), Header: make(http.Header)}, nil
	})}
	r = doRequest(t, ownerClient, http.MethodPost, endpoint, body)
	var denied []jobs.ImageTransformJob
	if err := json.NewDecoder(r.Body).Decode(&denied); err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusAccepted || len(denied) != 2 {
		t.Fatalf("denied batch: %d %+v", r.StatusCode, denied)
	}
	waitImageJobs(t, store, owner.ID, product.ID, denied, "failed")
	if providerCalls.Load() != 1 {
		t.Fatalf("permission failure should stop the remaining job; calls=%d", providerCalls.Load())
	}
}

func waitImageJobs(t *testing.T, store *jobs.Store, userID, productID string, batch []jobs.ImageTransformJob, wanted string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		items, err := store.ListImageTransformJobs(userID, productID)
		if err != nil {
			t.Fatal(err)
		}
		matched := 0
		for _, item := range items {
			for _, target := range batch {
				if item.ID == target.ID && item.Status == wanted {
					matched++
				}
			}
		}
		if matched == len(batch) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("jobs did not reach %s: %+v", wanted, items)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
