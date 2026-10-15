package web

import "testing"

func TestValidateExtractionKinds(t *testing.T) {
	if err := validateExtractionKinds([]string{"person", "product", "brand_logo", "video_watermark", "scene", "pose", "action", "camera"}); err != nil {
		t.Fatal(err)
	}
	for _, kinds := range [][]string{nil, {"person", "person"}, {"unknown"}} {
		if err := validateExtractionKinds(kinds); err == nil {
			t.Fatalf("accepted kinds %v", kinds)
		}
	}
}
