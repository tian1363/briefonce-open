package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tian1363/scriptagent/internal/jobs"
)

func TestRenderSourceAcceptsOnlyOwnedCompletedGeneratedVideo(t *testing.T) {
	store, err := jobs.OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner, err := store.CreateUser(jobs.CreateUserInput{Email: "source-owner@example.com", PasswordHash: "x", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateUser(jobs.CreateUserInput{Email: "source-other@example.com", PasswordHash: "x", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	video, err := store.CreateVideoGeneration(jobs.CreateVideoGenerationInput{UserID: owner.ID, Mode: "image", Prompt: "test", Model: "test", Resolution: "720P", Ratio: "9:16", Duration: 1})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "generated.mp4")
	if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	h := &Handler{store: store}
	if _, err := h.renderSourcePath(owner.ID, "product", "", video.ID, "", ""); err == nil {
		t.Fatal("pending video accepted")
	}
	if err := store.UpdateVideoGeneration(video.ID, "completed", "", "", path, ""); err != nil {
		t.Fatal(err)
	}
	got, err := h.renderSourcePath(owner.ID, "product", "", video.ID, "", "")
	if err != nil || got != path {
		t.Fatalf("owner source: %q, %v", got, err)
	}
	if _, err := h.renderSourcePath(other.ID, "product", "", video.ID, "", ""); err == nil {
		t.Fatal("cross-user video accepted")
	}
	if _, err := h.renderSourcePath(owner.ID, "product", "asset", video.ID, "", ""); err == nil {
		t.Fatal("two sources accepted")
	}
}
