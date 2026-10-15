package assetextract

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/charactergen"
)

func TestKeyframePlanRequiresObservedDistinctViews(t *testing.T) {
	times := candidateTimes(12, 5)
	if len(times) < 5 {
		t.Fatalf("too few candidate times: %v", times)
	}
	for i, at := range times {
		if at < 0 || at >= 12 || (i > 0 && times[i-1] >= at) {
			t.Fatalf("invalid time order: %v", times)
		}
	}
	good := framePlan{Main: 1, Views: []plannedView{{Index: 1, View: "main"}, {Index: 2, View: "side"}}}
	if err := validateFramePlan(good, 3); err != nil {
		t.Fatal(err)
	}
	bad := good
	bad.Views[1].Index = 1
	if err := validateFramePlan(bad, 3); err == nil || !strings.Contains(err.Error(), "同一帧") {
		t.Fatalf("expected duplicate-frame rejection, got %v", err)
	}
	var parsed framePlan
	if err := decodeJSONText("```json\n{\"main\":0,\"views\":[{\"index\":0,\"view\":\"main\"}]}\n```", &parsed); err != nil || parsed.Main != 0 {
		t.Fatalf("parse = %+v, %v", parsed, err)
	}
	parsed = framePlan{}
	if err := decodeJSONText("结果：{\"main\":1,\"views\":[{\"index\":1,\"view\":\"main\"}]}\n补充说明：{不是 JSON}", &parsed); err != nil || parsed.Main != 1 {
		t.Fatalf("parse with trailing prose = %+v, %v", parsed, err)
	}
	plan, err := decodeFramePlanText("```json\n{\"main\":2,\"views\":[{\"index\":2,\"view\":\"main\"}],\"overlays\":[{\"type\":\"watermark\",\"description\":\"右上角水印\"}],\"garment_marks\":[{\"description\":\"衣服胸前标识\"}],\"uncertain\":[]}\n```")
	if err != nil || plan.Main != 2 || len(plan.Overlays) != 1 || plan.Overlays[0] != "右上角水印" || len(plan.GarmentMarks) != 1 {
		t.Fatalf("model object notes = %+v, %v", plan, err)
	}
	plan, err = decodeFramePlanText(`{"main":4,"views":[{"index":4,"view":"main"}],"overlays":[{"text":"微博水印","location":"右上角"}],"garment_marks":[{"brand_logo":"蓝色三角形","location":"左胸"}]}`)
	if err != nil || plan.Main != 4 || plan.Overlays[0] != "右上角：微博水印" || plan.GarmentMarks[0] != "左胸：蓝色三角形" {
		t.Fatalf("alternate model notes = %+v, %v", plan, err)
	}
	parsed = framePlan{Main: 7}
	if err := decodeJSONText(`{"main":2,"views":[{"index":2,"view":"main"}],"overlays":[{"description":"水印"}]}`, &parsed); err == nil || parsed.Main != 7 {
		t.Fatalf("failed decode mutated target = %+v, %v", parsed, err)
	}
}

func TestMarkCandidateTimesStayNearSelectedMoment(t *testing.T) {
	wide := markCandidateTimes(30, 12.25, false)
	if len(wide) != 7 || wide[0] != 9.25 || wide[len(wide)-1] != 15.25 {
		t.Fatalf("unexpected mark candidate times: %v", wide)
	}
	retry := markCandidateTimes(30, 12.25, true)
	if len(retry) != 3 || retry[0] != 11.75 || retry[1] != 12.25 || retry[2] != 12.75 {
		t.Fatalf("unexpected retry times: %v", retry)
	}
	nearStart := markCandidateTimes(2, 0, false)
	for i, second := range nearStart {
		if second < 0 || second >= 2 || (i > 0 && nearStart[i-1] >= second) {
			t.Fatalf("invalid boundary times: %v", nearStart)
		}
	}
}

func TestFramePlanNormalizesIndependentAssetKinds(t *testing.T) {
	cases := []struct {
		kind string
		raw  string
	}{
		{"person", `{"main":4,"views":[{"index":1,"view":"detail"}],"overlays":[{"text":"微博水印","position":"右上角"}],"overlay_boxes":[{"x":680,"y":35,"w":200,"h":45}]}`},
		{"scene", `{"main":4,"views":[{"index":4,"view":"main"},{"index":2,"view":"side"}],"garment_marks":[{"brand":"Klattermusen","location":"左胸"}]}`},
		{"brand_logo", `{"main":7,"views":[{"index":7,"view":"main"}],"target_boxes":[{"x":685,"y":730,"w":40,"h":40}],"overlays":[{"text":"微博水印"}],"overlay_boxes":[[693,41,910,83]],"garment_marks":[{"x":685,"y":730,"w":40,"h":40}]}`},
		{"video_watermark", `{"main":7,"views":[{"index":7,"view":"main"}],"target_boxes":[],"overlays":[{"text":"微博水印"}],"overlay_boxes":[[682,49,951,99]]}`},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			plan, err := decodeFramePlanText(tc.raw)
			if err != nil {
				t.Fatal(err)
			}
			plan = normalizeFramePlan(tc.kind, plan)
			if err := validateFramePlan(plan, 9); err != nil {
				t.Fatal(err)
			}
			if err := validateTargetPlan(tc.kind, plan); err != nil {
				t.Fatal(err)
			}
			if len(plan.Views) != 1 || plan.Views[0].Index != plan.Main {
				t.Fatalf("views = %+v", plan.Views)
			}
		})
	}
}

func TestDecodeFramePlanNormalizesObservedModelVariants(t *testing.T) {
	cases := []struct {
		raw      string
		main     int
		views    int
		overlays string
		garment  string
		box      overlayBox
	}{
		{`{"main":4,"views":[{"index":2,"view":"detail"}],"overlays":[{"text":"账号水印","position":"右上角"}],"overlay_boxes":[{"x":700,"y":30,"w":200,"h":45}],"garment_marks":[{"brand":"Klattermusen","type":"Logo","location":"左胸"}]}`, 4, 2, "右上角：账号水印", "左胸：Klattermusen", overlayBox{700, 30, 200, 45}},
		{`{"main":7,"views":[{"index":7,"view":"main"}],"overlays":[{"text":"账号水印","box":[682,49,951,99]}],"overlay_boxes":[[682,49,951,99]],"garment_marks":[{"description":"实体标识","box":[506,760,534,782]}]}`, 7, 1, "账号水印", "实体标识", overlayBox{682, 49, 269, 50}},
	}
	for _, tc := range cases {
		plan, err := decodeFramePlanText(tc.raw)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Main != tc.main || len(plan.Views) != tc.views || plan.Views[0] != (plannedView{Index: tc.main, View: "main"}) || plan.Overlays[0] != tc.overlays || plan.GarmentMarks[0] != tc.garment || plan.OverlayBoxes[0] != tc.box {
			t.Fatalf("unexpected normalized plan: %+v", plan)
		}
		if err := validateFramePlan(plan, 9); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := decodeFramePlanText(`{"views":[{"index":0,"view":"main"}]}`); err == nil {
		t.Fatal("accepted missing main index")
	}
	if _, err := decodeFramePlanText(`{"main":0,"overlay_boxes":[[10,10,5,5]]}`); err == nil {
		t.Fatal("accepted inverted box")
	}
}

func TestRemoveOnlyConfirmedSmallScreenOverlay(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 400, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 400; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 90, G: 105, B: 115, A: 255})
		}
	}
	for y := 180; y < 220; y++ {
		for x := 120; x < 280; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 250, G: 0, B: 0, A: 255})
		}
	}
	var raw bytes.Buffer
	if err := jpeg.Encode(&raw, img, nil); err != nil {
		t.Fatal(err)
	}
	clean := removeScreenOverlays(raw.Bytes(), []overlayBox{{X: 300, Y: 450, W: 400, H: 100}})
	result, err := jpeg.Decode(bytes.NewReader(clean))
	if err != nil {
		t.Fatal(err)
	}
	center := color.RGBAModel.Convert(result.At(200, 200)).(color.RGBA)
	outside := color.RGBAModel.Convert(result.At(40, 40)).(color.RGBA)
	if center.R > 140 || outside.R < 70 || outside.R > 110 {
		t.Fatalf("overlay not removed or outside changed: center=%+v outside=%+v", center, outside)
	}
	unchanged := removeScreenOverlays(raw.Bytes(), []overlayBox{{X: 0, Y: 0, W: 900, H: 900}})
	if !bytes.Equal(raw.Bytes(), unchanged) {
		t.Fatal("large untrusted mask changed image")
	}
}

func TestExtractReferenceClip(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	video := filepath.Join(t.TempDir(), "source.mp4")
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=640x480:rate=8:duration=5", "-c:v", "libx264", "-pix_fmt", "yuv420p", video)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("source video: %v %s", err, out)
	}
	data, err := extractReferenceClip(context.Background(), video, 2.5, 5, []overlayBox{{X: 200, Y: 200, W: 100, H: 50}})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 1000 || !bytes.Contains(data[:min(20, len(data))], []byte("ftyp")) {
		t.Fatalf("invalid clip: %d bytes", len(data))
	}
	if _, err := os.Stat(video); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedAssetGateRejectsWatermarkAndMismatch(t *testing.T) {
	if !(imageReview{}).acceptable() {
		t.Fatal("clean image rejected")
	}
	for name, review := range map[string]imageReview{
		"watermark":       {WatermarkTransferred: true},
		"ambiguous logo":  {OverlayUncertain: true},
		"changed product": {ProductMismatch: true},
		"changed person":  {PersonMismatch: true},
		"invented detail": {UnsupportedDetails: true},
	} {
		if review.acceptable() {
			t.Fatalf("%s was accepted", name)
		}
	}
}

func TestReviewRetryTargetsOnlyUnconfirmedMarks(t *testing.T) {
	mark := imageReview{UnsupportedDetails: true}
	if !mark.markOnlyFailure() || !strings.Contains(retryExtractionInstruction(mark), "标识") {
		t.Fatal("unconfirmed mark should get a focused retry")
	}
	if got := reviewFailureSummary(mark, nil); !strings.Contains(got, "原画面没有") || strings.Contains(got, "unsupported_details") {
		t.Fatalf("unclear review message: %s", got)
	}
	mismatch := imageReview{ProductMismatch: true, UnsupportedDetails: true}
	if mismatch.markOnlyFailure() || !strings.Contains(retryExtractionInstruction(mismatch), "商品颜色") {
		t.Fatal("changed products must retry from the original frame")
	}
}

func TestPermanentImageModelFailureDoesNotTriggerGenerationRetries(t *testing.T) {
	if !nonRetryableImageError(charactergen.ErrAccessDenied) || !nonRetryableImageError(charactergen.ErrModelUnavailable) || !nonRetryableImageError(errors.New("图片模型 API Key 无效")) {
		t.Fatal("configuration errors should stop retries")
	}
	if nonRetryableImageError(errors.New("图片服务连接失败或超时")) {
		t.Fatal("transient connection errors should remain retryable")
	}
}

func TestFramePlanObservedMalformedCoordinateAndLabels(t *testing.T) {
	modelText := "```json\n{\"main\":4,\"views\":[{\"index\":4,\"view\":\"main\"}],\"overlays\":[{\"x\":738,\"y\":56,\"w\":200,\"h\":35\",\"label\":\"右上角水印\"}],\"garment_marks\":[{\"x\":621,\"y\":730,\"w\":40,\"h\":25,\"label\":\"左胸实体标识\"}],\"uncertain\":[]}\n```"
	plan, err := decodeFramePlanText(modelText)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Overlays) != 1 || plan.Overlays[0] != "右上角水印" || len(plan.OverlayBoxes) != 1 || plan.OverlayBoxes[0] != (overlayBox{738, 56, 200, 35}) || len(plan.GarmentMarks) != 1 {
		t.Fatalf("observed response was not normalized: %+v", plan)
	}
}

func TestNormalizeFramePlanKeepsOnlyRelevantViews(t *testing.T) {
	plan := framePlan{Main: 4, Views: []plannedView{{Index: 4, View: "main"}, {Index: 2, View: "detail"}}, Overlays: []string{"右上角水印"}, OverlayBoxes: []overlayBox{{680, 32, 105, 40}}}
	for _, kind := range []string{"person", "scene", "pose", "action", "camera"} {
		got := normalizeFramePlan(kind, plan)
		if len(got.Views) != 1 || got.Views[0].Index != 4 {
			t.Fatalf("%s views: %+v", kind, got.Views)
		}
	}
	got := normalizeFramePlan("video_watermark", plan)
	if len(got.TargetBoxes) != 1 || got.TargetBoxes[0] != plan.OverlayBoxes[0] {
		t.Fatalf("watermark target: %+v", got.TargetBoxes)
	}
	got = normalizeFramePlan("product", plan)
	if len(got.Views) != 2 {
		t.Fatalf("product views: %+v", got.Views)
	}
}

func TestGeneratedOverlayCleaningStaysOutOfProductArea(t *testing.T) {
	got := safeGeneratedOverlayBoxes([]overlayBox{{680, 30, 200, 50}, {350, 500, 200, 50}, {400, 900, 200, 50}})
	if len(got) != 1 || got[0] != (overlayBox{680, 30, 200, 50}) {
		t.Fatalf("unsafe cleaning boxes: %+v", got)
	}
}
