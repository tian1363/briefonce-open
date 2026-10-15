package web

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"
)

//go:embed caption_ocr.swift
var captionOCRSwift string

type ocrFrame struct {
	Index      int     `json:"index"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

func (h *Handler) scanRenderCaptions(w http.ResponseWriter, r *http.Request) {
	if !h.authLimit.allow("ocr:"+userIDFromRequest(r), 5, time.Minute) {
		writeError(w, 429, errors.New("扫描请求较多，请稍后再试"))
		return
	}
	var input transcriptionRequest
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, errors.New("视频来源无效"))
		return
	}
	userID := userIDFromRequest(r)
	if _, err := h.store.GetUserProduct(userID, input.ProductID); err != nil {
		writeError(w, 400, errors.New("请选择自己的产品资料"))
		return
	}
	path, err := h.renderSourcePath(userID, input.ProductID, input.SourceAssetID, input.SourceVideoID, input.SourceBatchID, input.SourceVariantID)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	seconds, err := probeRenderDuration(r.Context(), path)
	if err != nil || seconds <= 0 || seconds > 30 {
		writeError(w, 400, errors.New("原视频需可读取且不超过 30 秒"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	segments, err := scanVisibleCaptions(ctx, path, seconds)
	if err != nil {
		writeError(w, 502, err)
		return
	}
	writeJSON(w, 200, map[string]any{"segments": segments, "duration": seconds})
}

func scanVisibleCaptions(ctx context.Context, videoPath string, duration float64) ([]recognizedSegment, error) {
	dir, err := os.MkdirTemp("", "scriptagent-ocr-*")
	if err != nil {
		return nil, errors.New("无法准备画面扫描")
	}
	defer os.RemoveAll(dir)
	pattern := filepath.Join(dir, "frame-%03d.png")
	filter := "fps=1,scale=720:-2,crop=iw:ih*0.38:0:ih*0.62"
	if output, err := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-i", videoPath, "-vf", filter, "-frames:v", "30", pattern).CombinedOutput(); err != nil {
		_ = output
		return nil, errors.New("无法提取画面帧")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.New("无法读取画面帧")
	}
	paths := []string{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "frame-") && strings.HasSuffix(entry.Name(), ".png") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, errors.New("视频没有可扫描的画面")
	}
	frames, err := recognizeCaptionFrames(ctx, dir, paths)
	if err != nil {
		return nil, err
	}
	segments := groupOCRFrames(frames, duration)
	if len(segments) == 0 {
		return nil, errors.New("未在画面底部找到可识别的字幕；可手动添加逐句字幕")
	}
	return segments, nil
}

func recognizeCaptionFrames(ctx context.Context, dir string, paths []string) ([]ocrFrame, error) {
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("swift"); err == nil {
			scriptPath := filepath.Join(dir, "caption_ocr.swift")
			if err := os.WriteFile(scriptPath, []byte(captionOCRSwift), 0600); err != nil {
				return nil, errors.New("无法准备画面识别")
			}
			output, err := exec.CommandContext(ctx, "swift", append([]string{scriptPath}, paths...)...).Output()
			if err != nil {
				return nil, errors.New("画面文字识别失败")
			}
			var frames []ocrFrame
			if err := json.Unmarshal(output, &frames); err != nil {
				return nil, errors.New("画面文字结果无法读取")
			}
			return frames, nil
		}
	}
	if _, err := exec.LookPath("tesseract"); err != nil {
		return nil, errors.New("画面扫描需要 macOS Vision 或安装 Tesseract OCR")
	}
	frames := make([]ocrFrame, 0, len(paths))
	for i, path := range paths {
		output, err := exec.CommandContext(ctx, "tesseract", path, "stdout", "-l", "chi_sim+eng", "--psm", "6").Output()
		if err != nil {
			return nil, errors.New("Tesseract 识别失败，请安装中文语言数据 chi_sim")
		}
		frames = append(frames, ocrFrame{Index: i, Text: string(output), Confidence: 0.5})
	}
	return frames, nil
}

func cleanOCRText(value string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(value), " "))
}

func countVisibleLetters(value string) int {
	n := 0
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
		}
	}
	return n
}

func groupOCRFrames(frames []ocrFrame, duration float64) []recognizedSegment {
	byIndex := make(map[int]ocrFrame, len(frames))
	for _, frame := range frames {
		byIndex[frame.Index] = frame
	}
	segments := []recognizedSegment{}
	var current *recognizedSegment
	flush := func() {
		if current != nil {
			segments = append(segments, *current)
			current = nil
		}
	}
	for i := 0; i < int(math.Ceil(duration)) && i < 30; i++ {
		frame := byIndex[i]
		text := cleanOCRText(frame.Text)
		if frame.Confidence < .25 || countVisibleLetters(text) < 2 {
			flush()
			continue
		}
		end := math.Min(duration, float64(i+1))
		if current != nil && current.End == float64(i) && ocrSimilar(current.Text, text) {
			current.End = end
			continue
		}
		flush()
		current = &recognizedSegment{Start: float64(i), End: end, Text: text, NeedsReview: true, Source: "visual"}
	}
	flush()
	return segments
}

func ocrSimilar(a, b string) bool {
	a, b = strings.ReplaceAll(a, " ", ""), strings.ReplaceAll(b, " ", "")
	if a == b {
		return true
	}
	if len([]rune(a)) < 3 || len([]rune(b)) < 3 {
		return false
	}
	shorter, longer := a, b
	if len([]rune(shorter)) > len([]rune(longer)) {
		shorter, longer = longer, shorter
	}
	return strings.Contains(longer, shorter) && float64(len([]rune(shorter)))/float64(len([]rune(longer))) >= .7
}
