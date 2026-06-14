package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMimoASRClientUsesChatInputAudio(t *testing.T) {
	var requestedPath string
	var requestedModel string
	var audioData string
	var language string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		var req mimoASRRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requestedModel = req.Model
		language = req.ASROptions.Language
		if len(req.Messages) != 1 || len(req.Messages[0].Content) != 1 {
			t.Fatalf("unexpected messages: %#v", req.Messages)
		}
		audioData = req.Messages[0].Content[0].InputAudio.Data

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"转写文本"}}]}`))
	}))
	defer server.Close()

	audioPath := filepath.Join(t.TempDir(), "sample.mp3")
	if err := os.WriteFile(audioPath, []byte("fake audio"), 0644); err != nil {
		t.Fatalf("write audio: %v", err)
	}

	client := NewMimoASRClient(MimoASRClientConfig{
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
		Model:   "mimo-v2.5-asr",
		Timeout: 5,
	})

	got, err := client.TranscribeFile(audioPath, "ko")
	if err != nil {
		t.Fatalf("TranscribeFile failed: %v", err)
	}
	if got != "转写文本" {
		t.Fatalf("transcript = %q, want 转写文本", got)
	}
	if requestedPath != "/v1/chat/completions" {
		t.Fatalf("request path = %q, want /v1/chat/completions", requestedPath)
	}
	if requestedModel != "mimo-v2.5-asr" {
		t.Fatalf("model = %q, want mimo-v2.5-asr", requestedModel)
	}
	if language != "ko" {
		t.Fatalf("language = %q, want ko", language)
	}
	if !strings.HasPrefix(audioData, "data:audio/mpeg;base64,") {
		t.Fatalf("audio data should be a data URI, got prefix %.32q", audioData)
	}
}

func TestBuildSRTFromASRSegmentsUsesGlobalOffsets(t *testing.T) {
	segments := []asrSegment{
		{Start: 0, Duration: 3, Text: "第一段"},
		{Start: 60, Duration: 2.5, Text: "第二段"},
	}

	got := buildSRTFromASRSegments(segments)

	if !strings.Contains(got, "00:00:00,000 --> 00:00:03,000") {
		t.Fatalf("missing first timecode in %q", got)
	}
	if !strings.Contains(got, "00:01:00,000 --> 00:01:02,500") {
		t.Fatalf("missing second timecode in %q", got)
	}
	if !strings.Contains(got, "第二段") {
		t.Fatalf("missing transcript text in %q", got)
	}
}
