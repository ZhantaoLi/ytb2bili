package handlers

import (
	"fmt"
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
	"github.com/difyz9/bilibili-go-sdk/bilibili"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	if os.Getenv("YTB2BILI_FAKE_YTDLP_METADATA") == "1" {
		fmt.Println("WARNING: Your yt-dlp version is older than 90 days!")
		fmt.Println(`{"title":"GLM 5.2 vs Claude Was Crazy","description":"reference metadata","uploader":"creator","duration":943}`)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestFetchAndSaveMetadataParsesYtDlpWarnings(t *testing.T) {
	tmpDir := t.TempDir()
	fakeYtDlp := filepath.Join(tmpDir, "yt-dlp.exe")
	testBinary, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatalf("read test binary: %v", err)
	}
	if err := os.WriteFile(fakeYtDlp, testBinary, 0755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}
	t.Setenv("YTB2BILI_FAKE_YTDLP_METADATA", "1")

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SavedVideo{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	video := &model.SavedVideo{
		VideoID: "warn-video",
		URL:     "https://www.youtube.com/watch?v=warn-video",
		Status:  "200",
	}
	if err := db.Create(video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	cfg := types.NewDefaultConfig()
	cfg.YtDlpPath = tmpDir
	cfg.Path = filepath.Join(tmpDir, "config.toml")
	task := &UploadToBilibili{
		BaseTask: base.BaseTask{
			StateManager: &manager.StateManager{VideoID: video.VideoID},
		},
		App:               &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
		SavedVideoService: services.NewSavedVideoService(db),
	}

	if err := task.fetchAndSaveMetadata(video.VideoID); err != nil {
		t.Fatalf("fetchAndSaveMetadata() error = %v", err)
	}

	var stored model.SavedVideo
	if err := db.First(&stored, video.ID).Error; err != nil {
		t.Fatalf("load stored video: %v", err)
	}
	if stored.Title != "GLM 5.2 vs Claude Was Crazy" {
		t.Fatalf("title = %q, want parsed yt-dlp title", stored.Title)
	}
	if stored.Description != "reference metadata" {
		t.Fatalf("description = %q, want parsed yt-dlp description", stored.Description)
	}
}

func TestBuildStudioInfoPrefersGeneratedChineseMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SavedVideo{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	video := &model.SavedVideo{
		VideoID:        "generated-meta-video",
		Title:          "Sudden thunderstorm mountain camping",
		Description:    "Foreign original description",
		GeneratedTitle: "山中雷雨木屋卡车露营",
		GeneratedDesc:  "本视频记录一次山中露营旅行。",
		GeneratedTags:  "露营,旅行,ASMR",
		Status:         "200",
	}
	if err := db.Create(video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	cfg := types.NewDefaultConfig()
	cfg.BilibiliConfig.UseOriginalTitle = true
	cfg.BilibiliConfig.UseOriginalDesc = true

	task := &UploadToBilibili{
		BaseTask: base.BaseTask{
			StateManager: &manager.StateManager{
				VideoID:    video.VideoID,
				CurrentDir: t.TempDir(),
			},
		},
		App:               &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
		SavedVideoService: services.NewSavedVideoService(db),
	}

	studio := task.buildStudioInfo(&bilibili.Video{Title: "part-1"}, "", map[string]interface{}{})
	if studio.Title != video.GeneratedTitle {
		t.Fatalf("studio title = %q, want generated Chinese title %q", studio.Title, video.GeneratedTitle)
	}
	if !strings.Contains(studio.Desc, video.GeneratedDesc) {
		t.Fatalf("studio desc = %q, want generated Chinese description", studio.Desc)
	}
	if strings.Contains(studio.Desc, video.Description) {
		t.Fatalf("studio desc should not include untranslated original description: %q", studio.Desc)
	}
	if studio.Tag != video.GeneratedTags {
		t.Fatalf("studio tags = %q, want %q", studio.Tag, video.GeneratedTags)
	}
}

func TestResolveCoverImagePathFallsBackToDownloadedCoverFile(t *testing.T) {
	tmpDir := t.TempDir()
	maxCover := filepath.Join(tmpDir, "maxresdefault.jpg")
	if err := os.WriteFile(maxCover, []byte("cover"), 0644); err != nil {
		t.Fatalf("write max cover: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "sddefault.jpg"), []byte("cover"), 0644); err != nil {
		t.Fatalf("write standard cover: %v", err)
	}

	task := &UploadToBilibili{
		BaseTask: base.BaseTask{
			StateManager: &manager.StateManager{CurrentDir: tmpDir},
		},
	}

	got := task.resolveCoverImagePath(map[string]interface{}{})
	if got != maxCover {
		t.Fatalf("resolveCoverImagePath() = %q, want %q", got, maxCover)
	}
}

func TestResolveCoverImagePathKeepsExistingContextCover(t *testing.T) {
	tmpDir := t.TempDir()
	contextCover := filepath.Join(tmpDir, "context-cover.jpg")
	if err := os.WriteFile(contextCover, []byte("cover"), 0644); err != nil {
		t.Fatalf("write context cover: %v", err)
	}
	localCover := filepath.Join(tmpDir, "maxresdefault.jpg")
	if err := os.WriteFile(localCover, []byte("cover"), 0644); err != nil {
		t.Fatalf("write local cover: %v", err)
	}

	task := &UploadToBilibili{
		BaseTask: base.BaseTask{
			StateManager: &manager.StateManager{CurrentDir: tmpDir},
		},
	}

	got := task.resolveCoverImagePath(map[string]interface{}{"cover_image_path": contextCover})
	if got != contextCover {
		t.Fatalf("resolveCoverImagePath() = %q, want context cover %q", got, contextCover)
	}
}

func TestFindLocalCoverImageIgnoresEmptyFiles(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "maxresdefault.jpg"), nil, 0644); err != nil {
		t.Fatalf("write empty cover: %v", err)
	}

	task := &UploadToBilibili{
		BaseTask: base.BaseTask{
			StateManager: &manager.StateManager{CurrentDir: tmpDir},
		},
		App: nil,
	}

	if got := task.findLocalCoverImage(); got != "" {
		t.Fatalf("findLocalCoverImage() = %q, want empty for zero-byte cover", got)
	}
}

func TestFindLocalCoverImageHandlesMissingDirectory(t *testing.T) {
	task := &UploadToBilibili{
		BaseTask: base.BaseTask{
			StateManager: &manager.StateManager{CurrentDir: filepath.Join(t.TempDir(), "missing")},
		},
	}

	if got := task.findLocalCoverImage(); got != "" {
		t.Fatalf("findLocalCoverImage() = %q, want empty for missing directory", got)
	}
}
