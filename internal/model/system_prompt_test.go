package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type scenarioConfig struct{ config RuntimeConfig }

func (p scenarioConfig) GetModelRuntimeConfig(context.Context, string) (RuntimeConfig, error) {
	return p.config, nil
}

func TestScenarioSystemPromptUsesDedicatedRole(t *testing.T) {
	for _, provider := range []string{"dashscope", "openai"} {
		t.Run(provider, func(t *testing.T) {
			var roles []string
			var systemText string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var raw map[string]any
				if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
					t.Error(err)
					return
				}
				messages := raw["messages"]
				if provider == "dashscope" {
					messages = raw["input"].(map[string]any)["messages"]
				}
				for _, item := range messages.([]any) {
					message := item.(map[string]any)
					roles = append(roles, message["role"].(string))
					if message["role"] == "system" {
						systemText, _ = message["content"].(string)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				if provider == "dashscope" {
					_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"text":"ok"}]}}]}}`))
				} else {
					_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
				}
			}))
			defer server.Close()
			client := NewDashScopeClient(DashScopeConfig{Provider: scenarioConfig{RuntimeConfig{APIKey: "test", Provider: provider, Endpoint: server.URL, Model: "test-model", Source: "byok"}}})
			_, err := client.GenerateDetailed(context.Background(), CallContext{SystemPrompt: "场景专用规则"}, []ContentItem{{Text: "用户素材"}})
			if err != nil {
				t.Fatal(err)
			}
			if len(roles) != 2 || roles[0] != "system" || roles[1] != "user" || systemText != "场景专用规则" {
				t.Fatalf("roles=%v system=%q", roles, systemText)
			}
		})
	}
}
