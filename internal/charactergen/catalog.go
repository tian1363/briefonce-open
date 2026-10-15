package charactergen

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"
	"unicode/utf8"
)

type ImageModel struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Family        string   `json:"family"`
	Tier          string   `json:"tier"`
	Tiers         []string `json:"tiers"`
	Ratios        []string `json:"ratios"`
	MaxImages     int      `json:"max_images"`
	MaxImageBytes int      `json:"max_image_bytes"`
	MaxPrompt     int      `json:"max_prompt"`
	PixelSize     bool     `json:"pixel_size"`
}

var Ratios = []string{"auto", "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "5:4", "4:5", "21:9"}
var ImageModels = []ImageModel{
	{ID: "gpt-image-2", Name: "GPT Image 2.0 标准", Family: "gpt", Tier: "1K", PixelSize: true},
	{ID: "gpt-image-2-2K", Name: "GPT Image 2.0 2K", Family: "gpt", Tier: "2K", PixelSize: true},
	{ID: "gpt-image-2-4K", Name: "GPT Image 2.0 4K", Family: "gpt", Tier: "4K", PixelSize: true},
	{ID: "nano_banana_2", Name: "Nano Banana 2 标准", Family: "nano", Tier: "default"},
	{ID: "nano_banana_2-2K", Name: "Nano Banana 2 2K", Family: "nano", Tier: "2K"},
	{ID: "nano_banana_2-4K", Name: "Nano Banana 2 4K", Family: "nano", Tier: "4K"},
	{ID: "nano_banana_pro-1K", Name: "Nano Banana Pro 1K", Family: "nano", Tier: "1K"},
	{ID: "nano_banana_pro-2K", Name: "Nano Banana Pro 2K", Family: "nano", Tier: "2K"},
	{ID: "nano_banana_pro-4K", Name: "Nano Banana Pro 4K", Family: "nano", Tier: "4K"},
	{ID: "nano-banana-2.1", Name: "Nano Banana 2.1", Family: "nano", Tier: "variable", Tiers: []string{"1K", "2K", "4K"}},
}
var PixelSizes = map[string][]string{
	"1:1": {"1024x1024", "2048x2048", "2880x2880"}, "16:9": {"1280x720", "2560x1440", "3840x2160"}, "9:16": {"720x1280", "1440x2560", "2160x3840"}, "3:2": {"1248x832", "2496x1664", "3504x2336"}, "2:3": {"832x1248", "1664x2496", "2336x3504"}, "4:3": {"1152x864", "2304x1728", "3264x2448"}, "3:4": {"864x1152", "1728x2304", "2448x3264"}, "5:4": {"1120x896", "2240x1792", "3200x2560"}, "4:5": {"896x1120", "1792x2240", "2560x3200"}, "21:9": {"1456x624", "3024x1296", "3696x1584"},
}

func FindModel(id string) (ImageModel, bool) {
	for _, m := range ImageModels {
		if m.ID == id {
			m.Ratios = Ratios
			m.MaxImages = 8
			m.MaxImageBytes = 10 << 20
			m.MaxPrompt = 10000
			return m, true
		}
	}
	return ImageModel{}, false
}

type Options struct {
	Model        string `json:"model,omitempty"`
	PriceVersion int64  `json:"price_version,omitempty"`
	AspectRatio  string `json:"aspect_ratio,omitempty"`
	Size         string `json:"size,omitempty"`
	ImageSize    string `json:"image_size,omitempty"`
}

func (o Options) Tier() string {
	m, _ := FindModel(o.Model)
	if m.Tier == "variable" {
		if o.ImageSize == "" {
			return "1K"
		}
		return o.ImageSize
	}
	return m.Tier
}
func (o Options) Validate() error {
	m, ok := FindModel(o.Model)
	if !ok {
		return errors.New("请选择支持的图片模型")
	}
	if o.AspectRatio != "" {
		ok = false
		for _, r := range Ratios {
			if r == o.AspectRatio {
				ok = true
			}
		}
		if !ok {
			return errors.New("不支持此图片比例")
		}
	}
	if o.Size != "" {
		if !m.PixelSize || o.AspectRatio != "" || o.ImageSize != "" {
			return errors.New("此模型不支持像素尺寸或尺寸参数冲突")
		}
		idx := map[string]int{"1K": 0, "2K": 1, "4K": 2}[m.Tier]
		ok = false
		for _, v := range PixelSizes {
			if v[idx] == o.Size {
				ok = true
			}
		}
		if !ok {
			return errors.New("像素尺寸与模型档位不匹配")
		}
	}
	if o.ImageSize != "" {
		if m.Tier != "variable" {
			return errors.New("此模型的档位由型号决定，请勿传分辨率覆盖")
		}
		if o.ImageSize != "1K" && o.ImageSize != "2K" && o.ImageSize != "4K" {
			return errors.New("分辨率必须为 1K、2K 或 4K")
		}
	}
	return nil
}
func ValidateInput(o Options, prompt string, refs [][]byte) error {
	if err := o.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(prompt) == "" || utf8.RuneCountInString(prompt) > 10000 {
		return errors.New("最终提示词需为 1–10000 字符（GPT 为平台暂定限制）")
	}
	if len(refs) > 8 {
		return errors.New("参考图最多 8 张")
	}
	totalBytes := 0
	for _, b := range refs {
		totalBytes += len(b)
	}
	if totalBytes > 40<<20 {
		return errors.New("参考图总大小需在 40MB 内")
	}
	for i, b := range refs {
		if len(b) == 0 || len(b) > 10<<20 {
			return fmt.Errorf("第 %d 张参考图为空或超过 10MB", i+1)
		}
		mime := http.DetectContentType(b)
		if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" {
			return fmt.Errorf("第 %d 张参考图需为 JPEG、PNG 或 WEBP", i+1)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
		if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 40000000 {
			return errors.New("参考图损坏或像素数量过大")
		}
		if _, _, err = image.Decode(bytes.NewReader(b)); err != nil {
			return errors.New("参考图数据不完整")
		}
	}
	return nil
}
func RequestBody(o Options, prompt string, refs [][]byte) (map[string]any, error) {
	if err := ValidateInput(o, prompt, refs); err != nil {
		return nil, err
	}
	b := map[string]any{"model": o.Model, "prompt": prompt}
	if o.AspectRatio != "" {
		b["aspect_ratio"] = o.AspectRatio
	}
	if o.Size != "" {
		b["size"] = o.Size
	}
	if o.ImageSize != "" {
		b["image_size"] = o.ImageSize
	}
	if len(refs) > 0 {
		images := []string{}
		for _, ref := range refs {
			images = append(images, "data:"+http.DetectContentType(ref)+";base64,"+base64.StdEncoding.EncodeToString(ref))
		}
		b["images"] = images
	}
	return b, nil
}
