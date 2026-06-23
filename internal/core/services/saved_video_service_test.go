package services

import (
	"testing"

	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetPendingVideosIncludesURLOnlyTasksForASRFlow(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SavedVideo{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := db.Create(&model.SavedVideo{
		VideoID:   "url-only-video",
		URL:       "https://www.youtube.com/watch?v=url-only",
		Status:    "001",
		Subtitles: "",
	}).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	videos, err := NewSavedVideoService(db).GetPendingVideos(10)
	if err != nil {
		t.Fatalf("GetPendingVideos() error = %v", err)
	}
	if len(videos) != 1 {
		t.Fatalf("pending videos = %d, want 1", len(videos))
	}
	if videos[0].VideoID != "url-only-video" {
		t.Fatalf("pending video id = %q, want url-only-video", videos[0].VideoID)
	}
}

func TestUpdateStatusIfCurrentPreventsStaleTransitions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SavedVideo{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	video := &model.SavedVideo{
		VideoID: "race-video",
		Status:  "200",
	}
	if err := db.Create(video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	service := NewSavedVideoService(db)
	updated, err := service.UpdateStatusIfCurrent(video.ID, "200", "201")
	if err != nil {
		t.Fatalf("first UpdateStatusIfCurrent() error = %v", err)
	}
	if !updated {
		t.Fatal("first UpdateStatusIfCurrent() updated = false, want true")
	}

	updated, err = service.UpdateStatusIfCurrent(video.ID, "200", "201")
	if err != nil {
		t.Fatalf("second UpdateStatusIfCurrent() error = %v", err)
	}
	if updated {
		t.Fatal("second UpdateStatusIfCurrent() updated = true, want false for stale status")
	}

	var stored model.SavedVideo
	if err := db.First(&stored, video.ID).Error; err != nil {
		t.Fatalf("load stored video: %v", err)
	}
	if stored.Status != "201" {
		t.Fatalf("stored status = %q, want 201", stored.Status)
	}
}

func TestUpdateGeneratedMetadataPreservesExistingStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SavedVideo{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	video := &model.SavedVideo{
		VideoID:        "metadata-video",
		Status:         "200",
		GeneratedTitle: "old title",
		GeneratedDesc:  "old desc",
		GeneratedTags:  "old",
	}
	if err := db.Create(video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	if err := NewSavedVideoService(db).UpdateGeneratedMetadata(video.ID, "new title", "new desc", "tag1,tag2"); err != nil {
		t.Fatalf("UpdateGeneratedMetadata() error = %v", err)
	}

	var stored model.SavedVideo
	if err := db.First(&stored, video.ID).Error; err != nil {
		t.Fatalf("load stored video: %v", err)
	}
	if stored.Status != "200" {
		t.Fatalf("stored status = %q, want 200", stored.Status)
	}
	if stored.GeneratedTitle != "new title" || stored.GeneratedDesc != "new desc" || stored.GeneratedTags != "tag1,tag2" {
		t.Fatalf("metadata not updated correctly: title=%q desc=%q tags=%q", stored.GeneratedTitle, stored.GeneratedDesc, stored.GeneratedTags)
	}
}
