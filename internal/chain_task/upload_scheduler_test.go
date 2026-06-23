package chain_task

import (
	"testing"
	"time"

	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

func TestUploadSchedulerRegistersCronEntryWithSecondsParser(t *testing.T) {
	task := cron.New(cron.WithSeconds())
	app := &core.AppServer{
		Config: &types.AppConfig{AutoUpload: false},
		Logger: zap.NewNop().Sugar(),
	}

	scheduler := NewUploadScheduler(app, task, nil, nil, nil)
	scheduler.SetUp()

	entries := task.Entries()
	if got := len(entries); got != 1 {
		t.Fatalf("registered upload scheduler cron entries = %d, want 1", got)
	}

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if got, want := entries[0].Schedule.Next(now), now.Add(5*time.Minute); !got.Equal(want) {
		t.Fatalf("next upload scheduler run = %s, want %s", got, want)
	}
}
