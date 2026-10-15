package jobs

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/tian1363/scriptagent/internal/charactergen"
	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/videogen"
)

// PlatformModelInput is written only by the bound administrator. An empty key
// keeps the existing capability key, or uses the shared DashScope key.
type PlatformModelInput struct {
	AccountRef   string `json:"account_ref"`
	Capability   string `json:"capability"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Endpoint     string `json:"endpoint"`
	APIKey       string `json:"api_key"`
	UseSharedKey bool   `json:"use_shared_key"`
}

type PublicPlatformModelProfile struct {
	AccountRef string `json:"account_ref"`
	Capability string `json:"capability"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Endpoint   string `json:"endpoint"`
	Configured bool   `json:"configured"`
	KeySource  string `json:"key_source"`
	Saved      bool   `json:"saved"`
}

var platformModelDefaults = []PlatformModelInput{
	{Capability: "text", Provider: "dashscope", Model: "qwen3.8-flash", Endpoint: model.DefaultDashScopeEndpoint},
	{Capability: "multimodal", Provider: "dashscope", Model: "qwen3.8-omni-flash", Endpoint: model.DefaultDashScopeEndpoint},
	// Image APIs need a workspace-specific endpoint. Leave it unset until the
	// administrator supplies the workspace and matching key.
	{Capability: "image_generation", Provider: "zexapi", Model: "gpt-image-2"},
	{Capability: "image_edit", Provider: "zexapi", Model: "gpt-image-2"},
	{Capability: "video_generation", Provider: "zexapi", Model: "seedance-2.0", Endpoint: videogen.Endpoint},
	{Capability: "embedding", Provider: "dashscope", Model: "text-embedding-v4", Endpoint: model.DefaultDashScopeEmbeddingEndpoint},
	{Capability: "speech_recognition", Provider: "openai", Model: "whisper-1", Endpoint: "https://api.openai.com/v1/audio/transcriptions"},
}

func platformDefault(capability string) (PlatformModelInput, bool) {
	for _, item := range platformModelDefaults {
		if item.Capability == capability {
			return item, true
		}
	}
	return PlatformModelInput{}, false
}

func validatePlatformModel(input PlatformModelInput) error {
	if _, ok := platformDefault(input.Capability); !ok {
		return errors.New("不支持的平台模型能力")
	}
	if len(input.AccountRef) > 120 {
		return errors.New("供应商账号标识最多 120 字节")
	}
	if input.Provider == "zexapi" {
		if input.Capability != "image_generation" && input.Capability != "image_edit" && input.Capability != "video_generation" {
			return errors.New("章鱼哥当前仅接入图片能力")
		}
		if _, ok := charactergen.FindModel(input.Model); !ok && input.Capability != "video_generation" {
			return errors.New("请选择已接入的 GPT 或 Nano Banana 模型")
		}
		if input.Capability == "video_generation" {
			if _, ok := videogen.FindModel(input.Model); !ok {
				return errors.New("请选择已接入的视频型号")
			}
		}
		if input.Endpoint != "https://zexapi.com/v1/videos" {
			return errors.New("图片接口需为 https://zexapi.com/v1/videos")
		}
		return nil
	}
	if input.Provider != "dashscope" && !(input.Capability == "speech_recognition" && input.Provider == "openai") {
		return errors.New("该能力不支持所选服务商")
	}
	if strings.TrimSpace(input.Model) == "" || len(input.Model) > 120 {
		return errors.New("请填写模型名称")
	}
	u, err := url.Parse(strings.TrimSpace(input.Endpoint))
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errors.New("模型接口必须是有效的 HTTPS 地址")
	}
	if input.Capability == "video_generation" {
		return errors.New("视频生成仅支持章鱼哥供应商")
	}
	if (input.Capability == "image_generation" || input.Capability == "image_edit") &&
		(!strings.HasSuffix(u.Hostname(), ".maas.aliyuncs.com") || u.Path != "/api/v1/services/aigc/multimodal-generation/generation" || u.RawQuery != "") {
		return errors.New("图片模型需要百炼工作空间的图片接口地址")
	}
	if input.Capability == "embedding" && (!strings.HasPrefix(input.Model, "text-embedding-") || u.Path != "/api/v1/services/embeddings/text-embedding/text-embedding") {
		return errors.New("当前素材检索需要百炼文字向量模型及对应接口")
	}
	return nil
}

func (s *Store) SavePlatformModelProfile(input PlatformModelInput) error {
	input.Capability = strings.TrimSpace(input.Capability)
	input.Provider = strings.TrimSpace(input.Provider)
	input.Model = strings.TrimSpace(input.Model)
	input.Endpoint = strings.TrimSpace(input.Endpoint)
	if err := validatePlatformModel(input); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var existing, oldProvider string
	err := s.db.QueryRow(`SELECT provider,encrypted_key FROM platform_model_profiles WHERE capability=?`, input.Capability).Scan(&oldProvider, &existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	key := existing
	if oldProvider != "" && oldProvider != input.Provider {
		key = ""
	}
	if input.UseSharedKey {
		if input.Provider != "dashscope" || strings.TrimSpace(input.APIKey) != "" {
			return errors.New("只有百炼模型可以切回平台共享 Key")
		}
		key = ""
	}
	if strings.TrimSpace(input.APIKey) != "" {
		key, err = s.encryptSecret(input.APIKey)
		if err != nil {
			return err
		}
	}
	if (input.Provider == "openai" || input.Provider == "zexapi") && key == "" {
		return errors.New("该供应商需要单独填写 API Key，不能复用百炼 Key")
	}
	_, err = s.db.Exec(`INSERT INTO platform_model_profiles(capability,provider,model,endpoint,encrypted_key,updated_at,account_ref)
VALUES(?,?,?,?,?,?,?) ON CONFLICT(capability) DO UPDATE SET provider=excluded.provider,model=excluded.model,
endpoint=excluded.endpoint,encrypted_key=excluded.encrypted_key,updated_at=excluded.updated_at,account_ref=excluded.account_ref`,
		input.Capability, input.Provider, input.Model, input.Endpoint, key, time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(input.AccountRef))
	return err
}

func (s *Store) platformModelRecord(capability string) (PlatformModelInput, string, bool, error) {
	profile, ok := platformDefault(capability)
	if !ok {
		return PlatformModelInput{}, "", false, errors.New("不支持的平台模型能力")
	}
	var encrypted string
	err := s.db.QueryRow(`SELECT provider,model,endpoint,encrypted_key,account_ref FROM platform_model_profiles WHERE capability=?`, capability).
		Scan(&profile.Provider, &profile.Model, &profile.Endpoint, &encrypted, &profile.AccountRef)
	if errors.Is(err, sql.ErrNoRows) {
		return profile, "", false, nil
	}
	if err != nil {
		return PlatformModelInput{}, "", false, err
	}
	key, err := s.decryptSecret(encrypted)
	return profile, key, true, err
}

func (s *Store) GetPlatformModelRuntimeConfig(capability string) (model.RuntimeConfig, error) {
	profile, key, saved, err := s.platformModelRecord(capability)
	if err != nil {
		return model.RuntimeConfig{}, err
	}
	if !saved && (capability == "video_generation" || capability == "speech_recognition") {
		return model.RuntimeConfig{}, fmt.Errorf("管理员尚未配置平台%s模型", capability)
	}
	if key == "" && profile.Provider == "dashscope" {
		key, err = s.ManagedAPIKey()
		if err != nil {
			return model.RuntimeConfig{}, err
		}
	}
	return model.RuntimeConfig{SupplierAccount: profile.AccountRef, Source: "managed", Provider: profile.Provider, Model: profile.Model, Endpoint: profile.Endpoint, APIKey: key}, nil
}

func (s *Store) ListPublicPlatformModelProfiles() ([]PublicPlatformModelProfile, error) {
	result := make([]PublicPlatformModelProfile, 0, len(platformModelDefaults))
	for _, item := range platformModelDefaults {
		profile, key, saved, err := s.platformModelRecord(item.Capability)
		if err != nil {
			return nil, err
		}
		source := ""
		if key != "" {
			source = "capability"
		} else if profile.Provider == "dashscope" {
			key, err = s.ManagedAPIKey()
			if err != nil {
				return nil, err
			}
			if key != "" {
				source = "shared"
			}
		}
		result = append(result, PublicPlatformModelProfile{
			AccountRef: profile.AccountRef, Capability: item.Capability, Provider: profile.Provider, Model: profile.Model,
			Endpoint: profile.Endpoint, Configured: key != "" && profile.Endpoint != "" && (saved || item.Capability != "video_generation" && item.Capability != "speech_recognition"),
			KeySource: source, Saved: saved,
		})
	}
	return result, nil
}
