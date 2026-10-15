package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tian1363/scriptagent/internal/telemetry"
)

// ErrToolCallingUnsupported permits a caller to use a compatibility protocol.
// Other provider errors must not silently downgrade to text parsing.
var ErrToolCallingUnsupported = errors.New("native tool calling is unsupported")

type ToolSpec struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type ToolMessage struct {
	Role       string
	Content    []ContentItem
	Text       string
	ToolCalls  []ToolCall
	ToolCallID string
}

type ToolTurn struct {
	Text      string
	ToolCalls []ToolCall
	Generation
}

// GenerateToolTurn uses the provider's structured function-call protocol.
func (c *DashScopeClient) GenerateToolTurn(ctx context.Context, callCtx CallContext, messages []ToolMessage, tools []ToolSpec) (turn ToolTurn, callErr error) {
	capability := "text"
	for _, message := range messages {
		if capabilityForContent(message.Content) == "multimodal" {
			capability = "multimodal"
			break
		}
	}
	runtime := c.runtimeConfig(ctx, capability)
	if runtime.APIKey == "" {
		return ToolTurn{}, errors.New("当前没有可用的模型额度，请前往设置连接 API Key")
	}
	request, logRequest, err := toolRequest(runtime.Provider, runtime.Model, messages, tools)
	if err != nil {
		return ToolTurn{}, err
	}
	finish, err := c.beginUsage(ctx, runtime, capability, 1, callCtx)
	if err != nil {
		return ToolTurn{}, err
	}
	defer func() { finish(callErr == nil) }()
	ctx, span := telemetry.StartGeneration(ctx, telemetry.GenerationAttributes{
		Name: callCtx.Step, TraceName: callCtx.TraceName, RunID: callCtx.RunID,
		SpaceID: callCtx.SpaceID, RefID: callCtx.RefID, SessionID: callCtx.SessionID,
		Model: runtime.Model, Input: string(logRequest),
	})
	defer func() {
		telemetry.EndGeneration(span, turn.Text, turn.PromptTokens, turn.OutputTokens, turn.TotalTokens, callErr)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, runtime.Endpoint, bytes.NewReader(request))
	if err != nil {
		return ToolTurn{}, err
	}
	req.Header.Set("Authorization", "Bearer "+runtime.APIKey)
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	res, err := c.httpClient.Do(req)
	if err != nil {
		c.record(ctx, callCtx, runtime.Model, logRequest, nil, Generation{Model: runtime.Model, LatencyMS: elapsedMS(started)}, err)
		return ToolTurn{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 20*1024*1024))
	if err != nil {
		c.record(ctx, callCtx, runtime.Model, logRequest, nil, Generation{Model: runtime.Model, LatencyMS: elapsedMS(started)}, err)
		return ToolTurn{}, err
	}
	logBody := sanitizedResponseJSON(body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if providerRejectsTools(res.StatusCode, body) {
			callErr = ErrToolCallingUnsupported
		} else if runtime.Provider == "openai" {
			callErr = fmt.Errorf("OpenAI 兼容接口请求失败 (%d)：%s", res.StatusCode, string(body))
		} else {
			callErr = friendlyDashScopeError(res.StatusCode, body)
		}
		c.record(ctx, callCtx, runtime.Model, logRequest, logBody, Generation{Model: runtime.Model, LatencyMS: elapsedMS(started)}, callErr)
		return ToolTurn{}, callErr
	}
	turn, err = parseToolTurn(runtime.Provider, body)
	if err != nil {
		c.record(ctx, callCtx, runtime.Model, logRequest, logBody, Generation{Model: runtime.Model, LatencyMS: elapsedMS(started)}, err)
		return ToolTurn{}, err
	}
	turn.Generation = Generation{Text: turn.Text, Model: runtime.Model, RequestJSON: string(logRequest), ResponseJSON: string(logBody), PromptTokens: turn.PromptTokens, OutputTokens: turn.OutputTokens, TotalTokens: turn.TotalTokens, LatencyMS: elapsedMS(started)}
	c.record(ctx, callCtx, runtime.Model, logRequest, logBody, turn.Generation, nil)
	return turn, nil
}

func providerRejectsTools(status int, body []byte) bool {
	if status != http.StatusBadRequest && status != http.StatusUnprocessableEntity {
		return false
	}
	value := strings.ToLower(string(body))
	return (strings.Contains(value, "tool") || strings.Contains(value, "function")) &&
		(strings.Contains(value, "unsupported") || strings.Contains(value, "not support") || strings.Contains(value, "not available"))
}

func toolRequest(provider, modelName string, messages []ToolMessage, tools []ToolSpec) ([]byte, []byte, error) {
	encode := func(redact bool) ([]byte, error) {
		wireMessages := make([]map[string]any, 0, len(messages))
		for _, message := range messages {
			wire := map[string]any{"role": message.Role}
			switch message.Role {
			case "user", "system":
				if provider == "openai" {
					content, err := openAIContent(message, redact)
					if err != nil {
						return nil, err
					}
					wire["content"] = content
				} else {
					content := append([]ContentItem{}, message.Content...)
					if message.Text != "" {
						content = append([]ContentItem{{Text: message.Text}}, content...)
					}
					if redact {
						content = redactedContent(content)
					}
					wire["content"] = content
				}
			case "assistant":
				if provider == "openai" {
					wire["content"] = message.Text
				} else if message.Text != "" {
					wire["content"] = []ContentItem{{Text: message.Text}}
				}
				if len(message.ToolCalls) > 0 {
					wire["tool_calls"] = wireToolCalls(message.ToolCalls)
				}
			case "tool":
				wire["content"] = message.Text
				wire["tool_call_id"] = message.ToolCallID
			default:
				return nil, fmt.Errorf("unsupported tool message role %q", message.Role)
			}
			wireMessages = append(wireMessages, wire)
		}
		wireTools := make([]map[string]any, 0, len(tools))
		for _, tool := range tools {
			parameters := tool.Parameters
			if len(parameters) == 0 {
				parameters = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			if !json.Valid(parameters) {
				return nil, fmt.Errorf("invalid schema for tool %s", tool.Name)
			}
			wireTools = append(wireTools, map[string]any{"type": "function", "function": map[string]any{"name": tool.Name, "description": tool.Description, "parameters": parameters}})
		}
		if provider == "openai" {
			request := map[string]any{"model": modelName, "messages": wireMessages}
			if len(wireTools) > 0 {
				request["tools"] = wireTools
				request["parallel_tool_calls"] = false
			}
			return json.Marshal(request)
		}
		request := map[string]any{"model": modelName, "input": map[string]any{"messages": wireMessages}}
		parameters := map[string]any{"result_format": "message"}
		if len(wireTools) > 0 {
			parameters["tools"] = wireTools
			parameters["parallel_tool_calls"] = false
		}
		request["parameters"] = parameters
		return json.Marshal(request)
	}
	raw, err := encode(false)
	if err != nil {
		return nil, nil, err
	}
	logged, err := encode(true)
	return raw, logged, err
}

func openAIContent(message ToolMessage, redact bool) (any, error) {
	parts := make([]map[string]any, 0, len(message.Content)+1)
	if message.Text != "" {
		parts = append(parts, map[string]any{"type": "text", "text": message.Text})
	}
	for _, item := range message.Content {
		if item.Video != "" {
			return nil, errors.New("OpenAI 兼容接口不支持此视频输入")
		}
		if item.Text != "" {
			parts = append(parts, map[string]any{"type": "text", "text": item.Text})
		}
		if item.Image != "" {
			url := item.Image
			if redact {
				url = "[image data omitted]"
			}
			parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]string{"url": url}})
		}
	}
	if len(parts) == 1 && parts[0]["type"] == "text" {
		return parts[0]["text"], nil
	}
	return parts, nil
}

func redactedContent(content []ContentItem) []ContentItem {
	copy := append([]ContentItem{}, content...)
	for i := range copy {
		if copy[i].Image != "" {
			copy[i].Image = "[image data omitted]"
		}
		if copy[i].Video != "" {
			copy[i].Video = "[video data omitted]"
		}
	}
	return copy
}

func wireToolCalls(calls []ToolCall) []map[string]any {
	result := make([]map[string]any, 0, len(calls))
	for _, call := range calls {
		result = append(result, map[string]any{"id": call.ID, "type": "function", "function": map[string]any{"name": call.Name, "arguments": string(call.Arguments)}})
	}
	return result
}

func parseToolTurn(provider string, body []byte) (ToolTurn, error) {
	type wireCall struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	type wireMessage struct {
		Content   json.RawMessage `json:"content"`
		ToolCalls []wireCall      `json:"tool_calls"`
	}
	type wireChoice struct {
		Message wireMessage `json:"message"`
	}
	var parsed struct {
		Choices []wireChoice `json:"choices"`
		Output  struct {
			Choices []wireChoice `json:"choices"`
		} `json:"output"`
		Usage usage `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ToolTurn{}, err
	}
	var message wireMessage
	if provider == "openai" {
		if len(parsed.Choices) == 0 {
			return ToolTurn{}, errors.New("OpenAI compatible tool response has no choices")
		}
		message = parsed.Choices[0].Message
	} else {
		if len(parsed.Output.Choices) == 0 {
			return ToolTurn{}, errors.New("DashScope tool response has no choices")
		}
		message = parsed.Output.Choices[0].Message
	}
	content := message.Content
	text := ""
	if len(content) > 0 && string(content) != "null" {
		if content[0] == '"' {
			if err := json.Unmarshal(content, &text); err != nil {
				return ToolTurn{}, err
			}
		} else {
			var parts []struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(content, &parts); err != nil {
				return ToolTurn{}, err
			}
			for _, part := range parts {
				text += part.Text
			}
		}
	}
	turn := ToolTurn{Text: strings.TrimSpace(text)}
	for _, call := range message.ToolCalls {
		if call.ID == "" || call.Function.Name == "" || call.Type != "function" {
			return ToolTurn{}, errors.New("tool call is missing id, name, or type")
		}
		turn.ToolCalls = append(turn.ToolCalls, ToolCall{ID: call.ID, Name: call.Function.Name, Arguments: json.RawMessage(call.Function.Arguments)})
	}
	if turn.Text == "" && len(turn.ToolCalls) == 0 {
		return ToolTurn{}, errors.New("tool response contains neither text nor calls")
	}
	turn.PromptTokens = parsed.Usage.promptTokens()
	turn.OutputTokens = parsed.Usage.outputTokens()
	turn.TotalTokens = parsed.Usage.totalTokens()
	return turn, nil
}
