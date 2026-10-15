package web

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tian1363/scriptagent/internal/jobs"
)

func TestRenderCaptionVideoPreservesSourceAndAddsOverlay(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	dir := t.TempDir()
	source, overlay, out := filepath.Join(dir, "source.mp4"), filepath.Join(dir, "overlay.png"), filepath.Join(dir, "out.mp4")
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=blue:s=360x640:d=1:r=10", "-c:v", "libx264", "-pix_fmt", "yuv420p", source)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("source: %v %s", err, output)
	}
	img := image.NewRGBA(image.Rect(0, 0, 720, 1280))
	for y := 1000; y < 1280; y++ {
		for x := 0; x < 720; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	f, err := os.Create(overlay)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := renderCaptionVideo(ctx, source, overlay, out); err != nil {
		t.Fatal(err)
	}
	seconds, err := probeRenderDuration(ctx, out)
	if err != nil || seconds < 0.8 || seconds > 1.2 {
		t.Fatalf("output duration %v: %v", seconds, err)
	}
	if stat, err := os.Stat(out); err != nil || stat.Size() == 0 {
		t.Fatalf("missing output: %v", err)
	}
	frame := filepath.Join(dir, "frame.png")
	cmd = exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-i", out, "-frames:v", "1", frame)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("frame: %v %s", err, output)
	}
	f, err = os.Open(frame)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	red, green, blue, _ := decoded.At(360, 1150).RGBA()
	if red <= green*2 || red <= blue*2 {
		t.Fatalf("overlay missing at bottom: R=%d G=%d B=%d", red, green, blue)
	}
	red, green, blue, _ = decoded.At(360, 250).RGBA()
	if blue <= red*2 || blue <= green*2 {
		t.Fatalf("source missing at top: R=%d G=%d B=%d", red, green, blue)
	}
}

func TestTimedCaptionAppearsOnlyWithinSegment(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	dir := t.TempDir()
	source, overlay, out := filepath.Join(dir, "source.mp4"), filepath.Join(dir, "overlay.png"), filepath.Join(dir, "out.mp4")
	if output, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=blue:s=360x640:d=2:r=10", "-c:v", "libx264", "-pix_fmt", "yuv420p", source).CombinedOutput(); err != nil {
		t.Fatalf("source: %v %s", err, output)
	}
	img := image.NewRGBA(image.Rect(0, 0, 720, 1280))
	for y := 1100; y < 1280; y++ {
		for x := 0; x < 720; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	f, err := os.Create(overlay)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := renderTimedCaptionVideo(ctx, source, []jobs.CaptionSegment{{Start: .5, End: 1.2, Text: "字幕", OverlayPath: overlay}}, false, out); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		at      string
		caption bool
	}{{"0.1", false}, {"0.8", true}, {"1.6", false}} {
		frame := filepath.Join(dir, "frame-"+tc.at+".png")
		if output, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-ss", tc.at, "-i", out, "-frames:v", "1", frame).CombinedOutput(); err != nil {
			t.Fatalf("frame %s: %v %s", tc.at, err, output)
		}
		f, err := os.Open(frame)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		red, _, blue, _ := decoded.At(360, 1150).RGBA()
		if tc.caption && red <= blue*2 || !tc.caption && blue <= red*2 {
			t.Fatalf("caption at %s: red=%d blue=%d", tc.at, red, blue)
		}
	}
}
