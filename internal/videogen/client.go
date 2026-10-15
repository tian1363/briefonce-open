package videogen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tian1363/scriptagent/internal/model"
)

type ConfigProvider interface {
	GetModelRuntimeConfig(context.Context, string) (model.RuntimeConfig, error)
}

var ErrProviderRejected = errors.New("视频服务拒绝任务")

type Client struct {
	provider ConfigProvider
	http     *http.Client
}

type Request struct {
	Model, Prompt, NegativePrompt, Resolution, Ratio string
	References                                       []Reference
	Duration                                         int
	SoundEnabled                                     bool
}

type Reference struct {
	URL, Kind, Path, Name string
}

func New(provider ConfigProvider) *Client {
	return &Client{provider: provider, http: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Client) CheckConfig(ctx context.Context) error {
	cfg, err := c.provider.GetModelRuntimeConfig(ctx, "video_generation")
	if err != nil {
		return errors.New("请先在设置中配置视频生成模型")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return errors.New("视频生成 API Key 未配置")
	}
	return validateEndpoint(cfg.Endpoint)
}

func (c *Client) Submit(ctx context.Context, input Request) (string, error) {
	cfg, err := c.provider.GetModelRuntimeConfig(ctx, "video_generation")
	if err != nil {
		return "", err
	}
	if err = validateEndpoint(cfg.Endpoint); err != nil {
		return "", fmt.Errorf("%w: %v", ErrProviderRejected, err)
	}
	if cfg.APIKey == "" {
		return "", fmt.Errorf("%w: 视频 API Key 未配置", ErrProviderRejected)
	}
	if input.Model == "" {
		input.Model = cfg.Model
	}
	mode := "reference"
	for _, r := range input.References {
		if r.Kind == "first_frame" || r.Kind == "last_frame" {
			mode = "first_frame"
		}
	}
	if err = Validate(input.Model, input.Resolution, input.Ratio, input.Duration, mode); err != nil {
		return "", fmt.Errorf("%w: %v", ErrProviderRejected, err)
	}
	if err = validateReferences(input.References); err != nil {
		return "", fmt.Errorf("%w: %v", ErrProviderRejected, err)
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	prompt := input.Prompt
	if input.NegativePrompt != "" {
		prompt += "\n避免：" + input.NegativePrompt
	}
	if !input.SoundEnabled {
		prompt += "\n声音要求：静音，不生成对白、音乐或音效。"
	}
	fields := map[string]string{"model": input.Model, "prompt": prompt, "aspect_ratio": input.Ratio, "seconds": strconv.Itoa(input.Duration)}
	if input.Resolution != "" && input.Resolution != "default" {
		fields["resolution"] = strings.ToLower(input.Resolution)
	}
	if mode == "first_frame" {
		fields["first_last_frame"] = "true"
	}
	for k, v := range fields {
		if err = w.WriteField(k, v); err != nil {
			return "", err
		}
	}
	// Provider requires first frame before last frame, independent of input order.
	refs := append([]Reference{}, input.References...)
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].Kind == "first_frame" && refs[j].Kind != "first_frame" })
	for _, ref := range refs {
		if ref.Path != "" {
			f, e := os.Open(ref.Path)
			if e != nil {
				return "", fmt.Errorf("%w: 读取参考素材失败", ErrProviderRejected)
			}
			name := ref.Name
			if name == "" {
				name = filepath.Base(ref.Path)
			}
			part, e := w.CreateFormFile("input_reference", filepath.Base(name))
			if e == nil {
				_, e = io.Copy(part, f)
			}
			f.Close()
			if e != nil {
				return "", fmt.Errorf("%w: 上传素材准备失败", ErrProviderRejected)
			}
		} else if ref.URL != "" {
			if err = w.WriteField("input_reference", ref.URL); err != nil {
				return "", err
			}
		}
	}
	if err = w.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode/100 != 2 {
		return "", fmt.Errorf("%w: %s", ErrProviderRejected, providerMessage(payload, res.Status))
	}
	var out struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(payload, &out) != nil || out.ID == "" {
		return "", errors.New("视频服务未返回任务编号，请核对供应商记录")
	}
	return out.ID, nil
}
func (c *Client) Poll(ctx context.Context, taskID string) (status, videoURL, message string, err error) {
	cfg, err := c.provider.GetModelRuntimeConfig(ctx, "video_generation")
	if err != nil {
		return "", "", "", err
	}
	if err = validateEndpoint(cfg.Endpoint); err != nil {
		return "", "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(cfg.Endpoint, "/")+"/"+url.PathEscape(taskID), nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	res, err := c.http.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer res.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode/100 != 2 {
		return "", "", "", fmt.Errorf("查询视频进度失败：%s", providerMessage(payload, res.Status))
	}
	var out struct {
		Status   string `json:"status"`
		URL      string `json:"url"`
		VideoURL string `json:"video_url"`
		Error    struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err = json.Unmarshal(payload, &out); err != nil {
		return "", "", "", err
	}
	videoURL = out.VideoURL
	if videoURL == "" {
		videoURL = out.URL
	}
	status = strings.ToUpper(out.Status)
	if status == "COMPLETED" {
		status = "SUCCEEDED"
	}
	return status, videoURL, out.Error.Message, nil
}
func validateEndpoint(raw string) error {
	if raw != Endpoint {
		return errors.New("视频接口需为 https://zexapi.com/v1/videos")
	}
	return nil
}
func validateReferences(refs []Reference) error {
	if len(refs) > 9 {
		return errors.New("参考素材合计最多 9 项")
	}
	counts := map[string]int{}
	for _, ref := range refs {
		switch ref.Kind {
		case "first_frame", "last_frame", "reference_image", "reference_video", "reference_audio":
		default:
			return errors.New("不支持的视频参考素材类型")
		}
		if ref.Path == "" && ref.URL == "" {
			return errors.New("参考素材为空")
		}
		counts[ref.Kind]++
	}
	if counts["first_frame"] > 1 || counts["last_frame"] > 1 {
		return errors.New("首帧和尾帧各限一张")
	}
	if counts["last_frame"] > 0 && counts["first_frame"] == 0 {
		return errors.New("尾帧需要与首帧一起使用")
	}
	if counts["first_frame"]+counts["last_frame"] > 0 && counts["reference_image"]+counts["reference_video"]+counts["reference_audio"] > 0 {
		return errors.New("首尾帧不能与参考内容混用")
	}
	return nil
}

func (c *Client) Download(ctx context.Context, videoURL string) (io.ReadCloser, error) {
	u, err := url.Parse(videoURL)
	if err != nil || u.Scheme != "https" {
		return nil, errors.New("视频服务返回了无效下载地址")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode/100 != 2 {
		res.Body.Close()
		return nil, fmt.Errorf("下载成片失败：%s", res.Status)
	}
	return res.Body, nil
}

func providerMessage(payload []byte, fallback string) string {
	var x struct {
		Message string `json:"message"`
		Code    string `json:"code"`
		Error   struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(payload, &x) == nil {
		message := x.Message
		if message == "" {
			message = x.Error.Message
		}
		if message != "" {
			return message
		}
	}
	return fallback
}

// WithRuntime pins a task to the credentials and endpoint saved at submission.
func (c *Client) WithRuntime(cfg model.RuntimeConfig) *Client {
	return &Client{provider: fixedConfig{cfg}, http: c.http}
}

type fixedConfig struct{ cfg model.RuntimeConfig }

func (f fixedConfig) GetModelRuntimeConfig(context.Context, string) (model.RuntimeConfig, error) {
	return f.cfg, nil
}
