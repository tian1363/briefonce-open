package reactagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/model"
)

type nativeTestProvider struct{ endpoint string }

func (p nativeTestProvider) GetModelRuntimeConfig(context.Context, string) (model.RuntimeConfig, error) {
	return model.RuntimeConfig{APIKey: "test", Provider: "openai", Endpoint: p.endpoint, Model: "test-model", Source: "byok"}, nil
}

func TestNativeToolCallingRoundTrip(t *testing.T) {
	for _, provider := range []string{"dashscope", "openai"} {
		t.Run(provider, func(t *testing.T) {
			requests, executions := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if requests == 1 {
					if provider == "openai" && len(body["tools"]) == 0 {
						t.Fatal("OpenAI request lacks native tools")
					}
					if provider == "dashscope" && !strings.Contains(string(body["parameters"]), `"tools"`) {
						t.Fatal("DashScope request lacks native tools in parameters")
					}
					if provider == "openai" {
						fmt.Fprint(w, `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"list_products","arguments":"{}"}}]}}]}`)
					} else {
						fmt.Fprint(w, `{"output":{"choices":[{"message":{"content":[],"tool_calls":[{"id":"call_1","type":"function","function":{"name":"list_products","arguments":"{}"}}]}}]}}`)
					}
					return
				}
				messageJSON := string(body["messages"])
				if provider == "dashscope" {
					messageJSON = string(body["input"])
				}
				if !strings.Contains(messageJSON, `"tool_call_id":"call_1"`) || !strings.Contains(messageJSON, `"role":"assistant"`) {
					t.Fatalf("tool result or assistant call missing: %s", messageJSON)
				}
				if provider == "dashscope" {
					var payload struct { Input struct { Messages []map[string]json.RawMessage `json:"messages"` } `json:"input"` }
					if err := json.Unmarshal(body["input"], &payload.Input); err != nil { t.Fatal(err) }
					for _, message := range payload.Input.Messages {
						if string(message["role"]) == `"assistant"` && string(message["content"]) == `[{}]` {
							t.Fatalf("empty assistant content sent as invalid modal item: %s", messageJSON)
						}
					}
				}
				if provider == "openai" {
					fmt.Fprint(w, `{"choices":[{"message":{"content":"完成"}}]}`)
				} else {
					fmt.Fprint(w, `{"output":{"choices":[{"message":{"content":[{"text":"完成"}]}}]}}`)
				}
			}))
			defer server.Close()
			cfg := model.DashScopeConfig{APIKey: "test", Endpoint: server.URL}
			if provider == "openai" {
				cfg.Provider = nativeTestProvider{endpoint: server.URL}
			}
			result, err := New(model.NewDashScopeClient(cfg), 4).Run(context.Background(), RunInput{Goal: "列出产品", Tools: []Tool{{Name: "list_products", Description: "列出产品", InputSchema: `{"type":"object","properties":{},"additionalProperties":false}`, Handler: func(context.Context, json.RawMessage) (string, error) {
				executions++
				return `[{"name":"梨汁"}]`, nil
			}}}})
			if err != nil || result.Answer != "完成" || executions != 1 || requests != 2 {
				t.Fatalf("native round trip failed: requests=%d executions=%d result=%+v err=%v", requests, executions, result, err)
			}
		})
	}
}

func TestUnsupportedNativeToolCallingFallsBackToTextProtocol(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body["parameters"]) > 0 {
			http.Error(w, `{"message":"tools not supported"}`, http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"output":{"choices":[{"message":{"content":[{"text":"{\"type\":\"final\",\"answer\":\"兼容回答\"}"}]}}]}}`)
	}))
	defer server.Close()
	client := model.NewDashScopeClient(model.DashScopeConfig{APIKey: "test", Endpoint: server.URL})
	result, err := New(client, 4).Run(context.Background(), RunInput{Goal: "回答", Tools: []Tool{{Name: "list_products", Description: "列出产品", InputSchema: `{"type":"object","properties":{}}`, Handler: func(context.Context, json.RawMessage) (string, error) { return "products", nil }}}})
	if err != nil || result.Answer != "兼容回答" || requests != 2 {
		t.Fatalf("fallback failed: requests=%d result=%+v err=%v", requests, result, err)
	}
}

func TestNativeToolFailureAfterExecutionDoesNotReplayLegacy(t *testing.T) {
	requests, executions := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			fmt.Fprint(w, `{"output":{"choices":[{"message":{"content":[],"tool_calls":[{"id":"call_1","type":"function","function":{"name":"list_products","arguments":"{}"}}]}}]}}`)
			return
		}
		http.Error(w, `{"message":"tools not supported"}`, http.StatusBadRequest)
	}))
	defer server.Close()
	client := model.NewDashScopeClient(model.DashScopeConfig{APIKey: "test", Endpoint: server.URL})
	_, err := New(client, 4).Run(context.Background(), RunInput{Goal: "list", Tools: []Tool{{Name: "list_products", InputSchema: `{"type":"object","properties":{}}`, Handler: func(context.Context, json.RawMessage) (string, error) { executions++; return "products", nil }}}})
	if err == nil || requests != 2 || executions != 1 {
		t.Fatalf("native failure replayed tool work: requests=%d executions=%d err=%v", requests, executions, err)
	}
}

func TestNativeRespondsToEveryReturnedCallID(t *testing.T) {
	requests, executions := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			fmt.Fprint(w, `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"first","arguments":"{}"}},{"id":"call_2","type":"function","function":{"name":"second","arguments":"{}"}}]}}]}`)
			return
		}
		var body struct {
			Messages []struct {
				Role       string `json:"role"`
				ToolCallID string `json:"tool_call_id"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		ids := map[string]bool{}
		for _, message := range body.Messages {
			if message.Role == "tool" {
				ids[message.ToolCallID] = true
			}
		}
		if !ids["call_1"] || !ids["call_2"] {
			t.Fatalf("missing tool responses: %+v", ids)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"完成"}}]}`)
	}))
	defer server.Close()
	client := model.NewDashScopeClient(model.DashScopeConfig{Provider: nativeTestProvider{endpoint: server.URL}})
	tool := func(name string) Tool {
		return Tool{Name: name, InputSchema: `{"type":"object","properties":{}}`, Handler: func(context.Context, json.RawMessage) (string, error) { executions++; return name, nil }}
	}
	result, err := New(client, 4).Run(context.Background(), RunInput{Goal: "use both", Tools: []Tool{tool("first"), tool("second")}})
	if err != nil || result.Answer != "完成" || executions != 2 || requests != 2 {
		t.Fatalf("multi-call round trip failed: result=%+v executions=%d requests=%d err=%v", result, executions, requests, err)
	}
}
