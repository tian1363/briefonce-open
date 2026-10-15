package jobs

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/tian1363/scriptagent/internal/userctx"
)

func TestTrialMemberUsesPlatformProfileEvenWithOldPersonalSettings(t *testing.T) {
	store, err := OpenStore(t.TempDir() + "/platform.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{4}, 32))); err != nil {
		t.Fatal(err)
	}
	member, err := store.CreateManagedUser(CreateUserInput{Email: "trial@example.com", PasswordHash: "hash"}, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), 100, 100, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveUserModelSettings(member.ID, ModelSettings{Capability: "video_generation", Mode: "byok", APIKey: "old-private-key", Endpoint: "https://old.example.com", Model: "old-model"}); err != nil {
		t.Fatal(err)
	}
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: member.ID})
	if _, err := store.GetModelRuntimeConfig(ctx, "video_generation"); err == nil {
		t.Fatal("trial account inherited an old personal video profile")
	}
	if err := store.SetManagedAPIKey("shared-platform-key"); err != nil {
		t.Fatal(err)
	}
	endpoint := "https://zexapi.com/v1/videos"
	if err := store.SavePlatformModelProfile(PlatformModelInput{Capability: "video_generation", Provider: "zexapi", Model: "seedance-2.0", Endpoint: endpoint, APIKey: "video-platform-key"}); err != nil {
		t.Fatal(err)
	}
	runtime, err := store.GetModelRuntimeConfig(ctx, "video_generation")
	if err != nil || runtime.Source != "managed" || runtime.APIKey != "video-platform-key" || runtime.Endpoint != endpoint || runtime.Model != "seedance-2.0" {
		t.Fatalf("wrong platform runtime: source=%q model=%q endpoint=%q has_key=%t err=%v", runtime.Source, runtime.Model, runtime.Endpoint, runtime.APIKey != "", err)
	}
	if err := store.SavePlatformModelProfile(PlatformModelInput{Capability: "video_generation", Provider: "zexapi", Model: "seedance-2.0", Endpoint: endpoint, APIKey: "dedicated-platform-key"}); err != nil {
		t.Fatal(err)
	}
	runtime, err = store.GetModelRuntimeConfig(ctx, "video_generation")
	if err != nil || runtime.APIKey != "dedicated-platform-key" {
		t.Fatalf("capability key not selected: %v", err)
	}
	var stored string
	if err := store.db.QueryRow(`SELECT encrypted_key FROM platform_model_profiles WHERE capability='video_generation'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "dedicated-platform-key") || !strings.HasPrefix(stored, encryptedSecretPrefix) {
		t.Fatal("platform capability key was not encrypted")
	}
	if err := store.SavePlatformModelProfile(PlatformModelInput{Capability: "video_generation", Provider: "zexapi", Model: "seedance-2.0", Endpoint: endpoint, UseSharedKey: true}); err == nil {
		t.Fatal("video accepted shared DashScope key")
	}
	runtime, err = store.GetModelRuntimeConfig(ctx, "video_generation")
	if err != nil || runtime.APIKey != "dedicated-platform-key" {
		t.Fatal("rejected key change modified credentials")
	}

}

func TestPlatformVideoRequiresWorkspaceEndpoint(t *testing.T) {
	store, err := OpenStore(t.TempDir() + "/platform.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SavePlatformModelProfile(PlatformModelInput{Capability: "video_generation", Provider: "dashscope", Model: "wan3.0-video", Endpoint: "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"}); err == nil {
		t.Fatal("accepted a non-Wan workspace endpoint")
	}
}

func TestPlatformImageRequiresWorkspaceEndpoint(t *testing.T) {
	base := PlatformModelInput{Capability: "image_edit", Provider: "dashscope", Model: "qwen-image-2.0"}
	base.Endpoint = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	if err := validatePlatformModel(base); err == nil {
		t.Fatal("accepted legacy public image endpoint")
	}
	base.Endpoint = "https://workspace.cn-beijing.maas.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	if err := validatePlatformModel(base); err != nil {
		t.Fatalf("rejected workspace image endpoint: %v", err)
	}
	profile, ok := platformDefault("image_edit")
	if !ok || profile.Endpoint != "" || profile.Model != "gpt-image-2" || profile.Provider != "zexapi" {
		t.Fatalf("unsafe image default: %+v", profile)
	}
}
