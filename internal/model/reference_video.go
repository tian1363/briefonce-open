package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/tian1363/scriptagent/internal/mediaanalysis"
)

var ErrReferenceAudioUnavailable = errors.New("当前模型服务不支持参考视频声音理解")

// AnalyzeReferenceVideoEvidence sends timestamped still frames to Bailian Kimi K3.
// K3 on Bailian accepts images but not a video file. Audio is checked in a
// separate Omni request using the same Bailian credential.
func (c *DashScopeClient) AnalyzeReferenceVideoEvidence(ctx context.Context, callCtx CallContext, path, videoDataURL string, fps int, prompt string) (Generation, error) {
	runtime := c.runtimeConfig(ctx, "multimodal")
	if runtime.Provider == "dashscope" && strings.EqualFold(runtime.Model, "kimi-k2.6") {
		visual, err := c.analyzeBailianKimiVideo(ctx, callCtx, runtime, videoDataURL, fps, prompt)
		if err != nil {
			return Generation{}, err
		}
		return c.appendReferenceAudio(ctx, callCtx, videoDataURL, fps, visual), nil
	}
	if runtime.Provider != "dashscope" || !strings.EqualFold(runtime.Model, "kimi-k3") {
		return c.AnalyzeReferenceVideo(ctx, callCtx, videoDataURL, fps, prompt)
	}
	frames, err := mediaanalysis.SampleFrames(ctx, path, fps, 72)
	if err != nil {
		return Generation{}, fmt.Errorf("Kimi K3 抽取参考视频画面失败: %w", err)
	}
	content := make([]ContentItem, 0, len(frames)*2+1)
	content = append(content, ContentItem{Text: prompt + "\n以下图片按时间顺序从原片抽取；每张前面是时间戳。仅据画面判断，声音一律留待后续音轨核对。"})
	for _, frame := range frames {
		content = append(content, ContentItem{Text: fmt.Sprintf("原片约 %.2f 秒", frame.Second)}, ContentItem{Image: frame.DataURL})
	}
	visual, err := c.GenerateDetailed(ctx, callCtx, content)
	if err != nil {
		return Generation{}, err
	}
	return c.appendReferenceAudio(ctx, callCtx, videoDataURL, fps, visual), nil
}

func (c *DashScopeClient) appendReferenceAudio(ctx context.Context, callCtx CallContext, videoDataURL string, fps int, visual Generation) Generation {
	audioPrompt := "只核对原片可听声音：逐段判断旁白/对话、BGM、音效/环境声的有无及时间；听不清写未确认。不要根据字幕推断声音，也不要重写画面分镜。"
	audio, audioErr := c.AnalyzeReferenceVideo(ctx, callCtx, videoDataURL, fps, audioPrompt)
	if audioErr == nil {
		visual.Text += "\n\n## 音轨核对（Omni）\n" + audio.Text
		visual.PromptTokens += audio.PromptTokens
		visual.OutputTokens += audio.OutputTokens
		visual.TotalTokens += audio.TotalTokens
	} else {
		visual.Text += "\n\n## 音轨核对\n旁白、对话、BGM 与音效均未确认；音轨分析不可用，不得从画面字幕推断声音。"
	}
	return visual
}

func (c *DashScopeClient) analyzeBailianKimiVideo(ctx context.Context, callCtx CallContext, runtime RuntimeConfig, videoDataURL string, fps int, prompt string) (Generation, error) {
	endpoint, err := referenceOmniEndpoint(runtime.Endpoint)
	if err != nil {
		return Generation{}, err
	}
	finish, err := c.beginUsage(ctx, runtime, "multimodal", 1, callCtx)
	if err != nil {
		return Generation{}, err
	}
	success := false
	defer func() { finish(success) }()
	if fps < 1 {
		fps = 2
	}
	request := map[string]any{"model": runtime.Model, "messages": []any{map[string]any{"role": "user", "content": []any{
		map[string]any{"type": "video_url", "video_url": map[string]string{"url": videoDataURL}, "fps": fps},
		map[string]any{"type": "text", "text": prompt + "\n仅据画面拆解；旁白、BGM 和音效留待后续音轨核对。"},
	}}}}
	raw, err := json.Marshal(request)
	if err != nil {
		return Generation{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return Generation{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+runtime.APIKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	started := time.Now()
	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return Generation{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 20<<20))
	if err != nil {
		return Generation{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Generation{}, fmt.Errorf("Kimi K2.6 参考视频分析失败（%d）", response.StatusCode)
	}
	textResult, usage, err := parseOmniResponse(body)
	if err != nil {
		return Generation{}, err
	}
	logRequest, _ := json.Marshal(map[string]any{"model": runtime.Model, "prompt": prompt, "video": "[video data omitted]"})
	result := Generation{Text: textResult, Model: runtime.Model, RequestJSON: string(logRequest), ResponseJSON: string(sanitizedResponseJSON(body)), PromptTokens: usage.PromptTokens, OutputTokens: usage.CompletionTokens, TotalTokens: usage.TotalTokens, LatencyMS: elapsedMS(started)}
	c.record(ctx, callCtx, runtime.Model, logRequest, sanitizedResponseJSON(body), result, nil)
	success = true
	return result, nil
}

// AnalyzeReferenceVideo uses an audio-video model when the account's
// multimodal credential belongs to DashScope. Its result can identify audible
// speech and music, unlike the visual video sampler used as a fallback.
func (c *DashScopeClient) AnalyzeReferenceVideo(ctx context.Context, callCtx CallContext, videoDataURL string, fps int, prompt string) (generation Generation, callErr error) {
	if fps <= 0 {
		fps = 2
	}
	runtime := c.runtimeConfig(ctx, "multimodal")
	if isKimiVideoRuntime(runtime) && runtime.APIKey != "" {
		finish, err := c.beginUsage(ctx, runtime, "multimodal", 1, callCtx)
		if err != nil {
			return Generation{}, err
		}
		defer func() { finish(callErr == nil) }()
		return c.generateOpenAICompatible(ctx, callCtx, runtime, []ContentItem{
			{Video: videoDataURL},
			{Text: prompt + "\n你只可依据可见画面判断。若模型不能听取该视频音轨，旁白、对话、BGM 与音效一律写未确认；不能把字幕当作可听旁白。"},
		})
	}
	if runtime.Provider != "dashscope" || runtime.APIKey == "" {
		return Generation{}, ErrReferenceAudioUnavailable
	}
	if len(videoDataURL) >= 10*1024*1024 {
		return Generation{}, ErrReferenceAudioUnavailable
	}
	endpoint, err := referenceOmniEndpoint(runtime.Endpoint)
	if err != nil {
		return Generation{}, ErrReferenceAudioUnavailable
	}
	modelName := strings.TrimSpace(os.Getenv("SCRIPT_AGENT_REFERENCE_OMNI_MODEL"))
	if modelName == "off" {
		return Generation{}, ErrReferenceAudioUnavailable
	}
	if modelName == "" {
		if strings.Contains(runtime.Model, "omni") {
			modelName = runtime.Model
		} else if strings.EqualFold(runtime.Model, "kimi-k3") || strings.EqualFold(runtime.Model, "kimi-k2.6") {
			modelName = "qwen3.8-omni-flash"
		} else {
			return Generation{}, ErrReferenceAudioUnavailable
		}
	}
	finish, err := c.beginUsage(ctx, runtime, "multimodal", 1, callCtx)
	if err != nil {
		return Generation{}, err
	}
	defer func() { finish(callErr == nil) }()
	request := map[string]any{
		"model":          modelName,
		"modalities":     []string{"text"},
		"stream":         true,
		"stream_options": map[string]bool{"include_usage": true},
		"messages": []any{map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "video_url", "video_url": map[string]string{"url": videoDataURL}, "fps": fps},
				map[string]any{"type": "text", "text": prompt},
			},
		}},
	}
	if strings.HasPrefix(modelName, "qwen3.8-") {
		request["reasoning_effort"] = "none"
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return Generation{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return Generation{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+runtime.APIKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	started := time.Now()
	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return Generation{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 20<<20))
	if err != nil {
		return Generation{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Generation{}, fmt.Errorf("参考视频声音分析失败（%d）", response.StatusCode)
	}
	textResult, usage, err := parseOmniResponse(body)
	if err != nil {
		return Generation{}, err
	}
	result := Generation{
		Text: textResult, Model: modelName,
		PromptTokens: usage.PromptTokens, OutputTokens: usage.CompletionTokens,
		TotalTokens: usage.TotalTokens, LatencyMS: elapsedMS(started),
	}
	logRequest, _ := json.Marshal(map[string]any{"model": modelName, "prompt": prompt, "video": "[video data omitted]"})
	result.RequestJSON = string(logRequest)
	result.ResponseJSON = string(sanitizedResponseJSON(body))
	c.record(ctx, callCtx, modelName, logRequest, sanitizedResponseJSON(body), result, nil)
	return result, nil
}

type omniUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func parseOmniResponse(body []byte) (string, omniUsage, error) {
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage omniUsage `json:"usage"`
	}
	if json.Unmarshal(body, &parsed) == nil && len(parsed.Choices) > 0 && strings.TrimSpace(parsed.Choices[0].Message.Content) != "" {
		return parsed.Choices[0].Message.Content, parsed.Usage, nil
	}
	var result strings.Builder
	var usage omniUsage
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var event struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *omniUsage `json:"usage"`
		}
		if json.Unmarshal([]byte(payload), &event) != nil {
			continue
		}
		if len(event.Choices) > 0 {
			result.WriteString(event.Choices[0].Delta.Content)
		}
		if event.Usage != nil {
			usage = *event.Usage
		}
	}
	if strings.TrimSpace(result.String()) == "" {
		return "", omniUsage{}, errors.New("参考视频声音分析没有返回内容")
	}
	return result.String(), usage, nil
}

func referenceOmniEndpoint(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || !strings.HasSuffix(parsed.Hostname(), ".aliyuncs.com") || !(strings.HasPrefix(parsed.Hostname(), "dashscope") || strings.HasSuffix(parsed.Hostname(), ".maas.aliyuncs.com")) {
		return "", ErrReferenceAudioUnavailable
	}
	return parsed.Scheme + "://" + parsed.Host + "/compatible-mode/v1/chat/completions", nil
}
