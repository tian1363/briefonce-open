package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
)

func TestTranscribeAudioSendsExtractedAudioAndReturnsTimedReviewFlags(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	video := filepath.Join(t.TempDir(), "source.mp4")
	if output, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=blue:s=64x64:d=1:r=10", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:v", "libx264", "-c:a", "aac", "-shortest", video).CombinedOutput(); err != nil {
		t.Fatalf("video: %v %s", err, output)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization missing")
		}
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			t.Errorf("multipart: %v", err)
		}
		if r.FormValue("response_format") != "verbose_json" || r.FormValue("timestamp_granularities[]") != "segment" {
			t.Errorf("format fields: %+v", r.Form)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("audio missing: %v", err)
		} else {
			file.Close()
		}
		json.NewEncoder(w).Encode(map[string]any{"segments": []map[string]any{{"start": .2, "end": .8, "text": " 测试一句", "avg_logprob": -1.2}}})
	}))
	defer server.Close()
	t.Setenv("SCRIPT_AGENT_ASR_ENDPOINT", server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	segments, err := transcribeAudio(ctx, video, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 1 || segments[0].Text != "测试一句" || !segments[0].NeedsReview {
		t.Fatalf("segments: %+v", segments)
	}
}

func TestTranscriptionUsesAccountSpeechSettings(t *testing.T) {
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
	if err := store.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(Config{RegistrationMode: "open", UploadDir: filepath.Join(dir, "uploads")}, store, storage.NewLocalStore(filepath.Join(dir, "uploads")), nil, nil, nil, nil)
	app := httptest.NewServer(h.Routes())
	defer app.Close()
	client := clientWithJar(t)
	user := registerTestUser(t, client, app.URL, "asr-settings@example.com", "")
	product, err := store.CreateProduct(jobs.CreateProductInput{Title: "商品", MDPath: filepath.Join(dir, "product.md"), MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimResource(user.ID, "product", product.ID); err != nil {
		t.Fatal(err)
	}
	videoPath := filepath.Join(dir, "source.mp4")
	if output, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=blue:s=64x64:d=1:r=10", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:v", "libx264", "-c:a", "aac", "-shortest", videoPath).CombinedOutput(); err != nil {
		t.Fatalf("video: %v %s", err, output)
	}
	asset, err := store.CreateProductAsset(jobs.ProductAsset{ProductID: product.ID, Kind: "video", Path: videoPath, OriginalName: "source.mp4", MimeType: "video/mp4"})
	if err != nil {
		t.Fatal(err)
	}
	transcriber := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer account-key" {
			t.Error("account key not used")
		}
		json.NewEncoder(w).Encode(map[string]any{"segments": []map[string]any{{"start": 0, "end": .9, "text": "广告口播"}}})
	}))
	defer transcriber.Close()
	if _, err := store.SaveUserModelSettings(user.ID, jobs.ModelSettings{Capability: "speech_recognition", Mode: "byok", APIKey: "account-key", Provider: "openai", Endpoint: transcriber.URL, Model: "whisper-1"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SCRIPT_AGENT_ASR_API_KEY", "")
	data, _ := json.Marshal(map[string]string{"product_id": product.ID, "source_asset_id": asset.ID})
	response, err := client.Post(app.URL+"/api/render-batches/transcribe", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("status %d: %s", response.StatusCode, payload)
	}
	var result struct {
		Segments []recognizedSegment `json:"segments"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Segments) != 1 || result.Segments[0].Text != "广告口播" {
		t.Fatalf("result: %+v", result)
	}
}
