package assetextract

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/storage"
)

func TestLogoAndWatermarkRequireTheirOwnEvidence(t *testing.T) {
	box := overlayBox{X: 100, Y: 100, W: 200, H: 120}
	if err := validateTargetPlan("brand_logo", framePlan{TargetBoxes: []overlayBox{box}, Overlays: []string{"platform mark"}}); err == nil || !strings.Contains(err.Error(), "实体") {
		t.Fatalf("overlay accepted as logo: %v", err)
	}
	if err := validateTargetPlan("video_watermark", framePlan{TargetBoxes: []overlayBox{box}, GarmentMarks: []string{"printed logo"}}); err == nil || !strings.Contains(err.Error(), "水印") {
		t.Fatalf("physical mark accepted as watermark: %v", err)
	}
	if err := validateTargetPlan("brand_logo", framePlan{TargetBoxes: []overlayBox{box}, GarmentMarks: []string{"printed logo"}}); err != nil {
		t.Fatal(err)
	}
	if err := validateTargetPlan("video_watermark", framePlan{TargetBoxes: []overlayBox{box}, Overlays: []string{"platform mark"}}); err != nil {
		t.Fatal(err)
	}
}

func TestCropTargetKeepsOnlyObservedPixels(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			src.Set(x, y, color.RGBA{R: 220, G: 220, B: 220, A: 255})
		}
	}
	for y := 40; y < 60; y++ {
		for x := 40; x < 60; x++ {
			src.Set(x, y, color.RGBA{R: 180, G: 10, B: 10, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, nil); err != nil {
		t.Fatal(err)
	}
	cropped, err := cropTarget(buf.Bytes(), overlayBox{X: 400, Y: 400, W: 200, H: 200})
	if err != nil {
		t.Fatal(err)
	}
	result, err := jpeg.Decode(bytes.NewReader(cropped))
	if err != nil {
		t.Fatal(err)
	}
	if result.Bounds().Dx() < 384 || result.Bounds().Dx() > 3072 || result.Bounds().Dy() < 384 || result.Bounds().Dy() > 3072 {
		t.Fatalf("unexpected crop size %v", result.Bounds())
	}
	r, g, _, _ := result.At(result.Bounds().Dx()/2, result.Bounds().Dy()/2).RGBA()
	if r <= g {
		t.Fatal("target pixels not retained")
	}
	if _, err := storage.NewLocalStore(t.TempDir()).SaveCharacterImage(cropped); err != nil {
		t.Fatalf("cropped logo is not accepted by asset storage: %v", err)
	}
	if _, err := cropTarget(buf.Bytes(), overlayBox{X: 950, Y: 950, W: 200, H: 200}); err == nil {
		t.Fatal("invalid box accepted")
	}
}
