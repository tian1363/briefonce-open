package web

import (
	"mime"
	"net/http/httptest"
	"testing"
)

func TestImageDownloadHeaders(t *testing.T) {
	for _, download := range []bool{false, true} {
		url := "/api/assets/a/file"
		if download {
			url += "?download=1&filename=外套"
		}
		w := httptest.NewRecorder()
		setImageDownloadHeaders(w, httptest.NewRequest("GET", url, nil), "original.png", "/uploads/a.png")
		if !download {
			if w.Header().Get("Content-Disposition") != "" {
				t.Fatal("preview changed")
			}
			continue
		}
		kind, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
		if err != nil || kind != "attachment" || params["filename"] != "外套.png" {
			t.Fatalf("invalid attachment: %v %v %v", kind, params, err)
		}
	}
}
