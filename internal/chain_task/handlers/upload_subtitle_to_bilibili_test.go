package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/base"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/services"
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestFindSubtitleFilesPrefersTranslatedChineseSRT(t *testing.T) {
	tmpDir := t.TempDir()
	zhPath := filepath.Join(tmpDir, "zh.srt")
	if err := os.WriteFile(zhPath, []byte("1\n00:00:00,000 --> 00:00:01,000\nhello\n\n"), 0644); err != nil {
		t.Fatalf("write zh.srt: %v", err)
	}

	task := &UploadSubtitleToBilibili{
		BaseTask: base.BaseTask{
			StateManager: &manager.StateManager{CurrentDir: tmpDir},
		},
		App: &core.AppServer{Logger: zap.NewNop().Sugar()},
	}

	files := task.findSubtitleFiles()
	if len(files) == 0 {
		t.Fatal("findSubtitleFiles() returned no files, want zh.srt")
	}
	if filepath.Base(files[0].Path) != "zh.srt" {
		t.Fatalf("first subtitle = %s, want zh.srt", filepath.Base(files[0].Path))
	}
	if files[0].Language != "zh-Hans" {
		t.Fatalf("language = %q, want zh-Hans", files[0].Language)
	}
}

func TestExtractAudioFailsWhenSourceVideoIsMissing(t *testing.T) {
	tmpDir := t.TempDir()
	task := NewExtractAudio("extract", &core.AppServer{}, &manager.StateManager{
		InputVideoPath: filepath.Join(tmpDir, "missing.mp4"),
		OriginalMP3:    filepath.Join(tmpDir, "out.mp3"),
	}, nil)

	context := map[string]interface{}{}
	if task.Execute(context) {
		t.Fatal("ExtractAudio.Execute() = true, want false for missing source video")
	}
	if context["error"] == nil {
		t.Fatal("context[error] missing for missing source video")
	}
}

func TestUploadSubtitleFailsWhenBVIDIsMissing(t *testing.T) {
	db := newUploadSubtitleTestDB(t)
	if err := db.Create(&model.SavedVideo{
		VideoID: "video-without-bvid",
		URL:     "https://www.youtube.com/watch?v=video-without-bvid",
		Status:  "300",
	}).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	task := NewUploadSubtitleToBilibili(
		"upload-subtitle",
		&core.AppServer{Logger: zap.NewNop().Sugar()},
		&manager.StateManager{VideoID: "video-without-bvid", CurrentDir: t.TempDir()},
		nil,
		services.NewSavedVideoService(db),
	)

	context := map[string]interface{}{}
	if task.Execute(context) {
		t.Fatal("UploadSubtitleToBilibili.Execute() = true, want false when BVID is missing")
	}
	assertContextErrorContains(t, context, "BVID")
}

func TestUploadSubtitleFailsWhenSubtitleFilesAreMissing(t *testing.T) {
	db := newUploadSubtitleTestDB(t)
	if err := db.Create(&model.SavedVideo{
		VideoID:  "video-without-subtitles",
		URL:      "https://www.youtube.com/watch?v=video-without-subtitles",
		Status:   "300",
		BiliBVID: "BV1missingSubtitle",
	}).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	task := NewUploadSubtitleToBilibili(
		"upload-subtitle",
		&core.AppServer{Logger: zap.NewNop().Sugar()},
		&manager.StateManager{VideoID: "video-without-subtitles", CurrentDir: t.TempDir()},
		nil,
		services.NewSavedVideoService(db),
	)

	context := map[string]interface{}{}
	if task.Execute(context) {
		t.Fatal("UploadSubtitleToBilibili.Execute() = true, want false when subtitle files are missing")
	}
	assertContextErrorContains(t, context, "字幕文件")
}

func newUploadSubtitleTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SavedVideo{}); err != nil {
		t.Fatalf("migrate saved videos: %v", err)
	}
	return db
}

func assertContextErrorContains(t *testing.T, context map[string]interface{}, want string) {
	t.Helper()

	got, ok := context["error"].(string)
	if !ok || got == "" {
		t.Fatalf("context[error] = %#v, want non-empty string containing %q", context["error"], want)
	}
	if !strings.Contains(got, want) {
		t.Fatalf("context[error] = %q, want substring %q", got, want)
	}
}
