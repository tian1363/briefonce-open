package model

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type referenceRoundTripper func(*http.Request) (*http.Response, error)

type referenceConfigProvider struct{ runtime RuntimeConfig }

func (provider referenceConfigProvider) GetModelRuntimeConfig(context.Context, string) (RuntimeConfig, error) {
	return provider.runtime, nil
}

func (f referenceRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestAnalyzeReferenceVideoUsesAudioVideoModel(t *testing.T) {
	t.Setenv("SCRIPT_AGENT_REFERENCE_OMNI_MODEL", "")
	called := false
	client := NewDashScopeClient(DashScopeConfig{APIKey: "test-key", Model: "qwen3.8-omni-flash"})
	client.httpClient = &http.Client{Transport: referenceRoundTripper(func(request *http.Request) (*http.Response, error) {
		called = true
		if request.URL.Path != "/compatible-mode/v1/chat/completions" {
			t.Fatalf("wrong endpoint: %s", request.URL)
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Content []struct {
					Type     string `json:"type"`
					FPS      int    `json:"fps"`
					VideoURL struct {
						URL string `json:"url"`
					} `json:"video_url"`
				} `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "qwen3.8-omni-flash" || len(body.Messages) != 1 || len(body.Messages[0].Content) != 2 || body.Messages[0].Content[0].FPS != 4 || body.Messages[0].Content[0].VideoURL.URL != "data:video/mp4;base64,dGVzdA==" {
			t.Fatalf("wrong audio-video request: %+v", body)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"旁白：无；BGM：有"}}],"usage":{"total_tokens":25}}`))}, nil
	})}
	result, err := client.AnalyzeReferenceVideo(context.Background(), CallContext{Step: "reference_video_analysis"}, "data:video/mp4;base64,dGVzdA==", 4, "核对声音")
	if err != nil || !called || !strings.Contains(result.Text, "BGM：有") {
		t.Fatalf("audio-video analysis failed: %+v %v", result, err)
	}
	if strings.Contains(result.RequestJSON, "dGVzdA==") {
		t.Fatal("video bytes leaked into recorded request")
	}
}

func TestReferenceOmniEndpointRejectsOtherProviders(t *testing.T) {
	if _, err := referenceOmniEndpoint("https://example.com/v1/chat/completions"); err == nil {
		t.Fatal("must not send account key to an unrelated host")
	}
}

func TestParseOmniStreamingResponse(t *testing.T) {
	text, usage, err := parseOmniResponse([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"旁白：\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"无\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"total_tokens\":12}}\n\ndata: [DONE]\n"))
	if err != nil || text != "旁白：无" || usage.TotalTokens != 12 {
		t.Fatalf("stream parse failed: %q %+v %v", text, usage, err)
	}
}

func TestKimiK3ReceivesVideoAndPromptWithoutClaimingAudio(t *testing.T) {
	client := NewDashScopeClient(DashScopeConfig{Provider: referenceConfigProvider{runtime: RuntimeConfig{
		APIKey: "kimi-test", Provider: "kimi", Endpoint: "https://api.moonshot.cn/v1/chat/completions", Model: "kimi-k3", Source: "byok",
	}}})
	client.httpClient = &http.Client{Transport: referenceRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "api.moonshot.cn" {
			t.Fatalf("video sent to unexpected host: %s", request.URL)
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Content []struct {
					Type     string `json:"type"`
					VideoURL struct {
						URL string `json:"url"`
					} `json:"video_url"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "kimi-k3" || body.Messages[0].Content[0].VideoURL.URL != "data:video/mp4;base64,dGVzdA==" || !strings.Contains(body.Messages[0].Content[1].Text, "未确认") {
			t.Fatalf("incorrect Kimi visual request: %+v", body)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"分镜表"}}]}`))}, nil
	})}
	result, err := client.AnalyzeReferenceVideo(context.Background(), CallContext{}, "data:video/mp4;base64,dGVzdA==", 4, "拆解视频")
	if err != nil || result.Model != "kimi-k3" || strings.Contains(result.RequestJSON, "dGVzdA==") {
		t.Fatalf("Kimi video request failed or leaked media: %+v %v", result, err)
	}
}

func TestBailianKimiK3UsesImagesAndOmniForAudio(t *testing.T) {
	videoPath := filepath.Join(t.TempDir(), "clip.mp4")
	if output, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=red:s=64x64:r=2:d=1", "-c:v", "mpeg4", videoPath).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg unavailable: %v %s", err, output)
	}
	client := NewDashScopeClient(DashScopeConfig{Provider: referenceConfigProvider{runtime: RuntimeConfig{
		APIKey: "bailian-test", Provider: "dashscope", Endpoint: "https://workspace.cn-beijing.maas.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation", Model: "kimi-k3", Source: "byok",
	}}})
	visualCalled, audioCalled := false, false
	client.httpClient = &http.Client{Transport: referenceRoundTripper(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		if request.URL.Path == "/compatible-mode/v1/chat/completions" {
			audioCalled = true
			if !strings.Contains(string(body), `"model":"qwen3.8-omni-flash"`) || !strings.Contains(string(body), `"type":"video_url"`) {
				t.Fatalf("incorrect Omni audio request: %s", body)
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"BGM：有；旁白：无"}}]}`))}, nil
		}
		visualCalled = true
		if !strings.Contains(string(body), `"model":"kimi-k3"`) || !strings.Contains(string(body), `"image":"data:image/jpeg;base64,`) || strings.Contains(string(body), `"video":`) {
			t.Fatalf("incorrect K3 frame request: %s", body)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"output":{"choices":[{"message":{"content":[{"text":"分镜表"}]}}]}}`))}, nil
	})}
	result, err := client.AnalyzeReferenceVideoEvidence(context.Background(), CallContext{}, videoPath, "data:video/mp4;base64,dGVzdA==", 2, "拆解视频")
	if err != nil || !visualCalled || !audioCalled || !strings.Contains(result.Text, "BGM：有") || strings.Contains(result.RequestJSON, "data:image/jpeg") {
		t.Fatalf("Bailian K3 analysis failed: %+v %v", result, err)
	}
}

func TestBailianKimiK26SendsVideoThenChecksAudio(t *testing.T) {
	client := NewDashScopeClient(DashScopeConfig{Provider: referenceConfigProvider{runtime: RuntimeConfig{
		APIKey: "bailian-test", Provider: "dashscope", Endpoint: "https://workspace.cn-beijing.maas.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation", Model: "kimi-k2.6", Source: "byok",
	}}})
	calls := 0
	client.httpClient = &http.Client{Transport: referenceRoundTripper(func(request *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(request.Body)
		if request.URL.Path != "/compatible-mode/v1/chat/completions" {
			t.Fatalf("wrong Kimi endpoint: %s", request.URL)
		}
		if calls == 1 {
			if !strings.Contains(string(body), `"model":"kimi-k2.6"`) || !strings.Contains(string(body), `"type":"video_url"`) || !strings.Contains(string(body), `"fps":4`) {
				t.Fatalf("incorrect K2.6 video request: %s", body)
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"画面分镜"}}]}`))}, nil
		}
		if !strings.Contains(string(body), `"model":"qwen3.8-omni-flash"`) {
			t.Fatalf("wrong audio model: %s", body)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"旁白：有"}}]}`))}, nil
	})}
	result, err := client.AnalyzeReferenceVideoEvidence(context.Background(), CallContext{}, "", "data:video/mp4;base64,dGVzdA==", 4, "拆解视频")
	if err != nil || calls != 2 || !strings.Contains(result.Text, "画面分镜") || !strings.Contains(result.Text, "旁白：有") || strings.Contains(result.RequestJSON, "dGVzdA==") {
		t.Fatalf("Kimi K2.6 analysis failed: %+v calls=%d err=%v", result, calls, err)
	}
}
