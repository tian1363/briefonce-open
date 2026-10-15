package web

import "testing"

func TestGroupOCRFramesLocatesVisibleTextAndGaps(t *testing.T) {
	frames := []ocrFrame{
		{Index: 0, Text: " 旧文案 ", Confidence: .8},
		{Index: 1, Text: "旧文案", Confidence: .7},
		{Index: 2, Text: "", Confidence: 0},
		{Index: 3, Text: "新文案", Confidence: .9},
	}
	segments := groupOCRFrames(frames, 4)
	if len(segments) != 2 || segments[0].Start != 0 || segments[0].End != 2 || segments[0].Text != "旧文案" || segments[1].Start != 3 || segments[1].End != 4 || segments[1].Source != "visual" {
		t.Fatalf("segments: %+v", segments)
	}
}
