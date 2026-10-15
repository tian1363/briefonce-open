package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type transcriptionRequest struct {
	ProductID       string `json:"product_id"`
	SourceAssetID   string `json:"source_asset_id"`
	SourceVideoID   string `json:"source_video_id"`
	SourceBatchID   string `json:"source_batch_id"`
	SourceVariantID string `json:"source_variant_id"`
}

type recognizedSegment struct {
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Text        string  `json:"text"`
	NeedsReview bool    `json:"needs_review"`
	Source      string  `json:"source,omitempty"`
}

func (h *Handler) transcribeRenderSource(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromRequest(r)
	key := ""
	endpoint := os.Getenv("SCRIPT_AGENT_ASR_ENDPOINT")
	modelName := os.Getenv("SCRIPT_AGENT_ASR_MODEL")
	managed := false
	user, _ := h.store.GetUser(userID)
	trialMember := h.store.ManagedEnabled(userID) && user.Role != "admin"
	if h.cfg.AllowManagedMode && h.store.ManagedEnabled(userID) {
		if runtime, err := h.store.GetModelRuntimeConfig(r.Context(), "speech_recognition"); err == nil && runtime.Source == "managed" {
			key, endpoint, modelName = runtime.APIKey, runtime.Endpoint, runtime.Model
			managed = true
		}
	}
	if !managed && !trialMember {
		if settings, err := h.store.GetUserModelSettingsForCapability(userID, "speech_recognition"); err == nil && settings.Mode == "byok" {
			key, endpoint, modelName = settings.APIKey, settings.Endpoint, settings.Model
		}
	}
	if key == "" {
		writeError(w, 503, errors.New("未配置语音识别服务。请在设置中填写语音识别 API Key，或手动添加逐句字幕"))
		return
	}
	if !h.authLimit.allow("transcribe:"+userID, 5, time.Minute) {
		writeError(w, 429, errors.New("识别请求较多，请稍后再试"))
		return
	}
	var input transcriptionRequest
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, errors.New("视频来源无效"))
		return
	}
	if _, err := h.store.GetUserProduct(userID, input.ProductID); err != nil {
		writeError(w, 400, errors.New("请选择自己的产品资料"))
		return
	}
	path, err := h.renderSourcePath(userID, input.ProductID, input.SourceAssetID, input.SourceVideoID, input.SourceBatchID, input.SourceVariantID)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	seconds, err := probeRenderDuration(r.Context(), path)
	if err != nil || seconds <= 0 || seconds > 30 {
		writeError(w, 400, errors.New("原视频需可读取且不超过 30 秒"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	chargeID := ""
	if managed {
		chargeID, err = h.store.ReserveUsage(ctx, "speech_recognition", 1)
		if err != nil {
			writeError(w, http.StatusPaymentRequired, err)
			return
		}
	}
	segments, err := transcribeAudioWithConfig(ctx, path, key, endpoint, modelName)
	_ = h.store.FinishUsage(chargeID, err == nil)
	if err != nil {
		writeError(w, 502, err)
		return
	}
	writeJSON(w, 200, map[string]any{"segments": segments, "duration": seconds})
}

func transcribeAudio(ctx context.Context, videoPath, apiKey string) ([]recognizedSegment, error) {
	return transcribeAudioWithConfig(ctx, videoPath, apiKey, os.Getenv("SCRIPT_AGENT_ASR_ENDPOINT"), os.Getenv("SCRIPT_AGENT_ASR_MODEL"))
}

func transcribeAudioWithConfig(ctx context.Context, videoPath, apiKey, endpoint, modelName string) ([]recognizedSegment, error) {
	temp, err := os.CreateTemp("", "scriptagent-asr-*.mp3")
	if err != nil {
		return nil, errors.New("无法准备语音识别")
	}
	audioPath := temp.Name()
	temp.Close()
	defer os.Remove(audioPath)
	output, err := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", videoPath, "-vn", "-ac", "1", "-ar", "16000", "-b:a", "64k", audioPath).CombinedOutput()
	if err != nil {
		_ = output
		return nil, errors.New("无法提取原视频音轨；请确认视频有可识别的人声")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "audio.mp3")
	if err != nil {
		return nil, err
	}
	audio, err := os.Open(audioPath)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, audio)
	audio.Close()
	if err != nil {
		return nil, err
	}
	if modelName == "" {
		modelName = "whisper-1"
	}
	writer.WriteField("model", modelName)
	writer.WriteField("response_format", "verbose_json")
	writer.WriteField("timestamp_granularities[]", "segment")
	writer.Close()
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/audio/transcriptions"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := (&http.Client{Timeout: 90 * time.Second}).Do(request)
	if err != nil {
		return nil, errors.New("语音识别服务暂时不可用")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("语音识别失败（%d），请检查 API Key、模型和额度", response.StatusCode)
	}
	var result struct {
		Segments []struct {
			Start        float64 `json:"start"`
			End          float64 `json:"end"`
			Text         string  `json:"text"`
			AvgLogprob   float64 `json:"avg_logprob"`
			NoSpeechProb float64 `json:"no_speech_prob"`
		} `json:"segments"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&result); err != nil {
		return nil, errors.New("语音识别结果无法读取")
	}
	segments := make([]recognizedSegment, 0, len(result.Segments))
	for _, item := range result.Segments {
		text := strings.TrimSpace(item.Text)
		if text == "" || item.End <= item.Start {
			continue
		}
		segments = append(segments, recognizedSegment{Start: item.Start, End: item.End, Text: text, NeedsReview: item.AvgLogprob < -1 || item.NoSpeechProb > .6, Source: "speech"})
	}
	if len(segments) == 0 {
		return nil, errors.New("未识别到带时间点的人声字幕；可以手动添加")
	}
	return segments, nil
}
