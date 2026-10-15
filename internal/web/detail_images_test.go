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

	"github.com/tian1363/scriptagent/internal/detailpage"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
)

func TestDetailImagesOwnershipIdempotencyRetryAndBilling(t *testing.T) {
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
	endpoint := server.URL + "/api/products/" + product.ID + "/detail-images"
	p := detailpage.Plan{Input: detailpage.Input{Facts: detailpage.Facts{Title: "衬衫"}, Assets: []detailpage.Asset{{ID: asset.ID}}, PlatformIDs: []string{"taobao"}}, Driver: "visual", StyleLock: "暖白背景，柔光"}
	for _, id := range []string{"hero", "benefits", "details", "specs", "usage"} {
		p.Pages = append(p.Pages, detailpage.Page{ID: id, SceneID: "hero", Purpose: "展示已确认商品", SourceAssetID: asset.ID, Headline: "衬衫", Visual: "完整商品与简短标题", Missing: []string{}, Evidence: []string{"title"}})
	}
	payload := func(key string) string {
		raw, _ := json.Marshal(map[string]any{"plan": p, "request_id": key, "confirmed": true, "platform_id": "taobao", "page_id": "hero"})
		return string(raw)
	}
	for _, method := range []string{"GET", "POST"} {
		r := doRequest(t, otherClient, method, endpoint, payload("foreign"))
		r.Body.Close()
		if r.StatusCode != 404 {
			t.Fatal("ownership", r.StatusCode)
		}
	}
	if err := store.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveUserModelSettings(owner.ID, jobs.ModelSettings{Capability: "image_edit", Mode: "byok", APIKey: "test-key", Model: "qwen-image-edit-plus", Endpoint: "https://workspace.cn-beijing.maas.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var fail atomic.Bool
	h.characterImages.HTTP = &http.Client{Transport: poseRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.Method == "GET" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(source.Bytes())), Header: make(http.Header)}, nil
		}
		calls.Add(1)
		if fail.Load() {
			return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(`{"message":"test failure"}`)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"output":{"choices":[{"message":{"content":[{"image":"https://result.aliyuncs.com/product.png"}]}}]}}`)), Header: make(http.Header)}, nil
	})}
	post := func(body string, code int) []jobs.DetailImageJob {
		t.Helper()
		r := doRequest(t, ownerClient, "POST", endpoint, body)
		defer r.Body.Close()
		if r.StatusCode != code {
			data, _ := io.ReadAll(r.Body)
			t.Fatalf("status %d want %d: %s", r.StatusCode, code, data)
		}
		var list []jobs.DetailImageJob
		if err := json.NewDecoder(r.Body).Decode(&list); err != nil {
			t.Fatal(err)
		}
		return list
	}
	wait := func(id, status string) jobs.DetailImageJob {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			j, e := store.GetDetailImageJob(owner.ID, product.ID, id)
			if e == nil && j.Status == status {
				return j
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("job timeout", status)
		return jobs.DetailImageJob{}
	}
	body := payload("first")
	created := post(body, 202)
	again := post(body, 200)
	if created[0].ID != again[0].ID {
		t.Fatal("duplicate task")
	}
	ready := wait(created[0].ID, "ready")
	if calls.Load() != 1 {
		t.Fatal("duplicate provider calls")
	}
	out, err := store.GetProductAsset(ready.OutputAssetID)
	if err != nil || out.Kind != "artwork" {
		t.Fatal("artwork not saved", err)
	}
	r := doRequest(t, ownerClient, "GET", server.URL+"/api/credits/image?asset_id="+out.ID, "")
	var activity jobs.UsageActivity
	json.NewDecoder(r.Body).Decode(&activity)
	r.Body.Close()
	if activity.TaskID != ready.ID || activity.Source != "byok" || activity.TaskStatus != "ready" {
		t.Fatalf("missing usage %+v", activity)
	}
	p.Pages[0].Headline = "changed"
	r = doRequest(t, ownerClient, "POST", endpoint, payload("first"))
	r.Body.Close()
	if r.StatusCode != 400 {
		t.Fatal("key reuse accepted")
	}
	p.Pages[0].Missing = []string{"核对商品细节"}
	r = doRequest(t, ownerClient, "POST", endpoint, payload("missing"))
	r.Body.Close()
	if r.StatusCode != 400 || calls.Load() != 1 {
		t.Fatal("unresolved facts dispatched")
	}
	p.Pages[0].Missing = nil
	fail.Store(true)
	failed := post(payload("failure"), 202)
	old := wait(failed[0].ID, "failed")
	fail.Store(false)
	raw, _ := json.Marshal(map[string]any{"retry_job_id": old.ID, "request_id": "retry", "confirmed": true})
	retry := post(string(raw), 202)
	done := wait(retry[0].ID, "ready")
	if done.Task.Prompt != old.Task.Prompt {
		t.Fatal("retry prompt changed")
	}
	// Whole-page mode submits one model request and checks missing content from all used sections.
	p.RenderMode = "whole"
	p.Pages[2].Missing = []string{"核对细节"}
	wholeBody := func(key string) string {
		raw, _ := json.Marshal(map[string]any{"plan": p, "request_id": key, "confirmed": true, "platform_id": "taobao"})
		return string(raw)
	}
	before := calls.Load()
	r = doRequest(t, ownerClient, "POST", endpoint, wholeBody("whole-missing"))
	r.Body.Close()
	if r.StatusCode != 400 || calls.Load() != before {
		t.Fatal("whole page bypassed unresolved details")
	}
	p.Pages[2].Missing = nil
	whole := post(wholeBody("whole"), 202)
	if len(whole) != 1 || whole[0].Task.PageID != "whole" {
		t.Fatal("whole page split into modules")
	}
	wait(whole[0].ID, "ready")
	if calls.Load() != before+1 {
		t.Fatal("whole page used multiple model requests")
	}
	duplicate := post(wholeBody("whole"), 200)
	if duplicate[0].ID != whole[0].ID || calls.Load() != before+1 {
		t.Fatal("whole-page duplicate charged or submitted")
	}

}
