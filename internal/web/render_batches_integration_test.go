package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
)

func TestRenderBatchAPIProducesUserOwnedVideo(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	dir := t.TempDir()
	store, err := jobs.OpenStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	uploadDir := filepath.Join(dir, "uploads")
	h := NewHandler(Config{RegistrationMode: "open", UploadDir: uploadDir}, store, storage.NewLocalStore(uploadDir), nil, nil, nil, nil)
	server := httptest.NewServer(h.Routes())
	defer server.Close()
	client := clientWithJar(t)
	user := registerTestUser(t, client, server.URL, "render-owner@example.com", "")
	product, err := store.CreateProduct(jobs.CreateProductInput{Title: "商品", MDPath: filepath.Join(dir, "product.md"), MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimResource(user.ID, "product", product.ID); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "source.mp4")
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=blue:s=360x640:d=1:r=10", "-c:v", "libx264", "-pix_fmt", "yuv420p", source)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("source: %v %s", err, output)
	}
	asset, err := store.CreateProductAsset(jobs.ProductAsset{ProductID: product.ID, Kind: "video", Path: source, OriginalName: "source.mp4", MimeType: "video/mp4"})
	if err != nil {
		t.Fatal(err)
	}
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 720, 1280))); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("product_id", product.ID)
	writer.WriteField("source_asset_id", asset.ID)
	writer.WriteField("title", "测试成片")
	writer.WriteField("variants", `[{"title":"字幕一","copy":"已核对的文案"}]`)
	part, err := writer.CreateFormFile("overlay_0", "overlay.png")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(pngBytes.Bytes())
	writer.Close()
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/render-batches", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusAccepted {
		payload, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("create: %d %s", response.StatusCode, payload)
	}
	var batch jobs.RenderBatch
	if err := json.NewDecoder(response.Body).Decode(&batch); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		response, err = client.Get(server.URL + "/api/render-batches/" + batch.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewDecoder(response.Body).Decode(&batch); err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if batch.Status == "completed" || batch.Status == "needs_review" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if batch.Status != "completed" {
		t.Fatalf("render status: %+v", batch)
	}
	response, err = client.Get(server.URL + "/api/render-batches/" + batch.ID + "/variants/" + batch.Variants[0].ID + "/file")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || len(data) < 1000 {
		t.Fatalf("download: %d %d bytes", response.StatusCode, len(data))
	}
	other := clientWithJar(t)
	registerTestUser(t, other, server.URL, "render-other@example.com", "")
	response, err = other.Get(server.URL + "/api/render-batches/" + batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatalf("cross-user status: %d", response.StatusCode)
	}
}

func TestRenderBatchAcceptsGeneratedVideoAndTimedSubtitles(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	dir := t.TempDir()
	store, err := jobs.OpenStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := NewHandler(Config{RegistrationMode: "open", UploadDir: filepath.Join(dir, "uploads")}, store, storage.NewLocalStore(filepath.Join(dir, "uploads")), nil, nil, nil, nil)
	server := httptest.NewServer(h.Routes())
	defer server.Close()
	client := clientWithJar(t)
	user := registerTestUser(t, client, server.URL, "generated-source@example.com", "")
	product, err := store.CreateProduct(jobs.CreateProductInput{Title: "商品", MDPath: filepath.Join(dir, "product.md"), MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimResource(user.ID, "product", product.ID); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "source.mp4")
	if output, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=blue:s=360x640:d=2:r=10", "-c:v", "libx264", "-pix_fmt", "yuv420p", source).CombinedOutput(); err != nil {
		t.Fatalf("source: %v %s", err, output)
	}
	video, err := store.CreateVideoGeneration(jobs.CreateVideoGenerationInput{UserID: user.ID, ProductID: product.ID, Mode: "image", Prompt: "test", Model: "test", Resolution: "720P", Ratio: "9:16", Duration: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateVideoGeneration(video.ID, "completed", "", "", source, ""); err != nil {
		t.Fatal(err)
	}
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 720, 1280))); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("product_id", product.ID)
	writer.WriteField("source_video_id", video.ID)
	writer.WriteField("variants", `[{"title":"逐句版","segments":[{"start":0.2,"end":1.4,"text":"新文案"}],"cover_existing":true}]`)
	part, err := writer.CreateFormFile("overlay_0_0", "caption.png")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(pngBytes.Bytes())
	writer.Close()
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/render-batches", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("create: %d %s", response.StatusCode, payload)
	}
	var batch jobs.RenderBatch
	if err := json.NewDecoder(response.Body).Decode(&batch); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(server.URL + "/api/render-batches/" + batch.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewDecoder(response.Body).Decode(&batch); err != nil {
			response.Body.Close()
			t.Fatal(err)
		}
		response.Body.Close()
		if batch.Status == "completed" || batch.Status == "needs_review" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if batch.Status != "completed" || batch.SourceVideoID != video.ID || len(batch.Variants[0].Segments) != 1 {
		t.Fatalf("batch: %+v", batch)
	}
}
