package assetextract

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/userctx"
)

func (s *Service) processReference(ctx context.Context, userID, extractionID, kind, path string, duration, at float64, initialFramePath string) error {
	frames, plan, err := s.planFrames(ctx, userID, extractionID, path, kind, duration, at)
	if err != nil {
		return fmt.Errorf("关键帧筛选失败：%w", err)
	}
	main := frames[plan.Main]
	clean := main.Image
	if kind == "brand_logo" || kind == "video_watermark" {
		clean, err = cropTarget(main.Image, plan.TargetBoxes[0])
		if err != nil {
			return fmt.Errorf("目标标识裁切失败：%w", err)
		}
	} else if len(plan.Overlays) > 0 {
		clean = removeScreenOverlays(main.Image, plan.OverlayBoxes)
	}
	analysisFrames := []candidateFrame{main}
	mainIndex := 0
	if kind == "brand_logo" || kind == "video_watermark" {
		analysisFrames = append(analysisFrames, candidateFrame{AtSeconds: main.AtSeconds, Image: clean})
	}
	if kind == "action" || kind == "camera" {
		if main.AtSeconds >= 1 {
			if before, readErr := extractFrame(ctx, path, main.AtSeconds-1); readErr == nil {
				analysisFrames = append([]candidateFrame{{AtSeconds: main.AtSeconds - 1, Image: before}}, analysisFrames...)
				mainIndex++
			}
		}
		if main.AtSeconds+1 < duration {
			if after, readErr := extractFrame(ctx, path, main.AtSeconds+1); readErr == nil {
				analysisFrames = append(analysisFrames, candidateFrame{AtSeconds: main.AtSeconds + 1, Image: after})
			}
		}
	}
	analysis, err := s.analyzeReference(ctx, userID, extractionID, kind, analysisFrames, mainIndex, plan.Overlays)
	if err != nil {
		return fmt.Errorf("参考资产解析失败：%w", err)
	}
	framePath, err := s.Files.SaveCharacterImage(main.Image)
	if err != nil {
		return err
	}
	imagePath, err := s.Files.SaveCharacterImage(clean)
	if err != nil {
		_ = os.Remove(framePath)
		return err
	}
	variant := jobs.AssetExtractionVariant{View: "main", AtSeconds: main.AtSeconds, FramePath: framePath, ImagePath: imagePath}
	if kind == "action" || kind == "camera" {
		boxes := plan.OverlayBoxes
		if len(plan.Overlays) == 0 {
			boxes = nil
		}
		clip, clipErr := extractReferenceClip(ctx, path, main.AtSeconds, duration, boxes)
		if clipErr != nil {
			_ = os.Remove(framePath)
			_ = os.Remove(imagePath)
			return clipErr
		}
		variant.VideoPath, err = s.Files.SaveGeneratedVideo(extractionID+"-reference", bytes.NewReader(clip))
		if err != nil {
			_ = os.Remove(framePath)
			_ = os.Remove(imagePath)
			return err
		}
	}
	if err = s.Store.CompleteAssetExtractionWithVariants(userID, extractionID, framePath, imagePath, "", "", analysis, []jobs.AssetExtractionVariant{variant}); err != nil {
		_ = os.Remove(framePath)
		_ = os.Remove(imagePath)
		if variant.VideoPath != "" {
			_ = os.Remove(variant.VideoPath)
		}
		return err
	}
	if initialFramePath != framePath {
		_ = os.Remove(initialFramePath)
	}
	return nil
}

func (s *Service) analyzeReference(ctx context.Context, userID, extractionID, kind string, frames []candidateFrame, mainIndex int, overlays []string) (string, error) {
	questions := map[string]string{
		"scene":           "描述真实场景的空间、背景、光线、色彩和可见道具；不要虚构地点或拍摄设备。",
		"pose":            "描述人物可见的站姿、手臂、腿部、朝向、构图和身体遮挡；不要推断身份。",
		"action":          "根据连续画面描述可观察的人物或商品动作、起止状态和动作节奏；不要把镜头移动当作人物动作。",
		"camera":          "根据连续画面描述景别、机位、构图以及可观察的推拉、摇移、跟拍或固定镜头；无法从帧序列确认时明确说明。",
		"brand_logo":      "第一张是完整原帧，第二张是从中裁出的实体标识。只记录标识的可见文字、形状、颜色和附着位置；不能由模糊画面推断完整品牌名称或商标归属。",
		"video_watermark": "第一张是完整原帧，第二张是从中裁出的屏幕固定水印。只记录可见文字、图形、颜色和画面位置；不要把水印当成商品品牌事实。",
	}
	prompt := fmt.Sprintf("资产类型：%s。画面按时间顺序排列，时间点为 %s 秒。主帧索引 %d（从 0 开始）。%s 视频叠加层：%s。请用中文 Markdown 分为‘可见内容’、‘可复用提示’、‘不确定处’，仅描述可见证据，避免品牌和商品事实推断。", kind, frameTimes(frames), mainIndex, questions[kind], notesOrNone(overlays))
	items := []model.ContentItem{{Text: prompt}}
	for _, frame := range frames {
		items = append(items, model.ContentItem{Image: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(frame.Image)})
	}
	result, err := s.generateVision(userctx.WithUser(ctx, userctx.User{ID: userID}), model.CallContext{Scope: "asset_extraction", RefID: extractionID, TraceName: "reference-asset-analysis", Step: "analyze_" + kind, SystemPrompt: assetSystemPrompt(kind) + "\n当前阶段输出中文视觉解析，遵守请求中的章节格式。"}, items)
	if err != nil {
		return "", err
	}
	analysis := strings.TrimSpace(result.Text)
	if analysis == "" {
		return "", errors.New("模型未返回参考资产解析")
	}
	if utf8.RuneCountInString(analysis) > 5000 {
		analysis = string([]rune(analysis)[:5000])
	}
	return analysis, nil
}

func extractReferenceClip(ctx context.Context, path string, at, duration float64, boxes []overlayBox) ([]byte, error) {
	start := at - 2
	if start < 0 {
		start = 0
	}
	length := 4.0
	if start+length > duration {
		length = duration - start
	}
	if length < .25 {
		return nil, errors.New("视频片段太短，无法提取动作或镜头")
	}
	tmp, err := os.CreateTemp("", "asset-reference-*.mp4")
	if err != nil {
		return nil, err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)
	filters := []string{"scale=1024:1024:force_original_aspect_ratio=decrease", "pad=1024:1024:(ow-iw)/2:(oh-ih)/2:white"}
	for _, box := range boxes {
		if box.X < 0 || box.Y < 0 || box.W < 4 || box.H < 4 || box.X+box.W > 1000 || box.Y+box.H > 1000 || box.W > 550 || box.H > 220 || box.W*box.H > 100000 {
			continue
		}
		x, y := box.X*1024/1000, box.Y*1024/1000
		w, h := box.W*1024/1000, box.H*1024/1000
		if x < 1 || y < 1 || x+w >= 1023 || y+h >= 1023 {
			continue
		}
		filters = append(filters, fmt.Sprintf("delogo=x=%d:y=%d:w=%d:h=%d", x, y, w, h))
	}
	clipCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()
	cmd := exec.CommandContext(clipCtx, "ffmpeg", "-y", "-v", "error", "-ss", strconv.FormatFloat(start, 'f', 3, 64), "-i", path, "-t", strconv.FormatFloat(length, 'f', 3, 64), "-an", "-vf", strings.Join(filters, ","), "-c:v", "libx264", "-preset", "veryfast", "-crf", "25", "-pix_fmt", "yuv420p", "-movflags", "+faststart", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("无法提取动态参考片段：%s", strings.TrimSpace(string(out)))
	}
	data, err := os.ReadFile(name)
	if err != nil || len(data) == 0 || len(data) > 20<<20 {
		return nil, errors.New("动态参考片段生成失败或超过 20MB")
	}
	return data, nil
}
