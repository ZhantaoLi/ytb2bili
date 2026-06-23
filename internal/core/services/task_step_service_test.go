package services

import (
	"strings"
	"testing"
	"time"

	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestResetAllRunningTasksUsesLowercaseStepStatuses(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.TaskStep{}, &model.SavedVideo{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := db.Create(&model.SavedVideo{
		VideoID: "video-1",
		URL:     "https://www.youtube.com/watch?v=abcdefghijk",
		Status:  "002",
	}).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	if err := db.Create(&model.TaskStep{
		VideoID:   "video-1",
		StepName:  "download",
		StepOrder: 1,
		Status:    model.TaskStepStatusRunning,
		CanRetry:  true,
	}).Error; err != nil {
		t.Fatalf("create task step: %v", err)
	}

	service := NewTaskStepService(db)
	if err := service.ResetAllRunningTasks(); err != nil {
		t.Fatalf("ResetAllRunningTasks() error = %v", err)
	}

	var step model.TaskStep
	if err := db.Where("video_id = ?", "video-1").First(&step).Error; err != nil {
		t.Fatalf("query step: %v", err)
	}
	if step.Status != model.TaskStepStatusPending {
		t.Fatalf("step status = %q, want %q", step.Status, model.TaskStepStatusPending)
	}

	var video model.SavedVideo
	if err := db.Where("video_id = ?", "video-1").First(&video).Error; err != nil {
		t.Fatalf("query video: %v", err)
	}
	if video.Status != "001" {
		t.Fatalf("video status = %q, want 001", video.Status)
	}
}

func TestGetPendingStepsIgnoresFreshlyInitializedSteps(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	if err := db.Create(&model.SavedVideo{
		VideoID: "video-fresh",
		URL:     "https://www.youtube.com/watch?v=abcdefghijk",
		Status:  "001",
	}).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	if err := service.InitTaskSteps("video-fresh"); err != nil {
		t.Fatalf("InitTaskSteps() error = %v", err)
	}

	steps, err := service.GetPendingSteps()
	if err != nil {
		t.Fatalf("GetPendingSteps() error = %v", err)
	}
	if len(steps) != 0 {
		t.Fatalf("fresh initialized pending steps = %d, want 0", len(steps))
	}
}

func TestInitTaskStepsIncludesPreparationChainStepNames(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	if err := service.InitTaskSteps("video-chain"); err != nil {
		t.Fatalf("InitTaskSteps() error = %v", err)
	}

	for _, stepName := range []string{"下载视频", "分离音频", "下载封面", "翻译字幕", "生成视频元数据"} {
		if _, err := service.GetTaskStepByName("video-chain", stepName); err != nil {
			t.Fatalf("missing initialized step %q: %v", stepName, err)
		}
	}
}

func TestInitTaskStepsBackfillsMissingStepsForExistingVideo(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	if err := db.Create(&model.TaskStep{
		VideoID:   "video-existing",
		StepName:  "下载视频",
		StepOrder: 1,
		Status:    model.TaskStepStatusCompleted,
		CanRetry:  true,
	}).Error; err != nil {
		t.Fatalf("create existing step: %v", err)
	}

	if err := service.InitTaskSteps("video-existing"); err != nil {
		t.Fatalf("InitTaskSteps() error = %v", err)
	}

	if _, err := service.GetTaskStepByName("video-existing", "生成视频元数据"); err != nil {
		t.Fatalf("missing backfilled metadata step: %v", err)
	}

	var step model.TaskStep
	if err := db.Where("video_id = ? AND step_name = ?", "video-existing", "下载视频").First(&step).Error; err != nil {
		t.Fatalf("query existing step: %v", err)
	}
	if step.Status != model.TaskStepStatusCompleted {
		t.Fatalf("existing step status = %q, want completed", step.Status)
	}
}

func TestInitTaskStepsRenamesLegacyMetadataStepWhenCanonicalMissing(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	if err := db.Create(&model.TaskStep{
		VideoID:   "video-legacy-only",
		StepName:  "生成元数据",
		StepOrder: 4,
		Status:    model.TaskStepStatusCompleted,
		CanRetry:  true,
	}).Error; err != nil {
		t.Fatalf("create legacy step: %v", err)
	}

	if err := service.InitTaskSteps("video-legacy-only"); err != nil {
		t.Fatalf("InitTaskSteps() error = %v", err)
	}

	if _, err := service.GetTaskStepByName("video-legacy-only", "生成视频元数据"); err != nil {
		t.Fatalf("canonical metadata step missing: %v", err)
	}

	var count int64
	if err := db.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ?", "video-legacy-only", "生成元数据").
		Count(&count).Error; err != nil {
		t.Fatalf("count legacy steps: %v", err)
	}
	if count != 0 {
		t.Fatalf("legacy metadata steps = %d, want 0 after rename", count)
	}
}

func TestGetTaskStepsAndProgressHideSupersededLegacyMetadataStep(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	for _, step := range []model.TaskStep{
		{
			VideoID:   "video-duplicate-metadata",
			StepName:  "下载视频",
			StepOrder: 1,
			Status:    model.TaskStepStatusCompleted,
			CanRetry:  true,
		},
		{
			VideoID:   "video-duplicate-metadata",
			StepName:  "生成元数据",
			StepOrder: 4,
			Status:    model.TaskStepStatusPending,
			CanRetry:  true,
		},
		{
			VideoID:   "video-duplicate-metadata",
			StepName:  "生成视频元数据",
			StepOrder: 7,
			Status:    model.TaskStepStatusCompleted,
			CanRetry:  true,
		},
	} {
		if err := db.Create(&step).Error; err != nil {
			t.Fatalf("create step %s: %v", step.StepName, err)
		}
	}

	steps, err := service.GetTaskStepsByVideoID("video-duplicate-metadata")
	if err != nil {
		t.Fatalf("GetTaskStepsByVideoID() error = %v", err)
	}
	for _, step := range steps {
		if step.StepName == "生成元数据" {
			t.Fatalf("legacy metadata step was returned: %#v", steps)
		}
	}
	if len(steps) != 2 {
		t.Fatalf("visible steps = %d, want 2", len(steps))
	}

	progress, err := service.GetTaskProgress("video-duplicate-metadata")
	if err != nil {
		t.Fatalf("GetTaskProgress() error = %v", err)
	}
	if progress["total_steps"] != 2 {
		t.Fatalf("total_steps = %v, want 2", progress["total_steps"])
	}
	if progress["completed_steps"] != 2 {
		t.Fatalf("completed_steps = %v, want 2", progress["completed_steps"])
	}
	if progress["progress_percent"] != 100 {
		t.Fatalf("progress_percent = %v, want 100", progress["progress_percent"])
	}
}

func TestGetTaskStepsByVideoIDRenamesLegacyMetadataStepWithoutInit(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	if err := db.Create(&model.TaskStep{
		VideoID:   "video-legacy-read",
		StepName:  "生成元数据",
		StepOrder: 4,
		Status:    model.TaskStepStatusCompleted,
		CanRetry:  true,
	}).Error; err != nil {
		t.Fatalf("create legacy metadata step: %v", err)
	}

	steps, err := service.GetTaskStepsByVideoID("video-legacy-read")
	if err != nil {
		t.Fatalf("GetTaskStepsByVideoID() error = %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("visible steps = %d, want 1", len(steps))
	}
	if steps[0].StepName != "生成视频元数据" {
		t.Fatalf("step name = %q, want canonical metadata step", steps[0].StepName)
	}
	if steps[0].StepOrder != 7 {
		t.Fatalf("step order = %d, want canonical order 7", steps[0].StepOrder)
	}
}

func TestGetTaskProgressTreatsPreparedVideoAsCompleteBeforeUpload(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	if err := db.Create(&model.SavedVideo{
		VideoID: "video-ready",
		URL:     "https://www.youtube.com/watch?v=abcdefghijk",
		Status:  "200",
	}).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	for _, step := range []model.TaskStep{
		{VideoID: "video-ready", StepName: "下载视频", StepOrder: 1, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-ready", StepName: "分离音频", StepOrder: 2, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-ready", StepName: "B站必剪转录", StepOrder: 3, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-ready", StepName: "生成字幕", StepOrder: 4, Status: model.TaskStepStatusSkipped, CanRetry: true},
		{VideoID: "video-ready", StepName: "下载封面", StepOrder: 5, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-ready", StepName: "翻译字幕", StepOrder: 6, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-ready", StepName: "生成视频元数据", StepOrder: 7, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-ready", StepName: "上传到Bilibili", StepOrder: 8, Status: model.TaskStepStatusPending, CanRetry: true},
		{VideoID: "video-ready", StepName: "上传字幕到Bilibili", StepOrder: 9, Status: model.TaskStepStatusPending, CanRetry: true},
	} {
		if err := db.Create(&step).Error; err != nil {
			t.Fatalf("create step %s: %v", step.StepName, err)
		}
	}

	progress, err := service.GetTaskProgress("video-ready")
	if err != nil {
		t.Fatalf("GetTaskProgress() error = %v", err)
	}
	if progress["total_steps"] != 7 {
		t.Fatalf("total_steps = %v, want 7", progress["total_steps"])
	}
	if progress["completed_steps"] != 7 {
		t.Fatalf("completed_steps = %v, want 7", progress["completed_steps"])
	}
	if progress["failed_steps"] != 0 {
		t.Fatalf("failed_steps = %v, want 0", progress["failed_steps"])
	}
	if progress["progress_percent"] != 100 {
		t.Fatalf("progress_percent = %v, want 100", progress["progress_percent"])
	}
}

func TestPreparedVideoTreatsStaleFailedSubtitleBranchAsSkipped(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	if err := db.Create(&model.SavedVideo{
		VideoID: "video-stale-mimo",
		URL:     "https://www.youtube.com/watch?v=abcdefghijk",
		Status:  "200",
	}).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	for _, step := range []model.TaskStep{
		{VideoID: "video-stale-mimo", StepName: "下载视频", StepOrder: 1, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-stale-mimo", StepName: "生成字幕", StepOrder: 2, Status: model.TaskStepStatusFailed, ErrorMsg: "old mimo failure", CanRetry: true},
		{VideoID: "video-stale-mimo", StepName: "翻译字幕", StepOrder: 3, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-stale-mimo", StepName: "生成元数据", StepOrder: 4, Status: model.TaskStepStatusCompleted, CanRetry: true},
	} {
		if err := db.Create(&step).Error; err != nil {
			t.Fatalf("create step %s: %v", step.StepName, err)
		}
	}

	steps, err := service.GetTaskStepsByVideoID("video-stale-mimo")
	if err != nil {
		t.Fatalf("GetTaskStepsByVideoID() error = %v", err)
	}
	var subtitleStep *model.TaskStep
	for i := range steps {
		if steps[i].StepName == "生成字幕" {
			subtitleStep = &steps[i]
			break
		}
	}
	if subtitleStep == nil {
		t.Fatalf("visible subtitle step missing: %#v", steps)
	}
	if subtitleStep.Status != model.TaskStepStatusSkipped {
		t.Fatalf("subtitle status = %q, want skipped for stale completed-preparation failure", subtitleStep.Status)
	}
	if subtitleStep.ErrorMsg != "" {
		t.Fatalf("subtitle error = %q, want cleared stale error", subtitleStep.ErrorMsg)
	}

	progress, err := service.GetTaskProgress("video-stale-mimo")
	if err != nil {
		t.Fatalf("GetTaskProgress() error = %v", err)
	}
	if progress["failed_steps"] != 0 {
		t.Fatalf("failed_steps = %v, want 0", progress["failed_steps"])
	}
	if progress["progress_percent"] != 100 {
		t.Fatalf("progress_percent = %v, want 100", progress["progress_percent"])
	}
}

func TestFailedVideoKeepsFailedSubtitleBranchVisible(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	if err := db.Create(&model.SavedVideo{
		VideoID: "video-real-failure",
		URL:     "https://www.youtube.com/watch?v=abcdefghijk",
		Status:  "999",
	}).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	if err := db.Create(&model.TaskStep{
		VideoID:   "video-real-failure",
		StepName:  "生成字幕",
		StepOrder: 4,
		Status:    model.TaskStepStatusFailed,
		ErrorMsg:  "current failure",
		CanRetry:  true,
	}).Error; err != nil {
		t.Fatalf("create failed step: %v", err)
	}

	progress, err := service.GetTaskProgress("video-real-failure")
	if err != nil {
		t.Fatalf("GetTaskProgress() error = %v", err)
	}
	if progress["failed_steps"] != 1 {
		t.Fatalf("failed_steps = %v, want 1 for real failed video", progress["failed_steps"])
	}
}

func TestGetTaskProgressIncludesSubtitleUploadAfterVideoUploaded(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	if err := db.Create(&model.SavedVideo{
		VideoID: "video-uploaded",
		URL:     "https://www.youtube.com/watch?v=abcdefghijk",
		Status:  "300",
	}).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	for _, step := range []model.TaskStep{
		{VideoID: "video-uploaded", StepName: "下载视频", StepOrder: 1, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-uploaded", StepName: "分离音频", StepOrder: 2, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-uploaded", StepName: "B站必剪转录", StepOrder: 3, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-uploaded", StepName: "生成字幕", StepOrder: 4, Status: model.TaskStepStatusSkipped, CanRetry: true},
		{VideoID: "video-uploaded", StepName: "下载封面", StepOrder: 5, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-uploaded", StepName: "翻译字幕", StepOrder: 6, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-uploaded", StepName: "生成视频元数据", StepOrder: 7, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-uploaded", StepName: "上传到Bilibili", StepOrder: 8, Status: model.TaskStepStatusCompleted, CanRetry: true},
		{VideoID: "video-uploaded", StepName: "上传字幕到Bilibili", StepOrder: 9, Status: model.TaskStepStatusPending, CanRetry: true},
	} {
		if err := db.Create(&step).Error; err != nil {
			t.Fatalf("create step %s: %v", step.StepName, err)
		}
	}

	progress, err := service.GetTaskProgress("video-uploaded")
	if err != nil {
		t.Fatalf("GetTaskProgress() error = %v", err)
	}
	if progress["total_steps"] != 9 {
		t.Fatalf("total_steps = %v, want 9", progress["total_steps"])
	}
	if progress["completed_steps"] != 8 {
		t.Fatalf("completed_steps = %v, want 8", progress["completed_steps"])
	}
	if progress["progress_percent"] != 88 {
		t.Fatalf("progress_percent = %v, want 88", progress["progress_percent"])
	}
}

func TestGetTaskStepsByVideoIDNormalizesHistoricalStepOrder(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	for _, step := range []model.TaskStep{
		{
			VideoID:   "video-old-order",
			StepName:  "下载视频",
			StepOrder: 1,
			Status:    model.TaskStepStatusCompleted,
			CanRetry:  true,
		},
		{
			VideoID:   "video-old-order",
			StepName:  "生成字幕",
			StepOrder: 2,
			Status:    model.TaskStepStatusSkipped,
			CanRetry:  true,
		},
		{
			VideoID:   "video-old-order",
			StepName:  "分离音频",
			StepOrder: 6,
			Status:    model.TaskStepStatusCompleted,
			CanRetry:  true,
		},
		{
			VideoID:   "video-old-order",
			StepName:  "下载封面",
			StepOrder: 9,
			Status:    model.TaskStepStatusCompleted,
			CanRetry:  true,
		},
	} {
		if err := db.Create(&step).Error; err != nil {
			t.Fatalf("create step %s: %v", step.StepName, err)
		}
	}

	steps, err := service.GetTaskStepsByVideoID("video-old-order")
	if err != nil {
		t.Fatalf("GetTaskStepsByVideoID() error = %v", err)
	}

	gotNames := make([]string, 0, len(steps))
	gotOrders := make([]int, 0, len(steps))
	for _, step := range steps {
		gotNames = append(gotNames, step.StepName)
		gotOrders = append(gotOrders, step.StepOrder)
	}

	wantNames := []string{"下载视频", "分离音频", "生成字幕", "下载封面"}
	wantOrders := []int{1, 2, 4, 5}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] || gotOrders[i] != wantOrders[i] {
			t.Fatalf("step[%d] = (%s,%d), want (%s,%d); all names=%v orders=%v",
				i, gotNames[i], gotOrders[i], wantNames[i], wantOrders[i], gotNames, gotOrders)
		}
	}
}

func TestGetPendingStepsIncludesUserRequestedRetry(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	if err := db.Create(&model.SavedVideo{
		VideoID: "video-retry",
		URL:     "https://www.youtube.com/watch?v=abcdefghijk",
		Status:  "999",
	}).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	now := time.Now().Add(-time.Minute)
	if err := db.Create(&model.TaskStep{
		VideoID:   "video-retry",
		StepName:  "生成字幕",
		StepOrder: 2,
		Status:    model.TaskStepStatusFailed,
		StartTime: &now,
		EndTime:   &now,
		ErrorMsg:  "previous failure",
		CanRetry:  true,
	}).Error; err != nil {
		t.Fatalf("create task step: %v", err)
	}
	if err := service.ResetTaskStep("video-retry", "生成字幕"); err != nil {
		t.Fatalf("ResetTaskStep() error = %v", err)
	}

	steps, err := service.GetPendingSteps()
	if err != nil {
		t.Fatalf("GetPendingSteps() error = %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("pending retry steps = %d, want 1", len(steps))
	}
	if steps[0].VideoID != "video-retry" || steps[0].StepName != "生成字幕" {
		t.Fatalf("unexpected retry step: %#v", steps[0])
	}
}

func TestSkipTaskStepIfPendingOnlySkipsPendingStep(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	if err := db.Create(&model.TaskStep{
		VideoID:   "video-branch",
		StepName:  "生成字幕",
		StepOrder: 4,
		Status:    model.TaskStepStatusPending,
		CanRetry:  true,
	}).Error; err != nil {
		t.Fatalf("create pending step: %v", err)
	}
	if err := db.Create(&model.TaskStep{
		VideoID:   "video-branch",
		StepName:  "B站必剪转录",
		StepOrder: 3,
		Status:    model.TaskStepStatusCompleted,
		CanRetry:  true,
	}).Error; err != nil {
		t.Fatalf("create completed step: %v", err)
	}

	if err := service.SkipTaskStepIfPending("video-branch", "生成字幕", "B站必剪转录已启用，跳过备用字幕生成"); err != nil {
		t.Fatalf("SkipTaskStepIfPending() pending error = %v", err)
	}
	if err := service.SkipTaskStepIfPending("video-branch", "B站必剪转录", "备用字幕生成已启用，跳过必剪转录"); err != nil {
		t.Fatalf("SkipTaskStepIfPending() completed error = %v", err)
	}

	var skipped model.TaskStep
	if err := db.Where("video_id = ? AND step_name = ?", "video-branch", "生成字幕").First(&skipped).Error; err != nil {
		t.Fatalf("query skipped step: %v", err)
	}
	if skipped.Status != model.TaskStepStatusSkipped {
		t.Fatalf("pending branch status = %q, want skipped", skipped.Status)
	}
	if !strings.Contains(skipped.ResultData, "B站必剪转录已启用") {
		t.Fatalf("skipped result_data = %q, want skip reason", skipped.ResultData)
	}

	var completed model.TaskStep
	if err := db.Where("video_id = ? AND step_name = ?", "video-branch", "B站必剪转录").First(&completed).Error; err != nil {
		t.Fatalf("query completed step: %v", err)
	}
	if completed.Status != model.TaskStepStatusCompleted {
		t.Fatalf("completed branch status = %q, want completed", completed.Status)
	}
}

func TestUpdateTaskStepStatusFailsWhenStepDoesNotExist(t *testing.T) {
	db := newTaskStepTestDB(t)
	service := NewTaskStepService(db)

	err := service.UpdateTaskStepStatus("video-missing", "分离音频", model.TaskStepStatusRunning)
	if err == nil {
		t.Fatal("UpdateTaskStepStatus() error = nil, want missing step error")
	}
	if !strings.Contains(err.Error(), "task step not found") {
		t.Fatalf("UpdateTaskStepStatus() error = %q, want task step not found", err)
	}
}

func newTaskStepTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.TaskStep{}, &model.SavedVideo{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}
