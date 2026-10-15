package mediaanalysis

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Frame struct {
	Second  float64
	DataURL string
}

// SampleFrames returns bounded, ordered JPEG frames for image-only video models.
func SampleFrames(ctx context.Context, path string, fps, maxFrames int) ([]Frame, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("reference video path is empty")
	}
	if fps < 1 {
		fps = 2
	}
	if fps > 4 {
		fps = 4
	}
	if maxFrames < 1 || maxFrames > 72 {
		maxFrames = 72
	}
	effectiveFPS := float64(fps)
	probeCtx, probeCancel := context.WithTimeout(ctx, 15*time.Second)
	probeOutput, probeErr := exec.CommandContext(probeCtx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	probeCancel()
	if probeErr == nil {
		if duration, parseErr := strconv.ParseFloat(strings.TrimSpace(string(probeOutput)), 64); parseErr == nil && duration > 0 && duration*effectiveFPS > float64(maxFrames) {
			effectiveFPS = float64(maxFrames) / duration
		}
	}
	if effectiveFPS < 0.1 {
		effectiveFPS = 0.1
	}
	dir, err := os.MkdirTemp("", "scriptagent-reference-frames-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	commandCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	pattern := filepath.Join(dir, "%04d.jpg")
	filter := fmt.Sprintf("fps=%.5f,scale=720:-2", effectiveFPS)
	output, err := exec.CommandContext(commandCtx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-i", path, "-vf", filter, "-frames:v", fmt.Sprint(maxFrames), "-q:v", "5", pattern).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(string(output)))
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.jpg"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("reference video has no readable frames")
	}
	frames := make([]Frame, 0, len(files))
	for index, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		frames = append(frames, Frame{Second: float64(index) / effectiveFPS, DataURL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)})
	}
	return frames, nil
}
