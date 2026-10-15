package charactergen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tian1363/scriptagent/internal/imagepolicy"
	"github.com/tian1363/scriptagent/internal/model"
)

type Controls struct {
	Description string `json:"description"`
	Age         int    `json:"age"`
	Hair        string `json:"hair"`
	Outfit      string `json:"outfit"`
	Expression  string `json:"expression"`
	Pose        string `json:"pose"`
	Framing     string `json:"framing"`
	Background  string `json:"background"`
	Lighting    string `json:"lighting"`
	Size        string `json:"size"`
	Seed        int64  `json:"seed"`
	Negative    string `json:"negative"`
}

func (c Controls) Validate() error {
	if c.Age < 18 || c.Age > 90 {
		return errors.New("角色年龄请设置为 18–90 岁")
	}
	if c.Seed < 0 || c.Seed > 2147483647 {
		return errors.New("种子需为 0–2147483647 的整数")
	}
	if c.Size != "1024*1024" && c.Size != "720*1280" && c.Size != "960*1280" {
		return errors.New("请选择支持的角色图尺寸")
	}
	for _, v := range []string{c.Description, c.Hair, c.Outfit, c.Expression, c.Pose, c.Framing, c.Background, c.Lighting, c.Negative} {
		if utf8.RuneCountInString(v) > 200 {
			return errors.New("每项角色描述最多 200 字")
		}
	}
	return nil
}
func (c Controls) Prompt(reference bool) string {
	identity := fmt.Sprintf("创建一位虚构成年广告出镜角色。外观描述：%s。年龄：%d 岁。发型：%s。", c.Description, c.Age, c.Hair)
	if reference {
		identity = "以输入图片中同一位成年角色为身份基准，保持五官比例、脸型、肤色、年龄和发型一致。仅根据下列造型要求编辑，不替换成另一个人。"
	}
	return fmt.Sprintf("%s 服装：%s。表情：%s。姿态：%s。构图：%s。背景：%s。光线：%s。单人真实摄影，面部清晰，皮肤纹理自然，无文字与水印。", identity, c.Outfit, c.Expression, c.Pose, c.Framing, c.Background, c.Lighting) + imagepolicy.NaturalSkin
}

type Client struct {
	HTTP    *http.Client
	Gateway func(context.Context, model.RuntimeConfig, string, string, string, int64, [][]byte) ([]byte, error)
}

var ErrRateLimited = errors.New("图片服务请求过快，请稍后再试")
var ErrModelUnavailable = errors.New("图片模型或工作空间不可用")
var ErrAccessDenied = errors.New("图片模型访问被拒绝")

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 180 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !TrustedURL(req.URL.String()) {
			return errors.New("不支持的图片重定向地址")
		}
		return nil
	}}}
}
func TrustedURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.User == nil && (u.Port() == "" || u.Port() == "443") && strings.HasSuffix(strings.ToLower(u.Hostname()), ".aliyuncs.com")
}
func (c *Client) Generate(ctx context.Context, cfg model.RuntimeConfig, controls Controls, reference []byte) ([]byte, error) {
	if err := controls.Validate(); err != nil {
		return nil, err
	}
	return c.generate(ctx, cfg, controls.Prompt(len(reference) > 0), "多人，面部模糊，肢体畸形，过度磨皮，文字，水印。"+controls.Negative, controls.Size, controls.Seed, reference)
}

// EditReference isolates an entity from a real reference frame using image editing.
func (c *Client) EditReference(ctx context.Context, cfg model.RuntimeConfig, prompt, negative string, reference []byte) ([]byte, error) {
	if strings.TrimSpace(prompt) == "" || len(reference) == 0 {
		return nil, errors.New("需要参考画面和提取说明")
	}
	return c.generate(ctx, cfg, prompt, negative, "1024*1024", 0, reference)
}

// EditReferences combines two or three owned reference images in the given order.
func (c *Client) EditReferences(ctx context.Context, cfg model.RuntimeConfig, prompt, negative, size string, references [][]byte) ([]byte, error) {
	if strings.TrimSpace(prompt) == "" || len(references) < 2 || len(references) > 3 {
		return nil, errors.New("需要两到三张参考图和编辑说明")
	}
	if size != "1024*1024" && size != "960*1280" && size != "720*1280" {
		return nil, errors.New("请选择支持的图片尺寸")
	}
	return c.generateRefs(ctx, cfg, prompt, negative, size, 0, references)
}

func (c *Client) generate(ctx context.Context, cfg model.RuntimeConfig, prompt, negative, size string, seed int64, reference []byte) ([]byte, error) {
	var references [][]byte
	if len(reference) > 0 {
		references = [][]byte{reference}
	}
	return c.generateRefs(ctx, cfg, prompt, negative, size, seed, references)
}

func (c *Client) generateRefs(ctx context.Context, cfg model.RuntimeConfig, prompt, negative, size string, seed int64, references [][]byte) ([]byte, error) {
	if cfg.Provider == "zexapi" {
		if c.Gateway == nil {
			return nil, errors.New("图片供应商任务服务未连接")
		}
		return c.Gateway(ctx, cfg, prompt, negative, size, seed, references)
	}
	if cfg.APIKey == "" {
		return nil, errors.New("请先在设置中连接图片生成／图片编辑模型")
	}
	if cfg.Provider != "dashscope" && cfg.Provider != "" {
		return nil, errors.New("角色图当前接入 Qwen 图片接口，请在对应图片能力中选择 Qwen 模型")
	}
	isEdit := len(references) > 0
	supported := strings.HasPrefix(cfg.Model, "qwen-image-2.0") || (isEdit && (strings.HasPrefix(cfg.Model, "qwen-image-edit-plus") || strings.HasPrefix(cfg.Model, "qwen-image-edit-max")))
	if !supported {
		return nil, errors.New("请为角色生成配置 qwen-image-2.0 系列；造型编辑也支持 qwen-image-edit-plus/max")
	}
	endpoint, endpointErr := url.Parse(cfg.Endpoint)
	if endpointErr != nil || endpoint == nil || !TrustedURL(cfg.Endpoint) || !strings.HasSuffix(strings.ToLower(endpoint.Hostname()), ".maas.aliyuncs.com") || endpoint.Path != "/api/v1/services/aigc/multimodal-generation/generation" || endpoint.RawQuery != "" {
		return nil, errors.New("请配置与 API Key 同地域的百炼工作空间图片接口地址")
	}
	content := []map[string]string{}
	if isEdit {
		for _, reference := range references {
			if len(reference) == 0 || len(reference) > 10<<20 {
				return nil, errors.New("每张参考图需在 10MB 内")
			}
			mime := http.DetectContentType(reference)
			if len(references) > 1 && mime != "image/jpeg" && mime != "image/png" {
				return nil, errors.New("参考图需为 JPG 或 PNG")
			}
			content = append(content, map[string]string{"image": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(reference)})
		}
	}
	content = append(content, map[string]string{"text": prompt})
	body, _ := json.Marshal(map[string]any{"model": cfg.Model, "input": map[string]any{"messages": []any{map[string]any{"role": "user", "content": content}}}, "parameters": map[string]any{"n": 1, "size": size, "seed": seed, "prompt_extend": false, "watermark": false, "negative_prompt": negative}})
	var data []byte
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		res, err := c.HTTP.Do(req)
		if err != nil {
			return nil, errors.New("角色图服务连接失败或超时，请稍后重试")
		}
		data, err = io.ReadAll(io.LimitReader(res.Body, 2<<20))
		res.Body.Close()
		if err != nil {
			return nil, err
		}
		if res.StatusCode/100 == 2 {
			break
		}
		var problem struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(data, &problem)
		if res.StatusCode == http.StatusTooManyRequests && strings.HasPrefix(problem.Code, "Throttling.") {
			if attempt == 0 {
				timer := time.NewTimer(15 * time.Second)
				select {
				case <-timer.C:
					continue
				case <-ctx.Done():
					timer.Stop()
					return nil, ctx.Err()
				}
			}
			return nil, ErrRateLimited
		}
		return nil, imageServiceError(res.StatusCode, problem.Code)
	}
	var out struct {
		Output struct {
			Choices []struct {
				Message struct {
					Content []struct {
						Image string `json:"image"`
					} `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"output"`
	}
	if json.Unmarshal(data, &out) != nil {
		return nil, errors.New("图片服务返回格式无效")
	}
	for _, choice := range out.Output.Choices {
		for _, item := range choice.Message.Content {
			if item.Image == "" {
				continue
			}
			if !TrustedURL(item.Image) {
				return nil, errors.New("图片服务返回了不支持的下载地址")
			}
			return c.downloadGeneratedImage(ctx, item.Image)
		}
	}
	return nil, errors.New("图片服务没有返回角色图，请重试")
}

func imageServiceError(status int, code string) error {
	switch code {
	case "InvalidApiKey", "invalid_api_key":
		return errors.New("图片模型 API Key 无效，请检查对应能力设置")
	case "AccessDenied.Unpurchased":
		return fmt.Errorf("%w：百炼服务尚未开通", ErrAccessDenied)
	case "Model.AccessDenied", "Workspace.AccessDenied", "App.AccessDenied", "AccessDenied", "access_denied":
		return fmt.Errorf("%w：请检查图片模型调用授权、API Key 所属业务空间及接口地域", ErrAccessDenied)
	case "Endpoint.AccessDenied":
		return fmt.Errorf("%w：请检查图片模型是否已下线及接口地址", ErrAccessDenied)
	case "Arrearage", "AllocationQuota.FreeTierOnly":
		return errors.New("图片模型额度不足，请检查百炼账户额度")
	case "ModelNotFound", "model_not_found":
		return fmt.Errorf("%w：请检查模型名称与授权", ErrModelUnavailable)
	case "WorkSpaceNotFound":
		return fmt.Errorf("%w：请检查图片接口地址和访问权限", ErrModelUnavailable)
	case "InvalidParameter":
		if status == http.StatusNotFound {
			return fmt.Errorf("%w：当前工作空间找不到此图片模型", ErrModelUnavailable)
		}
	case "CommodityNotPurchased", "PostpaidBillOverdue", "PrepaidBillOverdue":
		return errors.New("图片模型尚未开通或账单不可用，请检查百炼工作空间")
	}
	if status == http.StatusTooManyRequests {
		return ErrRateLimited
	}
	if status == http.StatusNotFound {
		return fmt.Errorf("%w：请检查工作空间、地域和模型名称", ErrModelUnavailable)
	}
	if status == http.StatusForbidden {
		return fmt.Errorf("%w（403）：请检查百炼图片模型开通状态、调用授权，以及 API Key 与接口地域是否匹配", ErrAccessDenied)
	}
	return fmt.Errorf("图片服务未完成生成（%d），请检查图片模型配置", status)
}

func (c *Client) downloadGeneratedImage(ctx context.Context, url string) ([]byte, error) {
	for attempt := 0; attempt < 3; attempt++ {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		response, err := c.HTTP.Do(request)
		if err == nil {
			if response.StatusCode == http.StatusOK {
				image, readErr := io.ReadAll(io.LimitReader(response.Body, (10<<20)+1))
				response.Body.Close()
				if readErr == nil && len(image) <= 10<<20 && len(image) > 0 {
					return image, nil
				}
			} else {
				response.Body.Close()
				if response.StatusCode < 500 && response.StatusCode != http.StatusTooManyRequests {
					return nil, errors.New("生成完成，但图片下载失败")
				}
			}
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * time.Second)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			}
		}
	}
	return nil, errors.New("生成完成，但图片下载失败")
}

// EditDetailReferences renders a complete content screen from owned product/style inputs.
func (c *Client) EditDetailReferences(ctx context.Context, cfg model.RuntimeConfig, prompt, size string, refs [][]byte) ([]byte, error) {
	if strings.TrimSpace(prompt) == "" || len(refs) < 1 || len(refs) > 2 {
		return nil, errors.New("需要商品素材和本屏说明")
	}
	switch size {
	case "1024*1024", "960*1280", "960*1536", "1440*720", "1536*512":
	default:
		return nil, errors.New("图片尺寸无效")
	}
	return c.generateRefs(ctx, cfg, prompt, "错误商品、改变商品颜色或标识、虚构参数、水印、乱码", size, 0, refs)
}
