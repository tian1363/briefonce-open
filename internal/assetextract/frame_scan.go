package assetextract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

const maxScanFrames = 240

func scanRate(duration float64, kind string) float64 {
	rate := 2.0
	if kind == "brand_logo" || kind == "video_watermark" {
		rate = 4
	}
	return math.Min(rate, float64(maxScanFrames)/duration)
}

// Decode in one sequential FFmpeg pass, rather than seeking once per sample.
func scanVideoFrames(ctx context.Context, path string, duration float64, kind string) ([]candidateFrame, error) {
	dir, err := os.MkdirTemp("", "asset-frame-scan-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	rate := scanRate(duration, kind)
	filter := fmt.Sprintf("fps=%.8f:start_time=0,scale=512:512:force_original_aspect_ratio=decrease,pad=512:512:(ow-iw)/2:(oh-ih)/2:white", rate)
	cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-i", path, "-vf", filter, "-frames:v", fmt.Sprint(maxScanFrames), "-q:v", "3", filepath.Join(dir, "%05d.jpg"))
	if err := cmd.Run(); err != nil {
		return nil, errors.New("全片关键帧扫描失败")
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.jpg"))
	if err != nil {
		return nil, err
	}
	frames := make([]candidateFrame, 0, len(paths))
	for i, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		frames = append(frames, candidateFrame{AtSeconds: float64(i) / rate, Image: data})
	}
	if len(frames) < 2 {
		return nil, errors.New("视频可读取帧不足")
	}
	return frames, nil
}

// Laplacian energy ranks detail within a time interval; it is not a semantic
// logo detector and is never used to eliminate mark candidates.
func frameSharpness(data []byte) float64 {
	im, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return -1
	}
	b := im.Bounds()
	var sum, sum2, n float64
	gray := func(x, y int) float64 {
		r, g, bl, _ := im.At(x, y).RGBA()
		return (.299*float64(r) + .587*float64(g) + .114*float64(bl)) / 257
	}
	for y := b.Min.Y + 1; y < b.Max.Y-1; y += 2 {
		for x := b.Min.X + 1; x < b.Max.X-1; x += 2 {
			v := 4*gray(x, y) - gray(x-1, y) - gray(x+1, y) - gray(x, y-1) - gray(x, y+1)
			sum += v
			sum2 += v * v
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum2/n - (sum/n)*(sum/n)
}
func selectSharpFrames(frames []candidateFrame, limit int, at float64) []candidateFrame {
	if len(frames) <= limit {
		return frames
	}
	result := make([]candidateFrame, 0, limit)
	// Reserve one slot for the exact nearest anchor to preserve target identity.
	nearest := 0
	for i := range frames {
		if math.Abs(frames[i].AtSeconds-at) < math.Abs(frames[nearest].AtSeconds-at) {
			nearest = i
		}
	}
	result = append(result, frames[nearest])
	seen := map[int]bool{nearest: true}
	for bin := 0; bin < limit-1; bin++ {
		start := bin * len(frames) / (limit - 1)
		end := (bin + 1) * len(frames) / (limit - 1)
		best := -1
		score := -2.0
		for i := start; i < end; i++ {
			if seen[i] {
				continue
			}
			s := frameSharpness(frames[i].Image)
			if s > score {
				best, score = i, s
			}
		}
		if best >= 0 {
			result = append(result, frames[best])
			seen[best] = true
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AtSeconds < result[j].AtSeconds })
	return result
}
func denseLocalTimes(duration, at float64) []float64 {
	out := []float64{}
	for i := -4; i <= 4; i++ {
		t := at + float64(i)*.125
		if t >= 0 && t < duration {
			out = append(out, t)
		}
	}
	return out
}
func fullResolutionFrames(ctx context.Context, path string, frames []candidateFrame) ([]candidateFrame, error) {
	out := make([]candidateFrame, len(frames))
	for i, f := range frames {
		data, err := extractFrame(ctx, path, f.AtSeconds)
		if err != nil {
			return nil, err
		}
		out[i] = candidateFrame{AtSeconds: f.AtSeconds, Image: data}
	}
	return out, nil
}
