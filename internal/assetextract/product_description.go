package assetextract

import (
	"github.com/tian1363/scriptagent/internal/jobs"
	"regexp"
	"strings"
	"unicode/utf8"
)

var descriptionMarkdown = regexp.MustCompile("[`*_#>]")
var descriptionLabel = regexp.MustCompile(`^[\s\-\d.、]*`)

// Product descriptions contain visible asset content, never review reasoning.
func ProductAssetDescription(item jobs.AssetExtraction) string {
	if item.Kind == "brand_logo" {
		return "透明背景品牌标识图。"
	}
	if item.Kind == "video_watermark" {
		return "透明背景视频水印图。"
	}
	text := item.Analysis
	if start := strings.Index(text, "### 生成图可见事实"); start >= 0 {
		text = text[start+len("### 生成图可见事实"):]
		if end := strings.Index(text, "\n### "); end >= 0 {
			text = text[:end]
		}
	}
	parts := []string{}
	blocked := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			blocked = false
			for _, word := range []string{"原帧", "证据", "质检", "结论", "不确定", "补造", "判定", "水印/叠字", "可复用提示"} {
				if strings.Contains(line, word) {
					blocked = true
				}
			}
			continue
		}
		if blocked || line == "" {
			continue
		}
		unsafe := false
		for _, word := range []string{"质检", "结论", "审核", "理由", "来源", "依据", "失败", "置信", "应该", "建议", "证明", "因此", "reason", "matches_", "关键帧选择", "视频叠加层", "已确认的", "不确定", "原帧", "原视频", "生成图", "一致", "比对", "核对", "检查", "判定", "证据", "通过", "未通过", "推断", "推测", "确认", "=false", "=true", "true", "false", "→", "移除", "未见", "仅用于", "不能作为", "归属", "保持了", "同一", "相同", "二者", "裁切", "第二张", "第三张", "图1", "图2", "图3", "新增", "臆造", "对应", "色相偏移"} {
			if strings.Contains(line, word) {
				unsafe = true
			}
		}
		if unsafe {
			continue
		}
		line = descriptionLabel.ReplaceAllString(descriptionMarkdown.ReplaceAllString(line, ""), "")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "：") || strings.HasSuffix(line, ":") {
			continue
		}
		parts = append(parts, line)
		if len(parts) == 2 {
			break
		}
	}
	if len(parts) > 0 {
		text = strings.Join(parts, "；")
		if utf8.RuneCountInString(text) > 180 {
			text = string([]rune(text)[:179]) + "…"
		}
		return text
	}
	labels := map[string]string{"person": "人物角色参考图", "product": "商品外观参考图", "scene": "场景参考图", "pose": "人物姿势参考图", "action": "人物动作参考片段", "camera": "镜头运动参考片段", "brand_logo": "透明背景品牌标识图", "video_watermark": "透明背景视频水印图"}
	if label := labels[item.Kind]; label != "" {
		return label + "。"
	}
	return "图片或视频参考素材。"
}
