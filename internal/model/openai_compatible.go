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
	"strings"
	"time"
)

// generateOpenAICompatible supports the OpenAI Chat Completions protocol,
// which is also offered by most gateway and self-hosted model vendors.
func (c *DashScopeClient) generateOpenAICompatible(ctx context.Context, callCtx CallContext, runtime RuntimeConfig, content []ContentItem) (Generation, error) {
	parts := make([]any, 0, len(content))
	media := false
	for _, item := range content {
		if item.Video != "" {
			if !isKimiVideoRuntime(runtime) {
				return Generation{}, errors.New("当前模型接口不支持视频输入；请配置千问视频模型或 Kimi K3 官方接口")
			}
			parts = append(parts, map[string]any{"type": "video_url", "video_url": map[string]string{"url": item.Video}})
			media = true
		}
		if item.Image != "" {
			parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]string{"url": item.Image}})
			media = true
		}
		if item.Text != "" {
			parts = append(parts, map[string]string{"type": "text", "text": item.Text})
		}
	}
	type message struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	}
	messages := make([]message, 0, 2)
	if systemPrompt := strings.TrimSpace(callCtx.SystemPrompt); systemPrompt != "" {
		messages = append(messages, message{Role: "system", Content: systemPrompt})
	}
	var userContent any = parts
	if !media {
		texts := make([]string, 0, len(content))
		for _, item := range content {
			if item.Text != "" {
				texts = append(texts, item.Text)
			}
		}
		userContent = strings.Join(texts, "\n")
	}
	messages = append(messages, message{Role: "user", Content: userContent})
	reqBody := struct {
		Model    string    `json:"model"`
		Messages []message `json:"messages"`
	}{Model: runtime.Model, Messages: messages}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return Generation{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, runtime.Endpoint, bytes.NewReader(raw))
	if err != nil {
		return Generation{}, err
	}
	req.Header.Set("Authorization", "Bearer "+runtime.APIKey)
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	res, err := c.httpClient.Do(req)
	if err != nil {
		return Generation{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 20*1024*1024))
	if err != nil {
		return Generation{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Generation{}, fmt.Errorf("OpenAI 兼容接口请求失败 (%d)：%s", res.StatusCode, string(body))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Generation{}, err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return Generation{}, errors.New("OpenAI 兼容接口没有返回文本")
	}
	logRaw := raw
	if media {
		logRaw, _ = json.Marshal(map[string]any{"model": runtime.Model, "content": telemetryContent(content)})
	}
	result := Generation{Text: parsed.Choices[0].Message.Content, Model: runtime.Model, RequestJSON: string(logRaw), ResponseJSON: string(body), PromptTokens: parsed.Usage.PromptTokens, OutputTokens: parsed.Usage.CompletionTokens, TotalTokens: parsed.Usage.TotalTokens, LatencyMS: elapsedMS(started)}
	c.record(ctx, callCtx, runtime.Model, logRaw, body, result, nil)
	return result, nil
}

func isKimiVideoRuntime(runtime RuntimeConfig) bool {
	if runtime.Provider != "kimi" && runtime.Provider != "openai" {
		return false
	}
	if !strings.HasPrefix(strings.ToLower(runtime.Model), "kimi-") {
		return false
	}
	parsed, err := url.Parse(runtime.Endpoint)
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	switch parsed.Hostname() {
	case "api.moonshot.cn", "api.moonshot.ai", "api.kimi.com", "api.kimi.ai":
		return strings.HasSuffix(parsed.Path, "/chat/completions")
	}
	return false
}
