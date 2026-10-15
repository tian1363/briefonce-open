package charactergen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/tian1363/scriptagent/internal/imagepolicy"
	"github.com/tian1363/scriptagent/internal/model"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEditReferencesSendsModelAndProductInOrder(t *testing.T) {
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 384, 384))); err != nil {
		t.Fatal(err)
	}
	client := New()
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		body := "image-bytes"
		if r.Method == "POST" {
			var req struct {
				Input struct {
					Messages []struct {
						Content []map[string]string `json:"content"`
					} `json:"messages"`
				} `json:"input"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			content := req.Input.Messages[0].Content
			if len(content) != 3 || !strings.HasPrefix(content[0]["image"], "data:image/png;base64,") || !strings.HasPrefix(content[1]["image"], "data:image/png;base64,") || content[2]["text"] != "person wears product" {
				t.Fatalf("wrong multi-image content: %+v", content)
			}
			body = `{"output":{"choices":[{"message":{"content":[{"image":"https://results.oss-cn-beijing.aliyuncs.com/result.png"}]}}]}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	cfg := model.RuntimeConfig{APIKey: "test", Provider: "dashscope", Model: "qwen-image-edit-plus", Endpoint: "https://workspace.cn-beijing.maas.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"}
	if _, err := client.EditReferences(context.Background(), cfg, "person wears product", "", "960*1280", [][]byte{picture.Bytes(), picture.Bytes()}); err != nil {
		t.Fatal(err)
	}
}

type transport func(*http.Request) (*http.Response, error)

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }
func TestReferenceControlsReachProvider(t *testing.T) {
	controls := Controls{Age: 28, Description: "must not change face", Hair: "ignored hairstyle", Outfit: "blue shirt", Size: "720*1280", Seed: 42}
	calls := 0
	client := New()
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := "image-bytes"
		if r.Method == "POST" {
			var req struct {
				Parameters map[string]any `json:"parameters"`
				Input      struct {
					Messages []struct {
						Content []map[string]string `json:"content"`
					} `json:"messages"`
				} `json:"input"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req.Parameters["seed"] != float64(42) || req.Parameters["prompt_extend"] != false || req.Parameters["size"] != "720*1280" {
				t.Fatalf("wrong controls: %+v", req.Parameters)
			}
			content := req.Input.Messages[0].Content
			if !strings.Contains(content[1]["text"], imagepolicy.NaturalSkin) {
				t.Fatal("natural skin requirements did not reach the image provider")
			}
			if !strings.HasPrefix(content[0]["image"], "data:") || strings.Contains(content[1]["text"], "ignored hairstyle") {
				t.Fatal("reference not preserved")
			}
			body = `{"output":{"choices":[{"message":{"content":[{"image":"https://results.oss-cn-beijing.aliyuncs.com/result.png"}]}}]}}`
		} else if r.Header.Get("Authorization") != "" {
			t.Fatal("key leaked to download")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	cfg := model.RuntimeConfig{APIKey: "test", Provider: "dashscope", Model: "qwen-image-edit-plus", Endpoint: "https://workspace.cn-beijing.maas.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"}
	out, err := client.Generate(context.Background(), cfg, controls, []byte("ref-image"))
	if err != nil || string(out) != "image-bytes" || calls != 2 {
		t.Fatalf("result %q %v calls %d", out, err, calls)
	}
}
func TestCharacterBoundaries(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1/test", "https://aliyuncs.com.evil.test/x", "https://test.aliyuncs.com@evil.test/x", "https://test.aliyuncs.com:8080/x"} {
		if TrustedURL(u) {
			t.Fatal(u)
		}
	}
	c := Controls{Age: 28, Size: "720*1280", Seed: 42}
	if c.Validate() != nil {
		t.Fatal("valid controls rejected")
	}
	c.Seed = -1
	if c.Validate() == nil {
		t.Fatal("negative seed accepted")
	}
	cfg := model.RuntimeConfig{APIKey: "test", Provider: "openai", Model: "other"}
	if _, err := New().Generate(context.Background(), cfg, Controls{Age: 28, Size: "720*1280"}, nil); err == nil {
		t.Fatal("unsupported provider accepted")
	}
}

func TestGeneratedImageDownloadRetriesTransientFailure(t *testing.T) {
	client := New()
	attempts := 0
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		attempts++
		status, body := http.StatusServiceUnavailable, "busy"
		if attempts == 2 {
			status, body = http.StatusOK, "image-bytes"
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	data, err := client.downloadGeneratedImage(context.Background(), "https://results.oss-cn-beijing.aliyuncs.com/result.png")
	if err != nil || string(data) != "image-bytes" || attempts != 2 {
		t.Fatalf("download = %q, %v, attempts=%d", data, err, attempts)
	}
}

func TestImageEndpointAndProviderErrorMapping(t *testing.T) {
	cfg := model.RuntimeConfig{APIKey: "test", Provider: "dashscope", Model: "qwen-image-edit-plus", Endpoint: "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"}
	if _, err := New().EditReference(context.Background(), cfg, "edit", "", []byte("reference")); err == nil || !strings.Contains(err.Error(), "工作空间") {
		t.Fatalf("legacy public endpoint should be rejected clearly: %v", err)
	}
	if err := imageServiceError(http.StatusNotFound, "ModelNotFound"); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("404 model error = %v", err)
	}
	if err := imageServiceError(http.StatusTooManyRequests, "Throttling.RateQuota"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("429 rate error = %v", err)
	}
	if err := imageServiceError(http.StatusTooManyRequests, "CommodityNotPurchased"); errors.Is(err, ErrRateLimited) || !strings.Contains(err.Error(), "开通") {
		t.Fatalf("429 purchase error = %v", err)
	}
	for _, code := range []string{"Model.AccessDenied", "Workspace.AccessDenied", "AccessDenied.Unpurchased", ""} {
		err := imageServiceError(http.StatusForbidden, code)
		if !errors.Is(err, ErrAccessDenied) || !strings.Contains(err.Error(), "图片模型") {
			t.Fatalf("403 %s error = %v", code, err)
		}
	}
}
