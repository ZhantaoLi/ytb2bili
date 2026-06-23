package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/base"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"go.uber.org/zap"
)

func TestTranslateSubtitleExecuteUsesFreeBingWhenCloudTranslationDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	videoID := "free-mode-video"

	sourceSRT := filepath.Join(tmpDir, videoID+".srt")
	if err := os.WriteFile(sourceSRT, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello world\n\n"), 0644); err != nil {
		t.Fatalf("write source srt: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			_, _ = w.Write([]byte("test-token"))
		case "/translate":
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Fatalf("Authorization = %q, want Bearer test-token", got)
			}
			var req []map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"translations": []map[string]string{{"text": "你好，世界"}}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	restoreFreeTranslateEndpoints(t, server.URL+"/auth", server.URL+"/translate", freeGoogleTranslateURL)

	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false
	cfg.DeepSeekTransConfig.ApiKey = ""
	cfg.DeepLXConfig.Enabled = false
	cfg.OpenAICompatibleConfig.Enabled = false

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

	if got := context["translated_count"]; got != 1 {
		t.Fatalf("translated_count = %v, want 1", got)
	}

	zhContent, err := os.ReadFile(filepath.Join(tmpDir, "zh.srt"))
	if err != nil {
		t.Fatalf("expected zh.srt in free mode: %v", err)
	}
	if !strings.Contains(string(zhContent), "你好，世界") {
		t.Fatalf("zh.srt = %q, want translated text", string(zhContent))
	}
}

func TestTranslateSubtitleExecutePrefersOriginalSRTOverStaleVideoIDSRT(t *testing.T) {
	tmpDir := t.TempDir()
	videoID := "bcut-source-video"

	enSRT := filepath.Join(tmpDir, "en.srt")
	if err := os.WriteFile(enSRT, []byte("1\n00:00:01,000 --> 00:00:02,000\nFresh Bcut source\n\n"), 0644); err != nil {
		t.Fatalf("write en srt: %v", err)
	}
	staleSRT := filepath.Join(tmpDir, videoID+".srt")
	if err := os.WriteFile(staleSRT, []byte("1\n00:00:00,000 --> 00:00:00,000\nstale placeholder\n\n"), 0644); err != nil {
		t.Fatalf("write stale srt: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			_, _ = w.Write([]byte("test-token"))
		case "/translate":
			var req []map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if len(req) != 1 || req[0]["Text"] != "Fresh Bcut source" {
				t.Fatalf("translated source = %#v, want en.srt content", req)
			}
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"translations": []map[string]string{{"text": "新的必剪字幕"}}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	restoreFreeTranslateEndpoints(t, server.URL+"/auth", server.URL+"/translate", freeGoogleTranslateURL)

	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false
	cfg.DeepLXConfig.Enabled = false
	cfg.OpenAICompatibleConfig.Enabled = false

	task := &TranslateSubtitle{
		BaseTask: base.BaseTask{
			Name: "translate_subtitles",
			StateManager: &manager.StateManager{
				VideoID:     videoID,
				CurrentDir:  tmpDir,
				OriginalSRT: enSRT,
			},
		},
		App:        &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
		GroupSize:  25,
		MaxWorkers: 1,
	}

	context := map[string]interface{}{}
	if ok := task.Execute(context); !ok {
		t.Fatalf("Execute() = false, context=%v", context)
	}
	if got := context["en_srt_path"]; got != enSRT {
		t.Fatalf("en_srt_path = %v, want %s", got, enSRT)
	}

	zhContent, err := os.ReadFile(filepath.Join(tmpDir, "zh.srt"))
	if err != nil {
		t.Fatalf("read zh.srt: %v", err)
	}
	if strings.Contains(string(zhContent), "stale") {
		t.Fatalf("zh.srt used stale subtitle: %q", string(zhContent))
	}
	if !strings.Contains(string(zhContent), "新的必剪字幕") {
		t.Fatalf("zh.srt = %q, want translated Bcut subtitle", string(zhContent))
	}
}

func TestTranslateSubtitleFailsWhenSourceSubtitleMissing(t *testing.T) {
	tmpDir := t.TempDir()
	videoID := "missing-source-subtitle"

	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false

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
	if ok := task.Execute(context); ok {
		t.Fatalf("Execute() = true, want false when source subtitle is missing; context=%v", context)
	}
	if _, exists := context["error"]; !exists {
		t.Fatalf("expected context error for missing source subtitle, context=%v", context)
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

func TestTranslateSubtitleFreeProviderFallsBackToGoogle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			http.Error(w, "bing unavailable", http.StatusBadGateway)
		case "/google":
			if got := r.URL.Query().Get("tl"); got != "zh-CN" {
				t.Fatalf("google tl = %q, want zh-CN", got)
			}
			_, _ = w.Write([]byte(`<div class="result-container">晚安</div>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	restoreFreeTranslateEndpoints(t, server.URL+"/auth", server.URL+"/translate", server.URL+"/google")

	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false
	cfg.DeepLXConfig.Enabled = false
	cfg.OpenAICompatibleConfig.Enabled = false

	task := &TranslateSubtitle{
		App:        &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
		GroupSize:  25,
		MaxWorkers: 1,
	}

	got, err := task.translateGroupSimple([]string{"Good night"})
	if err != nil {
		t.Fatalf("translateGroupSimple() returned error: %v", err)
	}
	if len(got) != 1 || got[0] != "晚安" {
		t.Fatalf("translated = %#v, want google fallback result", got)
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

func restoreFreeTranslateEndpoints(t *testing.T, bingAuth, bingTranslate, google string) {
	t.Helper()

	oldBingAuth := freeBingAuthEndpoint
	oldBingTranslate := freeBingTranslateEndpoint
	oldGoogle := freeGoogleTranslateURL

	freeBingAuthEndpoint = bingAuth
	freeBingTranslateEndpoint = bingTranslate
	freeGoogleTranslateURL = google

	t.Cleanup(func() {
		freeBingAuthEndpoint = oldBingAuth
		freeBingTranslateEndpoint = oldBingTranslate
		freeGoogleTranslateURL = oldGoogle
	})
}
