package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"

	"gorm.io/gorm"
)

// TaskStepService 任务步骤服务
type TaskStepService struct {
	DB *gorm.DB
}

const retryRequestedResultData = `{"retry_requested":true}`

type taskStepDefinition struct {
	Name     string
	Order    int
	CanRetry bool
}

var legacyTaskStepAliases = map[string]string{
	"生成元数据": "生成视频元数据",
}

// NewTaskStepService 创建任务步骤服务实例
func NewTaskStepService(db *gorm.DB) *TaskStepService {
	return &TaskStepService{
		DB: db,
	}
}

// InitTaskSteps 初始化视频的任务步骤
func (s *TaskStepService) InitTaskSteps(videoID string) error {
	// 定义标准任务步骤
	steps := standardTaskStepDefinitions()

	var existingSteps []model.TaskStep
	if err := s.DB.Where("video_id = ?", videoID).Find(&existingSteps).Error; err != nil {
		return err
	}
	if err := s.normalizeLegacyTaskSteps(videoID, existingSteps, steps); err != nil {
		return err
	}
	if err := s.DB.Where("video_id = ?", videoID).Find(&existingSteps).Error; err != nil {
		return err
	}
	existingByName := make(map[string]struct{}, len(existingSteps))
	for _, step := range existingSteps {
		existingByName[step.StepName] = struct{}{}
	}

	for _, step := range steps {
		if _, exists := existingByName[step.Name]; exists {
			if err := s.DB.Model(&model.TaskStep{}).
				Where("video_id = ? AND step_name = ? AND step_order != ?", videoID, step.Name, step.Order).
				Update("step_order", step.Order).Error; err != nil {
				return err
			}
			continue
		}
		taskStep := &model.TaskStep{
			VideoID:   videoID,
			StepName:  step.Name,
			StepOrder: step.Order,
			Status:    model.TaskStepStatusPending,
			CanRetry:  step.CanRetry,
		}

		if err := s.DB.Create(taskStep).Error; err != nil {
			return err
		}
	}

	return nil
}

func standardTaskStepDefinitions() []taskStepDefinition {
	return []taskStepDefinition{
		{"下载视频", 1, true},
		{"分离音频", 2, true},
		{"B站必剪转录", 3, true},
		{"生成字幕", 4, true},
		{"下载封面", 5, true},
		{"翻译字幕", 6, true},
		{"生成视频元数据", 7, true},
		{"上传到Bilibili", 8, true},
		{"上传字幕到Bilibili", 9, true},
	}
}

func (s *TaskStepService) normalizeLegacyTaskSteps(videoID string, existingSteps []model.TaskStep, standardSteps []taskStepDefinition) error {
	standardOrderByName := make(map[string]int, len(standardSteps))
	for _, step := range standardSteps {
		standardOrderByName[step.Name] = step.Order
	}

	existingByName := make(map[string]model.TaskStep, len(existingSteps))
	for _, step := range existingSteps {
		existingByName[step.StepName] = step
	}

	for legacyName, canonicalName := range legacyTaskStepAliases {
		legacy, hasLegacy := existingByName[legacyName]
		if !hasLegacy {
			continue
		}
		if _, hasCanonical := existingByName[canonicalName]; !hasCanonical {
			updates := map[string]interface{}{"step_name": canonicalName}
			if order, ok := standardOrderByName[canonicalName]; ok {
				updates["step_order"] = order
			}
			if err := s.DB.Model(&model.TaskStep{}).Where("id = ?", legacy.ID).Updates(updates).Error; err != nil {
				return err
			}
			continue
		}

		if legacy.Status == model.TaskStepStatusPending || legacy.Status == model.TaskStepStatusFailed {
			if err := s.SkipTaskStepIfPending(videoID, legacyName, "legacy step replaced by 生成视频元数据"); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetTaskStepsByVideoID 根据视频ID获取任务步骤列表
func (s *TaskStepService) GetTaskStepsByVideoID(videoID string) ([]model.TaskStep, error) {
	var steps []model.TaskStep
	err := s.DB.Where("video_id = ?", videoID).
		Order("step_order ASC").
		Find(&steps).Error
	if err != nil {
		return nil, err
	}
	videoStatus, err := s.getSavedVideoStatus(videoID)
	if err != nil {
		return nil, err
	}
	return normalizeVisibleTaskStepsForStatus(steps, videoStatus), nil
}

// UpdateTaskStepStatus 更新任务步骤状态
func (s *TaskStepService) UpdateTaskStepStatus(videoID, stepName, status string, errorMsg ...string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// 设置时间
	now := time.Now()
	if status == model.TaskStepStatusRunning {
		updates["start_time"] = &now
	} else if status == model.TaskStepStatusCompleted || status == model.TaskStepStatusFailed {
		updates["end_time"] = &now

		// 计算执行时长
		var step model.TaskStep
		if err := s.DB.Where("video_id = ? AND step_name = ?", videoID, stepName).First(&step).Error; err == nil {
			if step.StartTime != nil {
				duration := now.Sub(*step.StartTime).Milliseconds()
				updates["duration"] = duration
			}
		}
	}

	// 设置错误信息
	if len(errorMsg) > 0 && errorMsg[0] != "" {
		updates["error_msg"] = errorMsg[0]
	}

	result := s.DB.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ?", videoID, stepName).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task step not found: video_id=%s step_name=%s", videoID, stepName)
	}
	return nil
}

// UpdateTaskStepResult 更新任务步骤执行结果
func (s *TaskStepService) UpdateTaskStepResult(videoID, stepName string, resultData interface{}) error {
	var jsonData string
	if resultData != nil {
		if jsonBytes, err := json.Marshal(resultData); err == nil {
			jsonData = string(jsonBytes)
		}
	}

	return s.DB.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ?", videoID, stepName).
		Update("result_data", jsonData).Error
}

// ResetTaskStep 重置任务步骤（用于重新执行）
func (s *TaskStepService) ResetTaskStep(videoID, stepName string) error {
	updates := map[string]interface{}{
		"status":      model.TaskStepStatusPending,
		"start_time":  nil,
		"end_time":    nil,
		"duration":    0,
		"error_msg":   "",
		"result_data": retryRequestedResultData,
	}

	return s.DB.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ?", videoID, stepName).
		Updates(updates).Error
}

// SkipTaskStepIfPending marks an inactive branch step as skipped without overwriting active or completed work.
func (s *TaskStepService) SkipTaskStepIfPending(videoID, stepName, reason string) error {
	resultData := ""
	if reason != "" {
		if jsonBytes, err := json.Marshal(map[string]string{"skip_reason": reason}); err == nil {
			resultData = string(jsonBytes)
		}
	}

	result := s.DB.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ? AND status IN ?", videoID, stepName, []string{
			model.TaskStepStatusPending,
			model.TaskStepStatusFailed,
		}).
		Updates(map[string]interface{}{
			"status":      model.TaskStepStatusSkipped,
			"result_data": resultData,
			"error_msg":   "",
		})
	return result.Error
}

// GetTaskStepByName 根据视频ID和步骤名称获取特定步骤
func (s *TaskStepService) GetTaskStepByName(videoID, stepName string) (*model.TaskStep, error) {
	var step model.TaskStep
	err := s.DB.Where("video_id = ? AND step_name = ?", videoID, stepName).First(&step).Error
	if err != nil {
		return nil, err
	}
	return &step, nil
}

// GetTaskProgress 获取任务进度信息
func (s *TaskStepService) GetTaskProgress(videoID string) (map[string]interface{}, error) {
	var steps []model.TaskStep
	if err := s.DB.Where("video_id = ?", videoID).Order("step_order ASC").Find(&steps).Error; err != nil {
		return nil, err
	}
	videoStatus, err := s.getSavedVideoStatus(videoID)
	if err != nil {
		return nil, err
	}
	steps = normalizeVisibleTaskStepsForStatus(steps, videoStatus)
	steps = progressVisibleTaskSteps(steps, videoStatus)

	totalSteps := len(steps)
	completedSteps := 0
	failedSteps := 0
	currentStep := ""

	for _, step := range steps {
		switch step.Status {
		case model.TaskStepStatusCompleted, model.TaskStepStatusSkipped:
			completedSteps++
		case model.TaskStepStatusFailed:
			failedSteps++
		case model.TaskStepStatusRunning:
			currentStep = step.StepName
		}
	}

	progress := map[string]interface{}{
		"total_steps":      totalSteps,
		"completed_steps":  completedSteps,
		"failed_steps":     failedSteps,
		"current_step":     currentStep,
		"progress_percent": 0,
	}

	if totalSteps > 0 {
		progress["progress_percent"] = (completedSteps * 100) / totalSteps
	}

	return progress, nil
}

func (s *TaskStepService) getSavedVideoStatus(videoID string) (string, error) {
	var savedVideo model.SavedVideo
	if err := s.DB.Select("status").Where("video_id = ?", videoID).First(&savedVideo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return savedVideo.Status, nil
}

func progressVisibleTaskSteps(steps []model.TaskStep, videoStatus string) []model.TaskStep {
	filtered := make([]model.TaskStep, 0, len(steps))
	for _, step := range steps {
		if shouldIncludeStepInProgress(step, videoStatus) {
			filtered = append(filtered, step)
		}
	}
	return filtered
}

func shouldIncludeStepInProgress(step model.TaskStep, videoStatus string) bool {
	switch step.StepName {
	case "上传到Bilibili":
		return step.Status != model.TaskStepStatusPending || videoStatusAtOrAfterVideoUpload(videoStatus)
	case "上传字幕到Bilibili":
		return step.Status != model.TaskStepStatusPending || videoStatusAtOrAfterSubtitleUpload(videoStatus)
	default:
		return true
	}
}

func videoStatusAtOrAfterVideoUpload(status string) bool {
	switch status {
	case "201", "299", "300", "301", "399", "400":
		return true
	default:
		return false
	}
}

func videoStatusAtOrAfterSubtitleUpload(status string) bool {
	switch status {
	case "300", "301", "399", "400":
		return true
	default:
		return false
	}
}

func filterSupersededLegacySteps(steps []model.TaskStep) []model.TaskStep {
	present := make(map[string]struct{}, len(steps))
	for _, step := range steps {
		present[step.StepName] = struct{}{}
	}

	filtered := make([]model.TaskStep, 0, len(steps))
	for _, step := range steps {
		if canonicalName, ok := legacyTaskStepAliases[step.StepName]; ok {
			if _, hasCanonical := present[canonicalName]; hasCanonical {
				continue
			}
		}
		filtered = append(filtered, step)
	}
	return filtered
}

func normalizeVisibleTaskSteps(steps []model.TaskStep) []model.TaskStep {
	steps = filterSupersededLegacySteps(steps)

	orderByName := make(map[string]int)
	for _, step := range standardTaskStepDefinitions() {
		orderByName[step.Name] = step.Order
	}

	normalized := make([]model.TaskStep, len(steps))
	copy(normalized, steps)
	for i := range normalized {
		if canonicalName, ok := legacyTaskStepAliases[normalized[i].StepName]; ok {
			normalized[i].StepName = canonicalName
		}
		if order, ok := orderByName[normalized[i].StepName]; ok {
			normalized[i].StepOrder = order
		}
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].StepOrder == normalized[j].StepOrder {
			return normalized[i].ID < normalized[j].ID
		}
		return normalized[i].StepOrder < normalized[j].StepOrder
	})
	return normalized
}

func normalizeVisibleTaskStepsForStatus(steps []model.TaskStep, videoStatus string) []model.TaskStep {
	normalized := normalizeVisibleTaskSteps(steps)
	return reconcileStaleSubtitleBranchFailures(normalized, videoStatus)
}

func reconcileStaleSubtitleBranchFailures(steps []model.TaskStep, videoStatus string) []model.TaskStep {
	if !preparationStatusAtOrAfterComplete(videoStatus) || !hasCompletedMetadataStep(steps) {
		return steps
	}

	for i := range steps {
		if isSubtitleBranchStep(steps[i].StepName) && steps[i].Status == model.TaskStepStatusFailed {
			steps[i].Status = model.TaskStepStatusSkipped
			steps[i].ErrorMsg = ""
		}
	}
	return steps
}

func isSubtitleBranchStep(stepName string) bool {
	return stepName == "生成字幕" || stepName == "B站必剪转录"
}

func hasCompletedMetadataStep(steps []model.TaskStep) bool {
	for _, step := range steps {
		if step.StepName == "生成视频元数据" && (step.Status == model.TaskStepStatusCompleted || step.Status == model.TaskStepStatusSkipped) {
			return true
		}
	}
	return false
}

func preparationStatusAtOrAfterComplete(status string) bool {
	switch status {
	case "200", "250", "201", "299", "300", "301", "399", "400":
		return true
	default:
		return false
	}
}

// ResetAllRunningTasks 重置所有运行中的任务
func (s *TaskStepService) ResetAllRunningTasks() error {
	// 开始事务
	tx := s.DB.Begin()

	// 重置所有状态为 Running 的任务步骤为 Pending
	result := tx.Model(&model.TaskStep{}).
		Where("status = ?", model.TaskStepStatusRunning).
		Update("status", model.TaskStepStatusPending)

	if result.Error != nil {
		tx.Rollback()
		return fmt.Errorf("failed to reset running task steps: %v", result.Error)
	}

	taskStepsAffected := result.RowsAffected

	// 重置相关视频的状态
	// 将状态为 "002"(处理中) 的视频重置为 "001"(待处理)
	videoResult := tx.Model(&model.SavedVideo{}).
		Where("status = ?", "002").
		Update("status", "001")

	if videoResult.Error != nil {
		tx.Rollback()
		return fmt.Errorf("failed to reset running video status: %v", videoResult.Error)
	}

	videosAffected := videoResult.RowsAffected

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("Reset %d running task steps and %d videos (from processing to pending status)", taskStepsAffected, videosAffected)
	return nil
}

// GetPendingSteps 获取所有状态为pending的任务步骤
func (s *TaskStepService) GetPendingSteps() ([]*model.TaskStep, error) {
	var steps []*model.TaskStep

	// 使用 JOIN 查询，只获取未删除视频的待处理步骤
	result := s.DB.Table("tb_task_steps").
		Select("tb_task_steps.*").
		Joins("INNER JOIN tb_saved_videos ON tb_task_steps.video_id = tb_saved_videos.video_id").
		Where("tb_task_steps.status = ?", model.TaskStepStatusPending).
		Where("(tb_task_steps.result_data = ? OR tb_task_steps.start_time IS NOT NULL)", retryRequestedResultData).
		Where("tb_task_steps.deleted_at IS NULL").
		Where("tb_saved_videos.deleted_at IS NULL").
		Order("tb_task_steps.created_at ASC").
		Find(&steps)

	if result.Error != nil {
		return nil, fmt.Errorf("查询待重试步骤失败: %v", result.Error)
	}

	return steps, nil
}
