package creative

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type imageRoundTrip func(*http.Request) (*http.Response, error)

func (f imageRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestNoteImageProxyValidation(t *testing.T) {
	for _, raw := range []string{"http://sns-webpic.xhscdn.com/a", "https://localhost/a", "https://xhscdn.com.evil.test/a", "https://user:secret@xhscdn.com/a", "https://xhscdn.com:444/a", "https://127.0.0.1/a"} {
		if validateNoteImageURL(raw) == nil {
			t.Fatalf("accepted unsafe image: %s", raw)
		}
	}
	client := &http.Client{Transport: imageRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Fatal("forwarded credentials")
		}
		if r.Header.Get("Referer") != "https://www.xiaohongshu.com/" {
			t.Fatal("missing image referer")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("RIFF1234WEBPVP8 sample")), Header: make(http.Header)}, nil
	})}
	data, kind, err := fetchNoteImage(context.Background(), "https://sns-webpic-qc.xhscdn.com/a?sign=fixture", client)
	if err != nil || kind != "image/webp" || len(data) == 0 {
		t.Fatalf("raster not served: %s %v", kind, err)
	}
	client.Transport = imageRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("<html>login</html>")), Header: make(http.Header)}, nil
	})
	if _, _, err := fetchNoteImage(context.Background(), "https://xhscdn.com/a", client); err == nil {
		t.Fatal("served HTML as an image")
	}
}
