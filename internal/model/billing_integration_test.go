package model_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/userctx"
)

func TestManagedEmbeddingUsesPlatformModelAndEndpoint(t *testing.T) {
	store, err := jobs.OpenStore(filepath.Join(t.TempDir(), "embedding.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SetManagedAPIKey("platform-embedding-key"); err == nil {
		t.Fatal("saved a platform key without encryption")
	}
	if err := store.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))); err != nil {
		t.Fatal(err)
	}
	if err := store.SetManagedAPIKey("platform-embedding-key"); err != nil {
		t.Fatal(err)
	}
	user, err := store.CreateManagedUser(jobs.CreateUserInput{Email: "embedding@example.com", PasswordHash: "hash"}, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), 2, 10, "owner")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/services/embeddings/text-embedding/text-embedding" || r.Header.Get("Authorization") != "Bearer platform-embedding-key" {
			t.Errorf("unexpected platform embedding request")
		}
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model != "text-embedding-v4" {
			t.Errorf("model = %q, err = %v", body.Model, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"embeddings":[{"text_index":0,"embedding":[0.1,0.2]}]},"usage":{"total_tokens":1}}`))
	}))
	defer server.Close()
	previousTransport := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = previousTransport }()
	if err := store.SavePlatformModelProfile(jobs.PlatformModelInput{Capability: "embedding", Provider: "dashscope", Endpoint: server.URL + "/api/v1/services/embeddings/text-embedding/text-embedding", Model: "text-embedding-v4"}); err != nil {
		t.Fatal(err)
	}
	client := model.NewDashScopeClient(model.DashScopeConfig{Provider: store, AllowManaged: true})
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: user.ID})
	result, err := client.EmbedDetailed(ctx, model.CallContext{}, []string{"hello"}, "document")
	if err != nil || result.Model != "text-embedding-v4" || len(result.Vectors) != 1 {
		t.Fatalf("embedding result = %+v err = %v", result, err)
	}
}

func TestManagedModelUsesNewlySavedPlatformKeyAndChargesCredits(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "")
	store, err := jobs.OpenStore(filepath.Join(t.TempDir(), "dynamic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))); err != nil {
		t.Fatal(err)
	}
	user, err := store.CreateManagedUser(jobs.CreateUserInput{Email: "fresh@example.com", PasswordHash: "hash"}, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), 2, 10, "owner")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer newly-saved-secret" {
			t.Errorf("platform key not used")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"text":"reply"}]}}]}}`))
	}))
	defer server.Close()
	previousTransport := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = previousTransport }()
	if err := store.SavePlatformModelProfile(jobs.PlatformModelInput{Capability: "text", Provider: "dashscope", Endpoint: server.URL, Model: "test-model"}); err != nil {
		t.Fatal(err)
	}
	client := model.NewDashScopeClient(model.DashScopeConfig{Provider: store, AllowManaged: true})
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: user.ID})
	if _, err := client.GenerateDetailed(ctx, model.CallContext{}, []model.ContentItem{{Text: "before key"}}); err == nil {
		t.Fatal("model call succeeded without key")
	}
	if err := store.SetManagedAPIKey("newly-saved-secret"); err != nil {
		t.Fatal(err)
	}
	result, err := client.GenerateDetailed(ctx, model.CallContext{}, []model.ContentItem{{Text: "after key"}})
	if err != nil || result.Text != "reply" {
		t.Fatalf("managed request failed: %v", err)
	}
	credit, err := store.GetAccountCredit(user.ID)
	if err != nil || credit.Balance != 1 || calls.Load() != 1 {
		t.Fatalf("balance=%d calls=%d err=%v", credit.Balance, calls.Load(), err)
	}
}

func TestManagedModelUsesPointsBeforeSendingAndRefundsProviderFailure(t *testing.T) {
	store, err := jobs.OpenStore(filepath.Join(t.TempDir(), "model.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.CreateManagedUser(jobs.CreateUserInput{Email: "managed@example.com", PasswordHash: "hash"}, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), 1, 10, "owner")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var fail atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer platform-secret" {
			t.Errorf("wrong credential sent")
		}
		if fail.Load() {
			http.Error(w, `{"code":"Arrearage"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"text":"ok"}]}}]},"usage":{"total_tokens":2}}`))
	}))
	defer server.Close()
	previousTransport := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = previousTransport }()
	err = store.SavePlatformModelProfile(jobs.PlatformModelInput{Capability: "text", Provider: "dashscope", Endpoint: server.URL, Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	client := model.NewDashScopeClient(model.DashScopeConfig{APIKey: "platform-secret", Provider: store, AllowManaged: true})
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: user.ID})
	if _, err = client.GenerateDetailed(ctx, model.CallContext{}, []model.ContentItem{{Text: "first"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = client.GenerateDetailed(ctx, model.CallContext{}, []model.ContentItem{{Text: "second"}}); err == nil {
		t.Fatal("zero points allowed a provider call")
	}
	if calls.Load() != 1 {
		t.Fatalf("provider calls = %d", calls.Load())
	}
	if _, err = store.AdjustCredits(user.ID, "owner", 1, "retry"); err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	if _, err = client.GenerateDetailed(ctx, model.CallContext{}, []model.ContentItem{{Text: "failed"}}); err == nil {
		t.Fatal("provider failure hidden")
	}
	credit, _ := store.GetAccountCredit(user.ID)
	if credit.Balance != 1 || calls.Load() != 2 {
		t.Fatalf("balance=%d calls=%d", credit.Balance, calls.Load())
	}
}

func TestUserWithoutModelSettingsCannotUseServerKey(t *testing.T) {
	store, err := jobs.OpenStore(filepath.Join(t.TempDir(), "fallback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.CreateUser(jobs.CreateUserInput{Email: "byok@example.com", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	client := model.NewDashScopeClient(model.DashScopeConfig{APIKey: "platform-secret", Endpoint: server.URL, Provider: store, AllowManaged: true})
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: user.ID})
	if _, err = client.GenerateDetailed(ctx, model.CallContext{}, []model.ContentItem{{Text: "hello"}}); err == nil {
		t.Fatal("server key inherited")
	}
	if calls.Load() != 0 {
		t.Fatalf("provider was called %d times", calls.Load())
	}
}
