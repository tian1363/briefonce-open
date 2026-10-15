package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

type poseRoundTripper func(*http.Request) (*http.Response, error)

func (f poseRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPoseGenerationChecksBothResourceOwners(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.OpenStore(filepath.Join(dir, "pose.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := NewHandler(Config{RegistrationMode: "invite", InviteCodes: []string{"other"}, StaticDir: "../../web/app/public"}, store, storage.NewLocalStore(filepath.Join(dir, "uploads")), nil, nil, nil, nil)
	server := httptest.NewServer(h.Routes())
	defer server.Close()
	ownerClient := clientWithJar(t)
	owner := registerTestUser(t, ownerClient, server.URL, "pose-owner@example.com", "")
	poseResponse := doRequest(t, ownerClient, http.MethodGet, server.URL+"/api/pose-presets", "")
	var presets []posePreset
	if err := json.NewDecoder(poseResponse.Body).Decode(&presets); err != nil {
		t.Fatal(err)
	}
	poseResponse.Body.Close()
	if poseResponse.StatusCode != http.StatusOK || len(presets) != 568 {
		t.Fatalf("pose library = %d, %d", poseResponse.StatusCode, len(presets))
	}
	for _, preset := range presets {
		if preset.Action == "" || preset.Style == "" || preset.Gender == "" {
			t.Fatalf("pose preset missing filter or image: %+v", preset)
		}
		if preset.ImageURL == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join("../../web/app/public", preset.ImageURL)); err != nil {
			t.Fatalf("pose reference missing for %s: %v", preset.ID, err)
		}
	}
	otherClient := clientWithJar(t)
	other := registerTestUser(t, otherClient, server.URL, "pose-other@example.com", "other")
	path := filepath.Join(dir, "reference.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 384, 384))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(jobs.CreateProductInput{Title: "test product", MDPath: path, MDName: "test.md"})
	if err != nil {
		t.Fatal(err)
	}
	mustClaim(t, store, owner.ID, "product", product.ID)
	asset, err := store.CreateProductAsset(jobs.ProductAsset{ProductID: product.ID, Kind: "image", Path: path, OriginalName: "reference.png", MimeType: "image/png"})
	if err != nil {
		t.Fatal(err)
	}
	ownerRole, err := store.CreateCharacter(owner.ID, "owner", "upload", "", "ready", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishCharacter(owner.ID, ownerRole.ID, path, ""); err != nil {
		t.Fatal(err)
	}
	otherRole, err := store.CreateCharacter(other.ID, "other", "upload", "", "ready", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishCharacter(other.ID, otherRole.ID, path, ""); err != nil {
		t.Fatal(err)
	}
	body := `{"product_id":"` + product.ID + `","asset_id":"` + asset.ID + `","character_id":"` + ownerRole.ID + `","pose_id":"front","category":"apparel"}`
	r := doRequest(t, otherClient, http.MethodPost, server.URL+"/api/pose-generations", body)
	r.Body.Close()
	if r.StatusCode != 404 {
		t.Fatalf("other user accessed product: %d", r.StatusCode)
	}
	body = `{"product_id":"` + product.ID + `","asset_id":"` + asset.ID + `","character_id":"` + otherRole.ID + `","pose_id":"front","category":"apparel"}`
	r = doRequest(t, ownerClient, http.MethodPost, server.URL+"/api/pose-generations", body)
	r.Body.Close()
	if r.StatusCode != 404 {
		t.Fatalf("owner accessed foreign model: %d", r.StatusCode)
	}
	body = `{"product_id":"` + product.ID + `","asset_id":"` + asset.ID + `","character_id":"` + ownerRole.ID + `","pose_id":"front","category":"apparel"}`
	r = doRequest(t, ownerClient, http.MethodPost, server.URL+"/api/pose-generations", body)
	r.Body.Close()
	if r.StatusCode != 400 {
		t.Fatalf("missing image config accepted: %d", r.StatusCode)
	}

	for _, extra := range []string{
		`,"supporting_asset_id":"missing","same_style_confirmed":false`,
		`,"supporting_asset_id":"` + asset.ID + `","same_style_confirmed":true`,
		`,"scene":"unknown"`,
	} {
		invalid := strings.TrimSuffix(body, "}") + extra + "}"
		response := doRequest(t, ownerClient, http.MethodPost, server.URL+"/api/pose-generations", invalid)
		response.Body.Close()
		if response.StatusCode != 400 {
			t.Fatalf("invalid structure reference accepted: %d", response.StatusCode)
		}
	}
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	if err := store.ConfigureSecretEncryption(key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveUserModelSettings(owner.ID, jobs.ModelSettings{Capability: "image_edit", Mode: "byok", APIKey: "test-key", Model: "qwen-image-edit-plus", Endpoint: "https://workspace.cn-beijing.maas.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"}); err != nil {
		t.Fatal(err)
	}
	imageBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h.characterImages.HTTP = &http.Client{Transport: poseRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			payload, _ := io.ReadAll(req.Body)
			if strings.Count(string(payload), "data:image/") != 3 {
				t.Errorf("pose request must include identity, product and pose images")
			}
			if !strings.Contains(string(payload), "图三是姿势参考图") {
				t.Errorf("missing pose image role")
			}
		}

		body := []byte(`{"output":{"choices":[{"message":{"content":[{"image":"https://result.aliyuncs.com/pose.png"}]}}]}}`)
		contentType := "application/json"
		if req.Method == http.MethodGet {
			body = imageBytes
			contentType = "image/png"
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{"Content-Type": []string{contentType}}}, nil
	})}
	body = `{"product_id":"` + product.ID + `","asset_id":"` + asset.ID + `","character_id":"` + ownerRole.ID + `","pose_ids":["front","walk"],"category":"apparel"}`
	r = doRequest(t, ownerClient, http.MethodPost, server.URL+"/api/pose-generations", body)
	var batch []jobs.Character
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusAccepted || len(batch) != 2 || batch[0].ID == batch[1].ID {
		t.Fatalf("batch accepted = %d, items = %d", r.StatusCode, len(batch))
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		items, err := store.ListCharacters(owner.ID)
		if err != nil {
			t.Fatal(err)
		}
		ready := 0
		for _, item := range items {
			if (item.ID == batch[0].ID || item.ID == batch[1].ID) && item.Status == "ready" {
				ready++
			}
		}
		if ready == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("batch results did not complete")
		}
		time.Sleep(20 * time.Millisecond)
	}
	var providerCalls atomic.Int32
	h.characterImages.HTTP = &http.Client{Transport: poseRoundTripper(func(req *http.Request) (*http.Response, error) {
		providerCalls.Add(1)
		return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(`{"code":"Model.AccessDenied"}`)), Header: make(http.Header)}, nil
	})}
	r = doRequest(t, ownerClient, http.MethodPost, server.URL+"/api/pose-generations", body)
	var stopped []jobs.Character
	if err := json.NewDecoder(r.Body).Decode(&stopped); err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusAccepted || len(stopped) != 2 {
		t.Fatalf("batch accepted = %d, items = %d", r.StatusCode, len(stopped))
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		items, err := store.ListCharacters(owner.ID)
		if err != nil {
			t.Fatal(err)
		}
		failed := 0
		for _, item := range items {
			if (item.ID == stopped[0].ID || item.ID == stopped[1].ID) && item.Status == "failed" {
				failed++
			}
		}
		if failed == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("model access denial did not stop batch")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if providerCalls.Load() != 1 {
		t.Fatalf("model access denial should stop further requests, got %d", providerCalls.Load())
	}
	secondAsset, err := store.CreateProductAsset(jobs.ProductAsset{ProductID: product.ID, Kind: "image", Path: path, OriginalName: "second.png", MimeType: "image/png"})
	if err != nil {
		t.Fatal(err)
	}
	h.characterImages.HTTP = &http.Client{Transport: poseRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			payload, _ := io.ReadAll(req.Body)
			if strings.Count(string(payload), "data:image/") != 4 {
				t.Errorf("multi-reference request must have 4 images")
			}
		}
		responseBody := []byte(`{"output":{"choices":[{"message":{"content":[{"image":"https://result.aliyuncs.com/pose.png"}]}}]}}`)
		if req.Method == http.MethodGet {
			responseBody = imageBytes
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(responseBody)), Header: make(http.Header)}, nil
	})}
	if err := store.ConfigureSecretEncryption("MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI="); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveImagePrice(jobs.ImagePrice{Model: "gpt-image-2", Tier: "1K", Enabled: true, CostMicros: 300000, MultiplierBPS: 20000, PointMicros: 10000}, "test", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveUserModelSettings(owner.ID, jobs.ModelSettings{Capability: "image_edit", Mode: "byok", Provider: "zexapi", APIKey: "test-key", Model: "gpt-image-2", Endpoint: "https://zexapi.com/v1/videos"}); err != nil {
		t.Fatal(err)
	}
	body = `{"product_id":"` + product.ID + `","asset_ids":["` + asset.ID + `","` + secondAsset.ID + `"],"character_id":"` + ownerRole.ID + `","pose_ids":["front","walk"],"category":"apparel"}`
	r = doRequest(t, ownerClient, http.MethodPost, server.URL+"/api/pose-generations", body)
	var combined []jobs.Character
	if err := json.NewDecoder(r.Body).Decode(&combined); err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusAccepted || len(combined) != 2 {
		t.Fatalf("multi-asset batch = %d, items=%d", r.StatusCode, len(combined))
	}
	for _, item := range combined {
		var controls map[string]string
		if err := json.Unmarshal(item.Controls, &controls); err != nil {
			t.Fatal(err)
		}
		var selected []string
		if err := json.Unmarshal([]byte(controls["asset_ids"]), &selected); err != nil {
			t.Fatal(err)
		}
		if len(selected) != 2 || selected[0] != asset.ID || selected[1] != secondAsset.ID {
			t.Fatalf("references lost: %v", selected)
		}
	}
	queued, err := store.PendingProviderImages()
	if err != nil || len(queued) != 2 {
		t.Fatalf("expected one queued image per pose: %d %v", len(queued), err)
	}
	for _, task := range queued {
		if strings.Count(task.RequestJSON, "data:image/") != 4 {
			t.Fatal("missing shared product references")
		}
	}
	body = `{"product_id":"` + product.ID + `","asset_ids":["` + asset.ID + `","` + asset.ID + `"],"character_id":"` + ownerRole.ID + `","pose_ids":["front"],"category":"apparel"}`
	r = doRequest(t, ownerClient, http.MethodPost, server.URL+"/api/pose-generations", body)
	r.Body.Close()
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("duplicate product image accepted: %d", r.StatusCode)
	}
	thirdAsset, err := store.CreateProductAsset(jobs.ProductAsset{ProductID: product.ID, Kind: "image", Path: path, OriginalName: "third.png", MimeType: "image/png"})
	if err != nil {
		t.Fatal(err)
	}
	body = `{"product_id":"` + product.ID + `","asset_ids":["` + asset.ID + `","` + secondAsset.ID + `","` + thirdAsset.ID + `"],"character_id":"` + ownerRole.ID + `","pose_ids":["front","walk","side","back","hold","seated","lean"],"category":"apparel"}`
	r = doRequest(t, ownerClient, http.MethodPost, server.URL+"/api/pose-generations", body)
	r.Body.Close()
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("batch above 6 poses accepted: %d", r.StatusCode)
	}
}

func TestBuildPosePromptKeepsProductAsOnlyVisualReference(t *testing.T) {
	for _, category := range []string{"apparel", "bag", "handheld"} {
		prompt := buildPosePrompt(posePresets[0], category)
		for _, required := range []string{"图二是唯一商品视觉依据", "图三是姿势参考图", "仅参考身体朝向", "严禁沿用图三的人物身份", posePresets[0].Instruction, "优先保留商品真实外观"} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("%s prompt missing %q", category, required)
			}
		}
		if strings.Contains(prompt, "不会作为图像输入") {
			t.Fatalf("%s prompt still references pose image", category)
		}
	}
}

func TestSupportedPosePromptSeparatesTargetColorAndStructure(t *testing.T) {
	p := buildSupportedPosePrompt(posePresets[0], "apparel", "courtyard", true)
	for _, text := range []string{"图二确定目标商品颜色", "图三是用户已确认的同款", "禁止采用图三颜色", "图四是姿势参考图", "严禁沿用图四的人物身份", "AI 推测", "石材庭院", posePresets[0].Instruction} {
		if !strings.Contains(p, text) {
			t.Fatalf("missing %s", text)
		}
	}
	if strings.Contains(p, "图二是唯一商品视觉依据") {
		t.Fatal("conflicting reference roles")
	}
	colored := buildSupportedPosePrompt(posePresets[0], "apparel", "courtyard", true, "浅米白色")
	if !strings.Contains(colored, "浅米白色") || !strings.Contains(colored, "绝不输出结构参考的另一种颜色") {
		t.Fatal("explicit target color missing")
	}
	ordinary := buildSupportedPosePrompt(posePresets[0], "apparel", "studio", false)
	if ordinary != buildPosePrompt(posePresets[0], "apparel") {
		t.Fatal("legacy studio prompt changed")
	}
}

func TestLoadPoseReferenceRequiresRealLocalImage(t *testing.T) {
	data, err := loadPoseReference("../../web/app/public", posePresets[0])
	if err != nil || len(data) == 0 {
		t.Fatalf("missing catalog image: %v", err)
	}
	for _, url := range []string{"", "https://example.com/image.png", "/pose-gallery/../../secret", "/pose-gallery/missing.png"} {
		if _, err := loadPoseReference("../../web/app/public", posePreset{ImageURL: url}); err == nil {
			t.Fatalf("accepted unavailable reference %q", url)
		}
	}
}

func TestMultiPosePromptSeparatesAllImageRoles(t *testing.T) {
	for _, support := range []bool{false, true} {
		p := buildMultiPosePrompt(posePresets[0], "apparel", "studio", support, "黑色", 2)
		poseNumber := 4
		if support {
			poseNumber = 5
		}
		if !strings.Contains(p, fmt.Sprintf("第%d张图是姿势参考图", poseNumber)) || !strings.Contains(p, "图三至第3张图是同一商品的补充参考") {
			t.Fatal(p)
		}
		if support && !strings.Contains(p, "第4张图是用户已确认的同款") {
			t.Fatal(p)
		}
	}
}
