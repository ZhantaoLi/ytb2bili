package handlers

import (
	"encoding/json"
	"fmt"
	"io"
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

func TestGenerateMetadataFreeFallbackTranslatesOriginalMetadataToChinese(t *testing.T) {
	originalHTTPClient := freeTranslateHTTPClient
	originalBingAuthEndpoint := freeBingAuthEndpoint
	originalBingTranslateEndpoint := freeBingTranslateEndpoint
	originalGoogleTranslateURL := freeGoogleTranslateURL
	defer func() {
		freeTranslateHTTPClient = originalHTTPClient
		freeBingAuthEndpoint = originalBingAuthEndpoint
		freeBingTranslateEndpoint = originalBingTranslateEndpoint
		freeGoogleTranslateURL = originalGoogleTranslateURL
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			_, _ = w.Write([]byte("test-token"))
		case "/translate":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read translate request: %v", err)
			}
			var req []map[string]string
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("decode translate request: %v", err)
			}

			type translation struct {
				Text string `json:"text"`
			}
			type item struct {
				Translations []translation `json:"translations"`
			}

			resp := make([]item, 0, len(req))
			for _, entry := range req {
				resp = append(resp, item{Translations: []translation{{Text: fakeMetadataTranslation(entry["Text"])}}})
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("encode translate response: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	freeTranslateHTTPClient = server.Client()
	freeBingAuthEndpoint = server.URL + "/auth"
	freeBingTranslateEndpoint = server.URL + "/translate"
	freeGoogleTranslateURL = server.URL + "/google"

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SavedVideo{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	video := &model.SavedVideo{
		VideoID:     "foreign-metadata-video",
		Title:       "Sudden thunderstorm mountain camping with my dog",
		Description: "#camping #travel\nThis video supports multilingual subtitles and shows a quiet mountain camping trip.",
	}
	if err := db.Create(video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false
	cfg.GeminiConfig.Enabled = false
	cfg.OpenAICompatibleConfig.Enabled = false

	currentDir := t.TempDir()
	task := &GenerateMetadata{
		BaseTask: base.BaseTask{
			Name: "generate_metadata",
			StateManager: &manager.StateManager{
				VideoID:    video.VideoID,
				CurrentDir: currentDir,
			},
		},
		App:               &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
		SavedVideoService: services.NewSavedVideoService(db),
	}

	context := map[string]interface{}{}
	if ok := task.Execute(context); !ok {
		t.Fatalf("expected free metadata fallback to succeed, context=%v", context)
	}

	var fileMetadata VideoMetadata
	metaBytes, err := os.ReadFile(filepath.Join(currentDir, "meta.json"))
	if err != nil {
		t.Fatalf("read meta.json: %v", err)
	}
	if err := json.Unmarshal(metaBytes, &fileMetadata); err != nil {
		t.Fatalf("decode meta.json: %v", err)
	}

	if fileMetadata.Title != "山中雷雨木屋卡车露营" {
		t.Fatalf("meta title = %q, want translated Chinese title", fileMetadata.Title)
	}
	if !strings.Contains(fileMetadata.Description, "山中露营") {
		t.Fatalf("meta description = %q, want translated Chinese description", fileMetadata.Description)
	}
	if !containsString(fileMetadata.Tags, "露营") {
		t.Fatalf("meta tags = %#v, want translated camping tag", fileMetadata.Tags)
	}

	var updated model.SavedVideo
	if err := db.Where("video_id = ?", video.VideoID).First(&updated).Error; err != nil {
		t.Fatalf("load updated video: %v", err)
	}
	if updated.GeneratedTitle != fileMetadata.Title || updated.GeneratedDesc != fileMetadata.Description {
		t.Fatalf("db generated metadata not synchronized with meta.json: db title=%q desc=%q meta=%#v", updated.GeneratedTitle, updated.GeneratedDesc, fileMetadata)
	}
}

func TestGenerateMetadataFreeFallbackTranslatesJapaneseMetadataWithKanji(t *testing.T) {
	restoreTranslator := setupFakeFreeMetadataTranslator(t)
	defer restoreTranslator()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SavedVideo{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	video := &model.SavedVideo{
		VideoID:     "japanese-metadata-video",
		Title:       "山中キャンプと突然の雷雨",
		Description: "静かな山中キャンプの記録です。",
	}
	if err := db.Create(video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false
	cfg.GeminiConfig.Enabled = false
	cfg.OpenAICompatibleConfig.Enabled = false

	currentDir := t.TempDir()
	task := &GenerateMetadata{
		BaseTask: base.BaseTask{
			Name: "generate_metadata",
			StateManager: &manager.StateManager{
				VideoID:    video.VideoID,
				CurrentDir: currentDir,
			},
		},
		App:               &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
		SavedVideoService: services.NewSavedVideoService(db),
	}

	context := map[string]interface{}{}
	if ok := task.Execute(context); !ok {
		t.Fatalf("expected free metadata fallback to succeed, context=%v", context)
	}

	var fileMetadata VideoMetadata
	metaBytes, err := os.ReadFile(filepath.Join(currentDir, "meta.json"))
	if err != nil {
		t.Fatalf("read meta.json: %v", err)
	}
	if err := json.Unmarshal(metaBytes, &fileMetadata); err != nil {
		t.Fatalf("decode meta.json: %v", err)
	}

	if fileMetadata.Title != "山中雷雨露营" {
		t.Fatalf("meta title = %q, want translated Chinese title", fileMetadata.Title)
	}
	if strings.Contains(fileMetadata.Description, "キャンプ") || strings.Contains(fileMetadata.Description, "記録") {
		t.Fatalf("meta description still contains Japanese text: %q", fileMetadata.Description)
	}
}

func fakeMetadataTranslation(text string) string {
	switch {
	case strings.Contains(text, "Sudden thunderstorm"):
		return "山中雷雨木屋卡车露营"
	case strings.Contains(text, "This video supports"):
		return "本视频支持多语言字幕，记录一次安静的山中露营旅行。"
	case strings.Contains(text, "山中キャンプ"):
		return "山中雷雨露营"
	case strings.Contains(text, "静かな山中"):
		return "记录一次安静的山中露营。"
	case strings.EqualFold(strings.TrimSpace(text), "camping"):
		return "露营"
	case strings.EqualFold(strings.TrimSpace(text), "travel"):
		return "旅行"
	default:
		return fmt.Sprintf("中文%s", text)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestCleanMetadataDescriptionForTranslationDropsLeadingHashtagBlock(t *testing.T) {
	source := "#camping#travel#캠핑#rain\n\nThis video supports multilingual subtitles.\n# keep-this-topic"
	cleaned := cleanMetadataDescriptionForTranslation(source)

	if strings.Contains(cleaned, "#camping") || strings.Contains(cleaned, "#캠핑") {
		t.Fatalf("cleaned description still contains leading hashtag block: %q", cleaned)
	}
	if !strings.Contains(cleaned, "This video supports multilingual subtitles.") {
		t.Fatalf("cleaned description lost body text: %q", cleaned)
	}
	if !strings.Contains(cleaned, "# keep-this-topic") {
		t.Fatalf("cleaned description should only drop leading pure hashtag lines: %q", cleaned)
	}
}

func TestLocalizeMetadataTagsUsesKnownHashtagAliases(t *testing.T) {
	restoreTranslator := setupFakeFreeMetadataTranslator(t)
	defer restoreTranslator()

	task := &GenerateMetadata{
		BaseTask: base.BaseTask{
			StateManager: &manager.StateManager{VideoID: "tag-video"},
		},
		App: &core.AppServer{Config: types.NewDefaultConfig(), Logger: zap.NewNop().Sugar()},
	}
	translator := &TranslateSubtitle{App: task.App}
	metadata := &VideoMetadata{
		Description: "#camping#travel#painting#캠핑#여행#그림#우중캠핑#rain",
		Tags:        []string{"视频"},
	}

	tags := task.localizeMetadataTags(translator, metadata)
	want := []string{"露营", "旅行", "绘画", "雨中露营", "雨天"}
	if fmt.Sprint(tags) != fmt.Sprint(want) {
		t.Fatalf("tags = %#v, want %#v", tags, want)
	}
}

func setupFakeFreeMetadataTranslator(t *testing.T) func() {
	t.Helper()

	originalHTTPClient := freeTranslateHTTPClient
	originalBingAuthEndpoint := freeBingAuthEndpoint
	originalBingTranslateEndpoint := freeBingTranslateEndpoint
	originalGoogleTranslateURL := freeGoogleTranslateURL

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			_, _ = w.Write([]byte("test-token"))
		case "/translate":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read translate request: %v", err)
			}
			var req []map[string]string
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("decode translate request: %v", err)
			}

			type translation struct {
				Text string `json:"text"`
			}
			type item struct {
				Translations []translation `json:"translations"`
			}

			resp := make([]item, 0, len(req))
			for _, entry := range req {
				resp = append(resp, item{Translations: []translation{{Text: fakeMetadataTranslation(entry["Text"])}}})
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("encode translate response: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))

	freeTranslateHTTPClient = server.Client()
	freeBingAuthEndpoint = server.URL + "/auth"
	freeBingTranslateEndpoint = server.URL + "/translate"
	freeGoogleTranslateURL = server.URL + "/google"

	return func() {
		server.Close()
		freeTranslateHTTPClient = originalHTTPClient
		freeBingAuthEndpoint = originalBingAuthEndpoint
		freeBingTranslateEndpoint = originalBingTranslateEndpoint
		freeGoogleTranslateURL = originalGoogleTranslateURL
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

func TestGenerateMetadataLocalizesOpenAICompatibleEnglishResponseBeforeSaving(t *testing.T) {
	restoreTranslator := setupFakeFreeMetadataTranslator(t)
	defer restoreTranslator()

	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "{\"title\":\"Sudden thunderstorm mountain camping with my dog\",\"description\":\"This video supports multilingual subtitles and shows a quiet mountain camping trip.\",\"tags\":[\"camping\",\"travel\"]}"
				}
			}]
		}`))
	}))
	defer server.Close()

	currentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(currentDir, "zh.srt"), []byte("1\n00:00:00,000 --> 00:00:02,000\n山中露营\n\n"), 0644); err != nil {
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
				VideoID:    "english-openai-metadata-video",
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

	var fileMetadata VideoMetadata
	metaBytes, err := os.ReadFile(filepath.Join(currentDir, "meta.json"))
	if err != nil {
		t.Fatalf("read meta.json: %v", err)
	}
	if err := json.Unmarshal(metaBytes, &fileMetadata); err != nil {
		t.Fatalf("decode meta.json: %v", err)
	}

	if fileMetadata.Title != "山中雷雨木屋卡车露营" {
		t.Fatalf("meta title = %q, want translated Chinese title", fileMetadata.Title)
	}
	if !strings.Contains(fileMetadata.Description, "山中露营") {
		t.Fatalf("meta description = %q, want translated Chinese description", fileMetadata.Description)
	}
	if !containsString(fileMetadata.Tags, "露营") {
		t.Fatalf("meta tags = %#v, want translated Chinese tags", fileMetadata.Tags)
	}
}
