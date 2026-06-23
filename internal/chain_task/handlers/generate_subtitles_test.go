package handlers

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestGenerateSubtitlesFailsWhenNoSubtitleSourceOrASRConfig(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SavedVideo{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	video := &model.SavedVideo{
		VideoID:   "no-subtitle-video",
		URL:       "https://www.youtube.com/watch?v=no-subtitle-video",
		Title:     "No subtitle video",
		Subtitles: "",
	}
	if err := db.Create(video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	cfg := types.NewDefaultConfig()

	task := &GenerateSubtitles{
		BaseTask: base.BaseTask{
			Name: "generate_subtitles",
			StateManager: &manager.StateManager{
				Id:         video.ID,
				VideoID:    video.VideoID,
				CurrentDir: t.TempDir(),
			},
		},
		App:               &core.AppServer{Config: cfg, Logger: zap.NewNop().Sugar()},
		SavedVideoService: services.NewSavedVideoService(db),
	}

	context := map[string]interface{}{}
	if ok := task.Execute(context); ok {
		t.Fatalf("Execute() = true, want false when neither DB subtitles nor ASR are available; context=%v", context)
	}
	if _, exists := context["error"]; !exists {
		t.Fatalf("expected context error when subtitles cannot be generated, context=%v", context)
	}
}

func TestRunASRCommandWithTimeoutStopsLongRunningProcess(t *testing.T) {
	if os.Getenv("YTB2BILI_ASR_TIMEOUT_HELPER") == "1" {
		time.Sleep(time.Second)
		os.Exit(0)
		return
	}

	command := []string{os.Args[0], "-test.run=TestRunASRCommandWithTimeoutStopsLongRunningProcess"}
	_, err := runASRCommandWithTimeout(
		command,
		20*time.Millisecond,
		[]string{"YTB2BILI_ASR_TIMEOUT_HELPER=1"},
	)
	if err == nil {
		t.Fatal("runASRCommandWithTimeout() error = nil, want timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("runASRCommandWithTimeout() error = %q, want timed out", err)
	}
}

func TestRunASRCommandWithTimeoutReturnsOutput(t *testing.T) {
	if os.Getenv("YTB2BILI_ASR_OUTPUT_HELPER") == "1" {
		_, _ = io.WriteString(os.Stdout, "12.345\n")
		os.Exit(0)
		return
	}

	command := []string{os.Args[0], "-test.run=TestRunASRCommandWithTimeoutReturnsOutput"}
	output, err := runASRCommandWithTimeout(
		command,
		time.Second,
		[]string{"YTB2BILI_ASR_OUTPUT_HELPER=1"},
	)
	if err != nil {
		t.Fatalf("runASRCommandWithTimeout() error = %v", err)
	}
	if strings.TrimSpace(string(output)) != "12.345" {
		t.Fatalf("output = %q, want 12.345", output)
	}
}

func TestFindSourceVideoIgnoresEmptyVideoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	empty := filepath.Join(tmpDir, "empty.mp4")
	if err := os.WriteFile(empty, nil, 0644); err != nil {
		t.Fatalf("write empty video: %v", err)
	}

	task := &GenerateSubtitles{
		BaseTask: base.BaseTask{
			StateManager: &manager.StateManager{
				VideoID:    "empty",
				CurrentDir: tmpDir,
			},
		},
	}

	if _, err := task.findSourceVideo(); err == nil {
		t.Fatal("findSourceVideo() error = nil, want failure for empty video file")
	}
}
