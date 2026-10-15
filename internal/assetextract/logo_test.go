package assetextract

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestRemoveLogoChromaProducesTransparentPNG(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	for y := 0; y < 512; y++ {
		for x := 0; x < 512; x++ {
			c := color.NRGBA{R: 255, B: 255, A: 255}
			if x >= 150 && x < 350 && y >= 200 && y < 290 {
				c = color.NRGBA{R: 20, G: 40, B: 90, A: 255}
			}
			src.SetNRGBA(x, y, c)
		}
	}
	var input bytes.Buffer
	if err := png.Encode(&input, src); err != nil {
		t.Fatal(err)
	}
	data, err := removeLogoChroma(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, alpha := got.At(0, 0).RGBA(); alpha != 0 {
		t.Fatal("background was not transparent")
	}
	if _, _, _, alpha := got.At(200, 240).RGBA(); alpha != 0xffff {
		t.Fatal("logo foreground was removed")
	}
}

func TestRemoveLogoChromaRejectsOpaqueBackground(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	for y := 0; y < 512; y++ {
		for x := 0; x < 512; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}
	var input bytes.Buffer
	_ = png.Encode(&input, src)
	if _, err := removeLogoChroma(input.Bytes()); err == nil {
		t.Fatal("opaque background accepted as transparent logo")
	}
}
