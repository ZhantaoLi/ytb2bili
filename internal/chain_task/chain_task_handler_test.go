package chain_task

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/services"
	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExecutableRetryStepsSkipsUploadWhenAutoUploadDisabled(t *testing.T) {
	steps := []*model.TaskStep{
		{VideoID: "video-1", StepName: "上传到Bilibili"},
		{VideoID: "video-1", StepName: "上传字幕到Bilibili"},
	}

	got := executableRetrySteps(steps, false)
	if len(got) != 0 {
		t.Fatalf("executable retry steps = %d, want 0", len(got))
	}
}

func TestExecutableRetryStepsKeepsNonUploadAndEnabledUpload(t *testing.T) {
	steps := []*model.TaskStep{
		{VideoID: "video-1", StepName: "生成字幕"},
		{VideoID: "video-2", StepName: "上传到Bilibili"},
	}

	got := executableRetrySteps(steps, true)
	if len(got) != 2 {
		t.Fatalf("executable retry steps = %d, want 2", len(got))
	}

	got = executableRetrySteps(steps, false)
	if len(got) != 1 || got[0].StepName != "生成字幕" {
		t.Fatalf("executable retry steps with upload disabled = %#v, want only subtitle retry", got)
	}
}

func TestSubtitleBranchStepToSkip(t *testing.T) {
	if got := subtitleBranchStepToSkip(true); got != "生成字幕" {
		t.Fatalf("subtitleBranchStepToSkip(true) = %q, want 生成字幕", got)
	}
	if got := subtitleBranchStepToSkip(false); got != "B站必剪转录" {
		t.Fatalf("subtitleBranchStepToSkip(false) = %q, want B站必剪转录", got)
	}
}

func TestRunSingleTaskStepUploadSubtitleDoesNotSucceedWithEmptyChain(t *testing.T) {
	db := newChainTaskTestDB(t)
	video := model.SavedVideo{
		BaseModel: model.BaseModel{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		VideoID: "video-without-bvid",
		URL:     "https://www.youtube.com/watch?v=video-without-bvid",
		Status:  "300",
	}
	if err := db.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	const stepName = "上传字幕到Bilibili"
	if err := db.Create(&model.TaskStep{
		VideoID:   video.VideoID,
		StepName:  stepName,
		StepOrder: 9,
		Status:    model.TaskStepStatusPending,
		CanRetry:  true,
	}).Error; err != nil {
		t.Fatalf("create task step: %v", err)
	}

	handler := &ChainTaskHandler{
		App: &core.AppServer{
			Config: &types.AppConfig{FileUpDir: filepath.Join(t.TempDir(), "data")},
			Logger: zap.NewNop().Sugar(),
		},
		SavedVideoService: services.NewSavedVideoService(db),
		TaskStepService:   services.NewTaskStepService(db),
		Db:                db,
	}

	err := handler.RunSingleTaskStep(video.VideoID, stepName)
	if err == nil {
		t.Fatal("RunSingleTaskStep() error = nil, want failure when subtitle upload has no BVID")
	}

	var step model.TaskStep
	if err := db.Where("video_id = ? AND step_name = ?", video.VideoID, stepName).First(&step).Error; err != nil {
		t.Fatalf("load task step: %v", err)
	}
	if step.Status != model.TaskStepStatusFailed {
		t.Fatalf("step status = %q, want failed", step.Status)
	}
}

func TestRunSingleTaskStepSkipsInactiveSubtitleBranch(t *testing.T) {
	db := newChainTaskTestDB(t)
	video := model.SavedVideo{
		BaseModel: model.BaseModel{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		VideoID: "video-bcut",
		URL:     "https://www.youtube.com/watch?v=video-bcut",
		Status:  "999",
	}
	if err := db.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	const stepName = "生成字幕"
	if err := db.Create(&model.TaskStep{
		VideoID:   video.VideoID,
		StepName:  stepName,
		StepOrder: 4,
		Status:    model.TaskStepStatusFailed,
		ErrorMsg:  "old mimo failure",
		CanRetry:  true,
	}).Error; err != nil {
		t.Fatalf("create task step: %v", err)
	}

	handler := &ChainTaskHandler{
		App: &core.AppServer{
			Config: &types.AppConfig{
				FileUpDir: filepath.Join(t.TempDir(), "data"),
				WhisperConfig: &types.WhisperConfig{
					Enabled: true,
				},
			},
			Logger: zap.NewNop().Sugar(),
		},
		SavedVideoService: services.NewSavedVideoService(db),
		TaskStepService:   services.NewTaskStepService(db),
		Db:                db,
	}

	if err := handler.RunSingleTaskStep(video.VideoID, stepName); err != nil {
		t.Fatalf("RunSingleTaskStep() error = %v", err)
	}

	var step model.TaskStep
	if err := db.Where("video_id = ? AND step_name = ?", video.VideoID, stepName).First(&step).Error; err != nil {
		t.Fatalf("load task step: %v", err)
	}
	if step.Status != model.TaskStepStatusSkipped {
		t.Fatalf("step status = %q, want skipped", step.Status)
	}
	if step.ErrorMsg != "" {
		t.Fatalf("step error = %q, want cleared", step.ErrorMsg)
	}
	if !strings.Contains(step.ResultData, "subtitle branch not selected") {
		t.Fatalf("result_data = %q, want skip reason", step.ResultData)
	}
}

func TestBuildSingleTaskStepRecognizesPreparationRetrySteps(t *testing.T) {
	db := newChainTaskTestDB(t)
	stateManager := manager.NewStateManager(1, "video-1", t.TempDir(), time.Now())
	handler := &ChainTaskHandler{
		App: &core.AppServer{
			Config: &types.AppConfig{FileUpDir: filepath.Join(t.TempDir(), "data")},
			Logger: zap.NewNop().Sugar(),
		},
		SavedVideoService: services.NewSavedVideoService(db),
		TaskStepService:   services.NewTaskStepService(db),
		Db:                db,
	}

	tests := []struct {
		stepName string
		wantName string
	}{
		{stepName: "下载封面", wantName: "下载封面"},
		{stepName: "翻译字幕", wantName: "翻译字幕"},
		{stepName: "生成视频元数据", wantName: "生成视频元数据"},
		{stepName: "生成元数据", wantName: "生成视频元数据"},
	}

	for _, tt := range tests {
		t.Run(tt.stepName, func(t *testing.T) {
			task, err := handler.buildSingleTaskStep(tt.stepName, stateManager)
			if err != nil {
				t.Fatalf("buildSingleTaskStep(%q) error = %v", tt.stepName, err)
			}
			if task.GetName() != tt.wantName {
				t.Fatalf("task name = %q, want %q", task.GetName(), tt.wantName)
			}
		})
	}
}

func TestRunTaskChainTracksDownloadCoverStep(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "chain_task_handler.go", nil, 0)
	if err != nil {
		t.Fatalf("parse chain_task_handler.go: %v", err)
	}

	var foundDownloadCover bool
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isSelectorCall(call, "AddTask") || len(call.Args) != 1 {
			return true
		}

		if containsDownloadImgHandler(call.Args[0]) {
			foundDownloadCover = true
			if !isSelectorCallExpr(call.Args[0], "wrapTaskWithStepTracking") {
				t.Fatalf("下载封面任务必须经过 wrapTaskWithStepTracking，否则成功下载后步骤仍会停留在 pending")
			}
		}
		return true
	})

	if !foundDownloadCover {
		t.Fatal("RunTaskChain 未装配下载封面任务")
	}
}

func newChainTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SavedVideo{}, &model.TaskStep{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func containsDownloadImgHandler(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && isSelectorCall(call, "NewDownloadImgHandler") {
			found = true
			return false
		}
		return true
	})
	return found
}

func isSelectorCallExpr(expr ast.Expr, name string) bool {
	call, ok := expr.(*ast.CallExpr)
	return ok && isSelectorCall(call, name)
}

func isSelectorCall(call *ast.CallExpr, name string) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == name
}
