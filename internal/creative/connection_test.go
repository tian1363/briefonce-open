package creative

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCollectorIsolationAndSessionRecheck(t *testing.T) {
	logged := false
	reads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer isolated" {
			t.Error("missing collector authentication")
		}
		var data any
		switch r.URL.Path {
		case "/api/v1/login/status":
			data = map[string]any{"is_logged_in": logged, "username": "tester"}
		case "/api/v1/login/qrcode":
			data = map[string]any{"is_logged_in": false, "img": "data:image/png;base64,AAAA", "timeout": "4m0s"}
		case "/api/v1/login/cookies":
			logged = false
			data = map[string]any{}
		case "/api/v1/feeds/detail":
			reads++
			var input map[string]any
			json.NewDecoder(r.Body).Decode(&input)
			if input["feed_id"] != "6a9d4a1c000000002b0112ca" || input["xsec_token"] != "fixture-token" || input["load_all_comments"] != false {
				t.Error("wrong note request")
			}
			images := []map[string]string{}
			for i := 0; i < 12; i++ {
				images = append(images, map[string]string{"urlDefault": "http://sns-webpic.xhscdn.com/fixture"})
			}
			data = map[string]any{"feed_id": input["feed_id"], "data": map[string]any{"note": map[string]any{"noteId": input["feed_id"], "title": "fixture title", "desc": "full fixture body", "imageList": images}}}
		default:
			t.Error("unexpected collector endpoint")
		}
		json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "collectors.json")
	data, _ := json.Marshal(map[string]Collector{"owner-a": {URL: server.URL, Token: "isolated"}})
	os.WriteFile(path, data, 0600)
	t.Setenv("SCRIPT_AGENT_XHS_COLLECTORS_FILE", path)
	a := WithCollectorOwner(context.Background(), "owner-a")
	b := WithCollectorOwner(context.Background(), "owner-b")
	if ConnectionStatus(b).Status != "not_configured" {
		t.Fatal("unconfigured owner inherited session")
	}
	if ConnectionStatus(a).Status != "login_required" {
		t.Fatal("unauthenticated marked connected")
	}
	if Connect(a).Status != "awaiting_login" {
		t.Fatal("QR unavailable")
	}
	link := "https://www.xiaohongshu.com/discovery/item/6a9d4a1c000000002b0112ca?xsec_token=fixture-token"
	if ReadNote(a, link).Status != "login_required" || reads != 0 {
		t.Fatal("read without login")
	}
	logged = true
	if ConnectionStatus(a).Status != "connected" {
		t.Fatal("did not detect login")
	}
	note := ReadNote(a, link)
	if note.Status != "parsed" || note.Description != "full fixture body" || len(note.Images) != 12 {
		t.Fatalf("full note not preserved: %+v", note)
	}
	// A newly-created request context simulates leaving and reopening the UI.
	if ConnectionStatus(WithCollectorOwner(context.Background(), "owner-a")).Status != "connected" {
		t.Fatal("reopening lost existing session")
	}
	logged = false
	if ConnectionStatus(a).Status != "login_required" {
		t.Fatal("stale connected marker")
	}
	logged = true
	if err := Disconnect(a); err != nil {
		t.Fatal(err)
	}
	if ConnectionStatus(a).Status != "login_required" {
		t.Fatal("disconnect did not clear session")
	}
	duplicate, _ := json.Marshal(map[string]Collector{"owner-a": {URL: server.URL}, "owner-b": {URL: server.URL}})
	os.WriteFile(path, duplicate, 0600)
	if ConnectionStatus(a).Status != "not_configured" {
		t.Fatal("shared collector was accepted")
	}
}
func TestCollectorUnavailableAndIncompleteResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "internal sensitive details", 500) }))
	path := filepath.Join(t.TempDir(), "collectors.json")
	data, _ := json.Marshal(map[string]Collector{"a": {URL: server.URL}})
	os.WriteFile(path, data, 0600)
	t.Setenv("SCRIPT_AGENT_XHS_COLLECTORS_FILE", path)
	state := ConnectionStatus(WithCollectorOwner(context.Background(), "a"))
	if state.Status != "unavailable" || state.Message == "internal sensitive details" {
		t.Fatal("wrong failure state")
	}
	server.Close()
	if ConnectionStatus(WithCollectorOwner(context.Background(), "a")).Status != "unavailable" {
		t.Fatal("stopped collector not detected")
	}
}
