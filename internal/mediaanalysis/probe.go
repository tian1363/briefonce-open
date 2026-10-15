package mediaanalysis

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Evidence struct {
	DurationSeconds float64
	AudioTrack      bool
	AudioKnown      bool
	Cuts            []float64
}

var sceneTimePattern = regexp.MustCompile(`pts_time:([0-9]+(?:\.[0-9]+)?)`)

// Inspect supplies timing and audio-track facts. A track alone does not prove
// that speech or music is audible; those require listening to the source.
func Inspect(ctx context.Context, path string) Evidence {
	var result Evidence
	if strings.TrimSpace(path) == "" {
		return result
	}
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(probeCtx, "ffprobe", "-v", "error", "-show_entries", "format=duration:stream=codec_type", "-of", "json", path).Output()
	if err != nil {
		return result
	}
	var metadata struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
		} `json:"streams"`
	}
	if json.Unmarshal(output, &metadata) != nil {
		return result
	}
	result.AudioKnown = true
	result.DurationSeconds, _ = strconv.ParseFloat(metadata.Format.Duration, 64)
	for _, stream := range metadata.Streams {
		if stream.CodecType == "audio" {
			result.AudioTrack = true
		}
	}
	cutCtx, cutCancel := context.WithTimeout(ctx, 45*time.Second)
	defer cutCancel()
	cutOutput, _ := exec.CommandContext(cutCtx, "ffmpeg", "-hide_banner", "-loglevel", "info", "-i", path, "-vf", `select=gt(scene\,0.25),showinfo`, "-an", "-f", "null", "-").CombinedOutput()
	for _, match := range sceneTimePattern.FindAllStringSubmatch(string(cutOutput), 80) {
		if second, err := strconv.ParseFloat(match[1], 64); err == nil && second > 0 {
			result.Cuts = append(result.Cuts, second)
		}
	}
	return result
}

func (e Evidence) PromptHint() string {
	parts := []string{}
	if e.DurationSeconds > 0 {
		parts = append(parts, fmt.Sprintf("机器读取时长约 %.2f 秒", e.DurationSeconds))
	} else {
		parts = append(parts, "机器未能读取时长")
	}
	if e.AudioKnown {
		if e.AudioTrack {
			parts = append(parts, "文件有音轨；音轨存在不代表一定有旁白或 BGM，须根据可听声音判断")
		} else {
			parts = append(parts, "文件没有音轨；原片旁白、BGM、音效均为无")
		}
	} else {
		parts = append(parts, "未能检查音轨；旁白和 BGM 不可据此推断")
	}
	if len(e.Cuts) == 0 {
		parts = append(parts, "未检测到明显切镜；不代表全片只有一个镜头")
	} else {
		cuts := make([]string, 0, len(e.Cuts))
		for _, cut := range e.Cuts {
			cuts = append(cuts, fmt.Sprintf("%.2f", cut))
		}
		parts = append(parts, "候选切镜秒数（机器提示，须核对画面）："+strings.Join(cuts, "、"))
	}
	return strings.Join(parts, "；")
}
