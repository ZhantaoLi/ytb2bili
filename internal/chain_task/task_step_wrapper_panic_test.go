package chain_task

import (
	"strings"
	"testing"

	"github.com/ZhantaoLi/ytb2bili/internal/core/services"
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type panicStepTask struct{}

func (panicStepTask) GetName() string {
	return "panic-step"
}

func (panicStepTask) InsertTask() error {
	return nil
}

func (panicStepTask) UpdateStatus(string, string) error {
	return nil
}

func (panicStepTask) Execute(map[string]interface{}) bool {
	panic("panic from task")
}

func TestTaskStepWrapperMarksStepFailedWhenTaskPanics(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.TaskStep{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := db.Create(&model.TaskStep{
		VideoID:   "video-panic",
		StepName:  "panic-step",
		StepOrder: 1,
		Status:    model.TaskStepStatusPending,
		CanRetry:  true,
	}).Error; err != nil {
		t.Fatalf("create task step: %v", err)
	}

	wrapper := &TaskStepWrapper{
		task:            panicStepTask{},
		videoID:         "video-panic",
		taskStepService: services.NewTaskStepService(db),
		logger:          zap.NewNop().Sugar(),
	}

	if got := wrapper.Execute(map[string]interface{}{}); got {
		t.Fatal("Execute() = true, want false after panic")
	}

	var step model.TaskStep
	if err := db.Where("video_id = ? AND step_name = ?", "video-panic", "panic-step").First(&step).Error; err != nil {
		t.Fatalf("query step: %v", err)
	}
	if step.Status != model.TaskStepStatusFailed {
		t.Fatalf("step status = %q, want %q", step.Status, model.TaskStepStatusFailed)
	}
	if !strings.Contains(step.ErrorMsg, "panic from task") {
		t.Fatalf("step error = %q, want panic detail", step.ErrorMsg)
	}
}
