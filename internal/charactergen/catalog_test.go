package charactergen

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestModelSpecificValidation(t *testing.T) {
	for _, o := range []Options{{Model: "gpt-image-2", Size: "3840x2160"}, {Model: "gpt-image-2", Size: "1024x1024", AspectRatio: "1:1"}, {Model: "nano_banana_pro-1K", ImageSize: "4K"}, {Model: "nano_banana_2", Size: "1024x1024"}, {Model: "nano-banana-2.1", ImageSize: "8K"}, {Model: "nano-banana-2", AspectRatio: "1:1"}} {
		if e := o.Validate(); e == nil {
			t.Fatalf("accepted %+v", o)
		}
	}
	for _, o := range []Options{{Model: "gpt-image-2-4K", Size: "3840x2160"}, {Model: "nano-banana-2.1", ImageSize: "4K"}, {Model: "nano_banana_pro-2K", AspectRatio: "auto"}} {
		if e := o.Validate(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestReferenceAndPromptLimits(t *testing.T) {
	o := Options{Model: "nano_banana_2"}
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	if _, e := RequestBody(o, "prompt", [][]byte{b.Bytes()}); e != nil {
		t.Fatal(e)
	}
	refs := make([][]byte, 9)
	for i := range refs {
		refs[i] = b.Bytes()
	}
	if e := ValidateInput(o, "p", refs); e == nil {
		t.Fatal("nine accepted")
	}
	if e := ValidateInput(o, strings.Repeat("中", 10001), nil); e == nil {
		t.Fatal("unicode overlimit accepted")
	}
	if e := ValidateInput(o, "p", [][]byte{[]byte("not png")}); e == nil {
		t.Fatal("invalid image accepted")
	}
	body, e := RequestBody(Options{Model: "nano_banana_pro-4K", AspectRatio: "3:4"}, "p", nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := body["image_size"]; ok {
		t.Fatal("suffix model leaked size")
	}
}
func TestURLAndTaskCompatibility(t *testing.T) {
	for _, raw := range []string{"http://example.com/image.png", "https://user:secret@example.com/i", "https://example.com:8080/i"} {
		if PublicImageURL(raw) == nil {
			t.Fatal("unsafe URL accepted")
		}
	}
	task := Task{ImageURL: "https://example.com/image.png"}
	if task.ResultURL() != task.ImageURL {
		t.Fatal("nano result ignored")
	}
}
