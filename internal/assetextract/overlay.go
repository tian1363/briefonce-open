package assetextract

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
)

func validTargetBox(box overlayBox) bool {
	return box.X >= 0 && box.Y >= 0 && box.W >= 8 && box.H >= 8 && box.X+box.W <= 1000 && box.Y+box.H <= 1000
}

// cropTarget preserves source pixels; it never redraws a logo or watermark.
func cropTarget(source []byte, box overlayBox) ([]byte, error) {
	if !validTargetBox(box) {
		return nil, errors.New("目标标识位置无效")
	}
	decoded, _, err := image.Decode(bytes.NewReader(source))
	if err != nil {
		return nil, err
	}
	bounds := decoded.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	marginX, marginY := int(math.Round(float64(box.W*width)/1000*.08)), int(math.Round(float64(box.H*height)/1000*.08))
	x1 := max(0, int(math.Floor(float64(box.X*width)/1000))-marginX)
	y1 := max(0, int(math.Floor(float64(box.Y*height)/1000))-marginY)
	x2 := min(width, int(math.Ceil(float64((box.X+box.W)*width)/1000))+marginX)
	y2 := min(height, int(math.Ceil(float64((box.Y+box.H)*height)/1000))+marginY)
	if x2-x1 < 8 || y2-y1 < 8 {
		return nil, errors.New("目标标识过小，无法可靠裁切")
	}
	// The asset store requires at least 384 pixels per side. Enlarge a small
	// original-pixel crop without inventing logo edges, letters, or colours.
	minSide := min(x2-x1, y2-y1)
	scale := max(1, int(math.Ceil(384/float64(minSide))))
	if (x2-x1)*scale > 3072 || (y2-y1)*scale > 3072 {
		return nil, errors.New("目标标识范围过扁，无法可靠保存")
	}
	out := image.NewRGBA(image.Rect(0, 0, (x2-x1)*scale, (y2-y1)*scale))
	for y := 0; y < out.Bounds().Dy(); y++ {
		for x := 0; x < out.Bounds().Dx(); x++ {
			out.Set(x, y, decoded.At(bounds.Min.X+x1+x/scale, bounds.Min.Y+y1+y/scale))
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 94}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// removeScreenOverlays interpolates only small, confidently identified
// screen-fixed regions. Uncertain marks are left for the model to flag.
func removeScreenOverlays(source []byte, boxes []overlayBox) []byte {
	if len(boxes) == 0 {
		return source
	}
	decoded, _, err := image.Decode(bytes.NewReader(source))
	if err != nil {
		return source
	}
	bounds := decoded.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(out, out.Bounds(), decoded, bounds.Min, draw.Src)
	changed := false
	for _, box := range boxes {
		if box.X < 0 || box.Y < 0 || box.W < 4 || box.H < 4 || box.X+box.W > 1000 || box.Y+box.H > 1000 {
			continue
		}
		if box.W > 550 || box.H > 220 || box.W*box.H > 100000 {
			continue
		}
		x1 := int(math.Round(float64(box.X*width) / 1000))
		y1 := int(math.Round(float64(box.Y*height) / 1000))
		x2 := int(math.Round(float64((box.X+box.W)*width) / 1000))
		y2 := int(math.Round(float64((box.Y+box.H)*height) / 1000))
		if x1 < 1 || y1 < 1 || x2 >= width-1 || y2 >= height-1 || x1 >= x2 || y1 >= y2 {
			continue
		}
		for x := x1; x < x2; x++ {
			top := color.RGBAModel.Convert(decoded.At(bounds.Min.X+x, bounds.Min.Y+y1-1)).(color.RGBA)
			bottom := color.RGBAModel.Convert(decoded.At(bounds.Min.X+x, bounds.Min.Y+y2)).(color.RGBA)
			for y := y1; y < y2; y++ {
				alpha := float64(y-y1+1) / float64(y2-y1+1)
				blend := func(a, b uint8) uint8 { return uint8(math.Round(float64(a)*(1-alpha) + float64(b)*alpha)) }
				out.SetRGBA(x, y, color.RGBA{R: blend(top.R, bottom.R), G: blend(top.G, bottom.G), B: blend(top.B, bottom.B), A: 255})
			}
		}
		changed = true
	}
	if !changed {
		return source
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 93}); err != nil {
		return source
	}
	return buf.Bytes()
}
