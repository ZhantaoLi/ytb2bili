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
	"github.com/ZhantaoLi/ytb2bili/internal/core/services"
	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGenerateMetadataUsesFreeFallbackWhenCloudMetadataDisabled(t *testing.T) {
	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false
	cfg.GeminiConfig.Enabled = false

	task := &GenerateMetadata{
		BaseTask: base.BaseTask{
			Name: "generate_metadata",
			StateManager: &manager.StateManager{
				VideoID:    "free-mode-video",
				CurrentDir: t.TempDir(),
			},
		},
		App: &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
	}

	context := map[string]interface{}{}

	if ok := task.Execute(context); !ok {
		t.Fatalf("expected free metadata fallback to succeed, context=%v", context)
	}

	if got := context["video_title"]; got != "free-mode-video" {
		t.Fatalf("expected default title from video id, got %v", got)
	}

	if got := context["metadata_skipped"]; got != "cloud_metadata_disabled" {
		t.Fatalf("expected cloud metadata skip marker, got %v", got)
	}

	if _, exists := context["error"]; exists {
		t.Fatalf("expected no error in free metadata mode, context=%v", context)
	}

	if _, err := os.Stat(filepath.Join(task.StateManager.CurrentDir, "meta.json")); err != nil {
		t.Fatalf("meta.json should be written in free metadata mode: %v", err)
	}
}

func TestGenerateMetadataFreeFallbackPersistsOriginalMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SavedVideo{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	video := &model.SavedVideo{
		VideoID:     "original-video",
		URL:         "https://www.youtube.com/watch?v=original-video",
		Title:       "原始标题",
		Description: "原始简介",
	}
	if err := db.Create(video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false
	cfg.GeminiConfig.Enabled = false
	cfg.OpenAICompatibleConfig.Enabled = false

	task := &GenerateMetadata{
		BaseTask: base.BaseTask{
			Name: "generate_metadata",
			StateManager: &manager.StateManager{
				VideoID:    video.VideoID,
				CurrentDir: t.TempDir(),
			},
		},
		App:               &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
		SavedVideoService: services.NewSavedVideoService(db),
	}

	context := map[string]interface{}{}
	if ok := task.Execute(context); !ok {
		t.Fatalf("expected free metadata fallback to succeed, context=%v", context)
	}

	var updated model.SavedVideo
	if err := db.Where("video_id = ?", video.VideoID).First(&updated).Error; err != nil {
		t.Fatalf("load updated video: %v", err)
	}
	if updated.GeneratedTitle != video.Title {
		t.Fatalf("generated title = %q, want %q", updated.GeneratedTitle, video.Title)
	}
	if updated.GeneratedDesc != video.Description {
		t.Fatalf("generated desc = %q, want %q", updated.GeneratedDesc, video.Description)
	}
}

func TestGenerateMetadataUsesOpenAICompatibleWhenConfigured(t *testing.T) {
	var requestedPath string
	var requestedModel string
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		authHeader = r.Header.Get("Authorization")
		var req OpenAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requestedModel = req.Model

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "{\"title\":\"测试标题\",\"description\":\"测试简介\",\"tags\":[\"ASMR\",\"韩语\"]}"
				}
			}],
			"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`))
	}))
	defer server.Close()

	currentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(currentDir, "zh.srt"), []byte("1\n00:00:00,000 --> 00:00:02,000\n测试字幕\n\n"), 0644); err != nil {
		t.Fatalf("write srt: %v", err)
	}

	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false
	cfg.GeminiConfig.Enabled = false
	cfg.OpenAICompatibleConfig.Enabled = true
	cfg.OpenAICompatibleConfig.APIKey = "test-key"
	cfg.OpenAICompatibleConfig.BaseURL = server.URL + "/v1"
	cfg.OpenAICompatibleConfig.Model = "gemini-3.5-flash"
	cfg.OpenAICompatibleConfig.MaxTokens = 256

	task := &GenerateMetadata{
		BaseTask: base.BaseTask{
			Name: "generate_metadata",
			StateManager: &manager.StateManager{
				VideoID:    "openai-compatible-video",
				CurrentDir: currentDir,
			},
		},
		App: &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
	}

	context := map[string]interface{}{}
	if ok := task.Execute(context); !ok {
		t.Fatalf("expected OpenAI-compatible metadata generation to succeed, context=%v", context)
	}

	if requestedPath != "/v1/chat/completions" {
		t.Fatalf("request path = %q, want /v1/chat/completions", requestedPath)
	}
	if requestedModel != "gemini-3.5-flash" {
		t.Fatalf("model = %q, want gemini-3.5-flash", requestedModel)
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		t.Fatalf("authorization header not set")
	}
	if got := context["video_title"]; got != "测试标题" {
		t.Fatalf("video_title = %v, want 测试标题", got)
	}
	if _, exists := context["metadata_skipped"]; exists {
		t.Fatalf("metadata should not be skipped, context=%v", context)
	}
	if _, err := os.Stat(filepath.Join(currentDir, "meta.json")); err != nil {
		t.Fatalf("meta.json should be written: %v", err)
	}
}
