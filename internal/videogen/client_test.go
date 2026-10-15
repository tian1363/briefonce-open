package videogen

import (
	"context"
	"github.com/tian1363/scriptagent/internal/model"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type videoConfig struct{}

func (videoConfig) GetModelRuntimeConfig(context.Context, string) (model.RuntimeConfig, error) {
	return model.RuntimeConfig{Endpoint: Endpoint, APIKey: "test-key", Model: "seedance-2.0", Provider: "zexapi", Source: "byok"}, nil
}

type videoTransport func(*http.Request) (*http.Response, error)

func (t videoTransport) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }
func TestFourModelsSubmitMultipartAndPoll(t *testing.T) {
	for _, m := range Models {
		t.Run(m.ID, func(t *testing.T) {
			c := New(videoConfig{})
			path := filepath.Join(t.TempDir(), "ref.mp4")
			os.WriteFile(path, []byte("video bytes"), 0600)
			requests := 0
			c.http.Transport = videoTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Fatal("missing auth")
				}
				response := `{"id":"task-123","status":"queued"}`
				if r.Method == "POST" {
					if r.URL.Path != "/v1/videos" {
						t.Fatal(r.URL)
					}
					if err := r.ParseMultipartForm(1 << 20); err != nil {
						t.Fatal(err)
					}
					defer r.MultipartForm.RemoveAll()
					for k, v := range map[string]string{"model": m.ID, "seconds": "15", "aspect_ratio": "21:9"} {
						if r.FormValue(k) != v {
							t.Fatalf("%s=%s", k, r.FormValue(k))
						}
					}
					if r.FormValue("resolution") != "" {
						t.Fatal("default resolution must be omitted")
					}
					if len(r.MultipartForm.Value["input_reference"]) != 1 || len(r.MultipartForm.File["input_reference"]) != 1 {
						t.Fatal("missing mixed references")
					}
				} else {
					if r.URL.Path != "/v1/videos/task-123" {
						t.Fatal(r.URL)
					}
					response = `{"status":"completed","url":"https://example.com/result.mp4"}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: http.Header{}}, nil
			})
			id, e := c.Submit(context.Background(), Request{Model: m.ID, Prompt: "test", Duration: 15, Resolution: "default", Ratio: "21:9", References: []Reference{{Kind: "reference_image", URL: "data:image/png;base64,AAAA"}, {Kind: "reference_video", Path: path}}})
			if e != nil || id != "task-123" {
				t.Fatal(id, e)
			}
			status, url, _, e := c.Poll(context.Background(), id)
			if e != nil || status != "SUCCEEDED" || url == "" || requests != 2 {
				t.Fatal(status, url, e)
			}
		})
	}
}
func TestRejectRetiredAndInvalidCombinations(t *testing.T) {
	for _, x := range []struct {
		model, mode string
		seconds     int
	}{{"wan3.0-video", "text", 5}, {"minimax-h3", "first_frame", 5}, {"seedance-2.0", "text", 30}} {
		if Validate(x.model, "default", "9:16", x.seconds, x.mode) == nil {
			t.Fatal(x)
		}
	}
	if validateEndpoint("https://workspace.maas.aliyuncs.com/api/v1/services/aigc/video-generation/video-synthesis") == nil {
		t.Fatal("old endpoint accepted")
	}
	if validateReferences(make([]Reference, 10)) == nil {
		t.Fatal("too many references")
	}
}
func TestSeedanceFirstLastOrder(t *testing.T) {
	c := New(videoConfig{})
	c.http.Transport = videoTransport(func(r *http.Request) (*http.Response, error) {
		r.ParseMultipartForm(1 << 20)
		defer r.MultipartForm.RemoveAll()
		refs := r.MultipartForm.Value["input_reference"]
		if r.FormValue("first_last_frame") != "true" || len(refs) != 2 || refs[0] != "first" || refs[1] != "last" {
			t.Fatal(refs)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":"task"}`))}, nil
	})
	_, err := c.Submit(context.Background(), Request{Model: "seedance-2.0", Prompt: "test", Resolution: "720p", Ratio: "9:16", Duration: 4, References: []Reference{{Kind: "last_frame", URL: "last"}, {Kind: "first_frame", URL: "first"}}})
	if err != nil {
		t.Fatal(err)
	}
}
