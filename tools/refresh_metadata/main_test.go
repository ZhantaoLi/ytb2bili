package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/handlers"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/services"
	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRefreshMetadataUpdatesOriginalAndGeneratedMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SavedVideo{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	video := &model.SavedVideo{
		VideoID:        "YObDxucFg4s",
		URL:            "https://www.youtube.com/watch?v=YObDxucFg4s",
		Status:         "200",
		GeneratedTitle: "YObDxucFg4s",
		GeneratedDesc:  "auto upload",
		GeneratedTags:  "video",
	}
	if err := db.Create(video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	cfg := types.NewDefaultConfig()
	cfg.DeepSeekTransConfig.Enabled = false
	cfg.GeminiConfig.Enabled = false
	cfg.OpenAICompatibleConfig.Enabled = false

	currentDir := t.TempDir()
	stateManager := &manager.StateManager{
		VideoID:    video.VideoID,
		CurrentDir: currentDir,
	}
	app := &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()}
	savedVideoService := services.NewSavedVideoService(db)

	summary, err := refreshMetadata(app, savedVideoService, video, stateManager, func() (*handlers.VideoMetadataInfo, error) {
		return &handlers.VideoMetadataInfo{
			Title:       "GLM 5.2 vs Claude Was Crazy",
			Description: "Reference YouTube description",
		}, nil
	})
	if err != nil {
		t.Fatalf("refreshMetadata() error = %v", err)
	}

	if summary.VideoID != video.VideoID {
		t.Fatalf("summary video id = %q, want %q", summary.VideoID, video.VideoID)
	}

	var stored model.SavedVideo
	if err := db.First(&stored, video.ID).Error; err != nil {
		t.Fatalf("load stored video: %v", err)
	}
	if stored.Title != "GLM 5.2 vs Claude Was Crazy" {
		t.Fatalf("title = %q, want refreshed original title", stored.Title)
	}
	if stored.Description != "Reference YouTube description" {
		t.Fatalf("description = %q, want refreshed original description", stored.Description)
	}
	if stored.GeneratedTitle != stored.Title {
		t.Fatalf("generated title = %q, want %q", stored.GeneratedTitle, stored.Title)
	}
	if stored.GeneratedDesc != stored.Description {
		t.Fatalf("generated desc = %q, want %q", stored.GeneratedDesc, stored.Description)
	}
	if _, err := os.Stat(filepath.Join(currentDir, "meta.json")); err != nil {
		t.Fatalf("meta.json should be written: %v", err)
	}
}
