package mediaanalysis

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectDistinguishesAudioTrackFromSpeechOrMusic(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	for _, test := range []struct {
		name, audioInput string
		wantAudio        bool
	}{
		{"silent-video", "", false},
		{"video-with-audio-track", "-f lavfi -i sine=frequency=440:duration=1 -shortest -c:a aac", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sample.mp4")
			args := []string{"-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=red:s=64x64:d=1", "-c:v", "mpeg4"}
			if test.audioInput != "" {
				args = []string{"-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=red:s=64x64:d=1", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-shortest", "-c:v", "mpeg4", "-c:a", "aac"}
			}
			args = append(args, path)
			if output, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
				t.Fatalf("create video: %v: %s", err, output)
			}
			evidence := Inspect(context.Background(), path)
			if !evidence.AudioKnown || evidence.AudioTrack != test.wantAudio || evidence.DurationSeconds <= 0 {
				t.Fatalf("unexpected media evidence: %+v", evidence)
			}
			if test.wantAudio && !strings.Contains(evidence.PromptHint(), "音轨存在不代表") {
				t.Fatalf("audio track was treated as speech/music: %s", evidence.PromptHint())
			}
		})
	}
}
