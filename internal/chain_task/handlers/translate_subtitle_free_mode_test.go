package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/types"
	"go.uber.org/zap"
)

func TestTranslateSubtitleExecuteSkipsCloudTranslationWhenDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	videoID := "free-mode-video"

	sourceSRT := filepath.Join(tmpDir, videoID+".srt")
	if err := os.WriteFile(sourceSRT, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello world\n\n"), 0644); err != nil {
		t.Fatalf("write source srt: %v", err)
	}

	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false
	cfg.DeepSeekTransConfig.ApiKey = ""

	task := &TranslateSubtitle{
		BaseTask: base.BaseTask{
			Name: "translate_subtitles",
			StateManager: &manager.StateManager{
				VideoID:    videoID,
				CurrentDir: tmpDir,
			},
		},
		App:        &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
		GroupSize:  25,
		MaxWorkers: 1,
	}

	context := map[string]interface{}{}

	if ok := task.Execute(context); !ok {
		t.Fatalf("expected free mode to skip cloud translation successfully, context=%v", context)
	}

	if _, exists := context["error"]; exists {
		t.Fatalf("expected no error in free mode, context=%v", context)
	}

	if got := context["translation_skipped"]; got != "cloud_translation_disabled" {
		t.Fatalf("expected cloud translation skip marker, got %v", got)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "zh.srt")); !os.IsNotExist(err) {
		t.Fatalf("expected no external cloud translation output, stat err=%v", err)
	}
}

func TestTranslateSubtitleUsesDeepLXWhenConfigured(t *testing.T) {
	var requests []map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}

		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requests = append(requests, req)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"data": "ZH:" + req["text"],
		})
	}))
	defer server.Close()

	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false
	cfg.DeepLXConfig = &types.DeepLXConfig{
		Enabled:    true,
		Endpoint:   server.URL,
		SourceLang: "EN",
		TargetLang: "ZH",
		Timeout:    5,
	}

	task := &TranslateSubtitle{
		App:        &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
		GroupSize:  25,
		MaxWorkers: 1,
	}

	got, err := task.translateGroupSimple([]string{"Hello world", "Good night"})
	if err != nil {
		t.Fatalf("translateGroupSimple() returned error: %v", err)
	}

	want := []string{"ZH:Hello world", "ZH:Good night"}
	if len(got) != len(want) {
		t.Fatalf("translated count = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("translated[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	if len(requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(requests))
	}
	if requests[0]["source_lang"] != "EN" || requests[0]["target_lang"] != "ZH" {
		t.Fatalf("unexpected language payload: %#v", requests[0])
	}
}

func TestTranslateSubtitleUsesOpenAICompatibleWhenConfigured(t *testing.T) {
	var gotAuth string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path

		var req OpenAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "local-test-model" {
			t.Fatalf("model = %q, want local-test-model", req.Model)
		}

		_ = json.NewEncoder(w).Encode(OpenAIResponse{
			Choices: []OpenAIChoice{
				{Message: OpenAIMessage{Role: "assistant", Content: "你好###SENTENCE_BREAK###晚安"}},
			},
		})
	}))
	defer server.Close()

	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false
	cfg.DeepLXConfig.Enabled = false
	cfg.OpenAICompatibleConfig = &types.OpenAICompatibleConfig{
		Enabled:     true,
		APIKey:      "local-test-key",
		BaseURL:     server.URL,
		Model:       "local-test-model",
		Timeout:     5,
		MaxTokens:   1000,
		Temperature: 0.1,
	}

	task := &TranslateSubtitle{
		App:        &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
		GroupSize:  25,
		MaxWorkers: 1,
	}

	got, err := task.translateGroupSimple([]string{"Hello", "Good night"})
	if err != nil {
		t.Fatalf("translateGroupSimple() returned error: %v", err)
	}

	want := []string{"你好", "晚安"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("translated[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("request path = %q, want /v1/chat/completions", gotPath)
	}
	if gotAuth != "Bearer local-test-key" {
		t.Fatalf("Authorization = %q, want Bearer local-test-key", gotAuth)
	}
}
