package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/services"
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"

	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	BaseHandler
	SavedVideoService *services.SavedVideoService
	TaskStepService   *services.TaskStepService
	UploadScheduler   interface {
		ExecuteManualUpload(videoID, taskType string) error
	}
	AnalyticsHandler *AnalyticsHandler
}

func NewVideoHandler(app *core.AppServer, savedVideoService *services.SavedVideoService, taskStepService *services.TaskStepService) *VideoHandler {
	return &VideoHandler{
		BaseHandler:       BaseHandler{App: app},
		SavedVideoService: savedVideoService,
		TaskStepService:   taskStepService,
		UploadScheduler:   nil, // Will be set later via SetUploadScheduler
	}
}

// SetUploadScheduler 设置上传调度器（避免循环依赖）
func (h *VideoHandler) SetUploadScheduler(scheduler interface {
	ExecuteManualUpload(videoID, taskType string) error
}) {
	h.UploadScheduler = scheduler
}

// RegisterRoutes 注册视频相关路由
func (h *VideoHandler) RegisterRoutes(api *gin.RouterGroup) {
	video := api.Group("/videos")
	{
		video.GET("", h.getVideoList)
		video.POST("/batch-delete", h.batchDeleteVideos)
		video.GET("/:id", h.getVideoDetail)
		video.DELETE("/:id", h.deleteVideo)
		video.POST("/:id/steps/:stepName/retry", h.retryTaskStep)
		video.GET("/:id/files", h.getVideoFiles)
		video.GET("/:id/files/:filename", h.serveVideoFile)
		video.POST("/:id/upload/video", h.manualUploadVideo)
		video.POST("/:id/upload/subtitle", h.manualUploadSubtitle)
	}
}

// VideoListResponse 视频列表响应
type VideoListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// VideoListData 视频列表数据
type VideoListData struct {
	Videos []VideoInfo `json:"videos"`
	Total  int         `json:"total"`
	Page   int         `json:"page"`
	Limit  int         `json:"limit"`
}

// VideoInfo 视频信息
type VideoInfo struct {
	ID             uint                     `json:"id"`
	VideoID        string                   `json:"video_id"`
	Title          string                   `json:"title"`
	URL            string                   `json:"url"`
	Status         string                   `json:"status"`
	GeneratedTitle string                   `json:"generated_title"`
	GeneratedDesc  string                   `json:"generated_desc"`
	GeneratedTags  string                   `json:"generated_tags"`
	BiliBVID       string                   `json:"bili_bvid"`
	BiliAID        int64                    `json:"bili_aid"`
	CreatedAt      string                   `json:"created_at"`
	UpdatedAt      string                   `json:"updated_at"`
	TaskSteps      []TaskStepInfo           `json:"task_steps,omitempty"`
	Progress       map[string]interface{}   `json:"progress,omitempty"`
	CoverImage     string                   `json:"cover_image,omitempty"`
	MetaData       map[string]interface{}   `json:"meta_data,omitempty"`
	Files          []map[string]interface{} `json:"files,omitempty"`
}

// TaskStepInfo 任务步骤信息
type TaskStepInfo struct {
	StepName  string `json:"step_name"`
	StepOrder int    `json:"step_order"`
	Status    string `json:"status"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Duration  int64  `json:"duration"`
	ErrorMsg  string `json:"error_msg"`
	CanRetry  bool   `json:"can_retry"`
}

// getVideoList 获取视频列表
func (h *VideoHandler) getVideoList(c *gin.Context) {
	// 解析分页参数
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}

	// 计算偏移量
	offset := (page - 1) * limit

	// 获取视频列表
	savedVideos, total, err := h.SavedVideoService.GetVideosPaginated(offset, limit)
	if err != nil {
		h.App.Logger.Errorf("获取视频列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, VideoListResponse{
			Code:    500,
			Message: "获取视频列表失败",
		})
		return
	}

	// 转换为响应格式
	var videos []VideoInfo
	for _, sv := range savedVideos {
		videos = append(videos, VideoInfo{
			ID:             sv.ID,
			VideoID:        sv.VideoID,
			Title:          sv.Title,
			URL:            sv.URL,
			Status:         sv.Status,
			GeneratedTitle: sv.GeneratedTitle,
			GeneratedDesc:  sv.GeneratedDesc,
			GeneratedTags:  sv.GeneratedTags,
			BiliBVID:       sv.BiliBVID,
			BiliAID:        sv.BiliAID,
			CreatedAt:      sv.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:      sv.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, VideoListResponse{
		Code:    200,
		Message: "success",
		Data: VideoListData{
			Videos: videos,
			Total:  total,
			Page:   page,
			Limit:  limit,
		},
	})
}

// getVideoDetail 获取视频详情
func (h *VideoHandler) getVideoDetail(c *gin.Context) {
	idStr := c.Param("id")

	// 尝试解析为数字ID，如果失败则当作video_id（字符串）处理
	var savedVideo *model.SavedVideo
	var err error

	if id, parseErr := strconv.ParseUint(idStr, 10, 32); parseErr == nil {
		// 如果可以解析为数字，则按ID查询
		savedVideo, err = h.SavedVideoService.GetByID(uint(id))
	} else {
		// 否则按video_id查询
		savedVideo, err = h.SavedVideoService.GetVideoByVideoID(idStr)
	}

	if err != nil {
		h.App.Logger.Errorf("获取视频详情失败: %v", err)
		c.JSON(http.StatusNotFound, VideoListResponse{
			Code:    404,
			Message: "视频不存在",
		})
		return
	}

	// 获取任务步骤
	taskSteps, err := h.TaskStepService.GetTaskStepsByVideoID(savedVideo.VideoID)
	if err != nil {
		h.App.Logger.Errorf("获取任务步骤失败: %v", err)
	}

	// 转换任务步骤格式
	var taskStepInfos []TaskStepInfo
	for _, step := range taskSteps {
		stepInfo := TaskStepInfo{
			StepName:  step.StepName,
			StepOrder: step.StepOrder,
			Status:    step.Status,
			Duration:  step.Duration,
			ErrorMsg:  step.ErrorMsg,
			CanRetry:  step.CanRetry,
		}

		if step.StartTime != nil {
			stepInfo.StartTime = step.StartTime.Format("2006-01-02 15:04:05")
		}
		if step.EndTime != nil {
			stepInfo.EndTime = step.EndTime.Format("2006-01-02 15:04:05")
		}

		taskStepInfos = append(taskStepInfos, stepInfo)
	}

	// 获取任务进度
	progress, err := h.TaskStepService.GetTaskProgress(savedVideo.VideoID)
	if err != nil {
		h.App.Logger.Errorf("获取任务进度失败: %v", err)
	}

	// 获取元数据文件
	metaData := h.getVideoMetaData(savedVideo.VideoID)

	// 获取封面图片
	coverImage := h.getVideoCoverImage(savedVideo.VideoID)
	videoDir := h.getVideoDirectory(savedVideo.VideoID)
	files := h.listVideoFiles(videoDir)

	videoInfo := VideoInfo{
		ID:             savedVideo.ID,
		VideoID:        savedVideo.VideoID,
		Title:          savedVideo.Title,
		URL:            savedVideo.URL,
		Status:         savedVideo.Status,
		GeneratedTitle: savedVideo.GeneratedTitle,
		GeneratedDesc:  savedVideo.GeneratedDesc,
		GeneratedTags:  savedVideo.GeneratedTags,
		BiliBVID:       savedVideo.BiliBVID,
		BiliAID:        savedVideo.BiliAID,
		CreatedAt:      savedVideo.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:      savedVideo.UpdatedAt.Format("2006-01-02 15:04:05"),
		TaskSteps:      taskStepInfos,
		Progress:       progress,
		CoverImage:     coverImage,
		MetaData:       metaData,
		Files:          files,
	}

	c.JSON(http.StatusOK, VideoListResponse{
		Code:    200,
		Message: "success",
		Data:    videoInfo,
	})
}

// retryTaskStep 重新执行任务步骤
func (h *VideoHandler) retryTaskStep(c *gin.Context) {
	idStr := c.Param("id")
	stepName := c.Param("stepName")

	// 尝试解析为数字ID，如果失败则当作video_id处理
	var savedVideo *model.SavedVideo
	var err error

	if id, parseErr := strconv.ParseUint(idStr, 10, 32); parseErr == nil {
		savedVideo, err = h.SavedVideoService.GetByID(uint(id))
	} else {
		savedVideo, err = h.SavedVideoService.GetVideoByVideoID(idStr)
	}

	if err != nil {
		h.App.Logger.Errorf("获取视频详情失败: %v", err)
		c.JSON(http.StatusNotFound, VideoListResponse{
			Code:    404,
			Message: "视频不存在",
		})
		return
	}

	// 检查步骤是否存在且可重试
	taskStep, err := h.TaskStepService.GetTaskStepByName(savedVideo.VideoID, stepName)
	if err != nil {
		c.JSON(http.StatusNotFound, VideoListResponse{
			Code:    404,
			Message: "任务步骤不存在",
		})
		return
	}

	if !taskStep.CanRetry {
		c.JSON(http.StatusBadRequest, VideoListResponse{
			Code:    400,
			Message: "此任务步骤不支持重试",
		})
		return
	}

	// 重新执行任务步骤
	h.App.Logger.Infof("🔄 用户请求重试任务步骤: %s - %s", savedVideo.VideoID, stepName)

	// 重置任务步骤状态为待执行，同时清理旧错误和旧结果。
	err = h.TaskStepService.ResetTaskStep(savedVideo.VideoID, stepName)
	if err != nil {
		h.App.Logger.Errorf("更新任务步骤状态失败: %v", err)
		c.JSON(http.StatusInternalServerError, VideoListResponse{
			Code:    500,
			Message: "更新任务状态失败",
		})
		return
	}

	h.App.Logger.Infof("✅ 任务步骤 %s 已重置为待执行状态，等待调度器处理", stepName)

	c.JSON(http.StatusOK, VideoListResponse{
		Code:    200,
		Message: fmt.Sprintf("任务步骤 %s 已加入重新执行队列", stepName),
		Data: gin.H{
			"video_id":  savedVideo.VideoID,
			"step_name": stepName,
			"status":    "pending",
			"message":   "任务已重置，将在下次调度时重新执行",
		},
	})
}

// runningStatuses 运行中状态：有后台进程（任务链 cron / 上传协程）正在读写该任务的
// 目录或数据库行。此时删除会撞上正在写的文件、或让协程写入已不存在的记录，因此一律拒删。
// 002 处理中 / 201 上传视频中 / 301 上传字幕中
var runningStatuses = map[string]struct{}{
	"001": {},
	"002": {},
	"200": {},
	"201": {},
	"300": {},
	"301": {},
}

func isRunningStatus(status string) bool {
	_, ok := runningStatuses[status]
	return ok
}

// deleteOutcome 单条删除结果，用于按 ID 如实回报每条任务的删除情况
type deleteOutcome struct {
	ID           uint   `json:"id"`
	VideoID      string `json:"video_id"`
	FilesRemoved bool   `json:"files_removed"` // 本地媒体目录是否被删除
	Note         string `json:"note,omitempty"`
}

type skipOutcome struct {
	ID      uint   `json:"id"`
	VideoID string `json:"video_id,omitempty"`
	Status  string `json:"status,omitempty"`
	Reason  string `json:"reason"`
}

// hardDeleteOne 物理删除一个任务的全部痕迹：DB 记录（事务内，步骤+视频一起删）+ 本地媒体目录。
// DB 删除失败直接返回错误（视为删除失败）；文件删除失败不回滚 DB，只如实记入 note，
// 因为删除的核心目的是清掉占空间的本地视频，文件没删掉用户必须知道。
func (h *VideoHandler) hardDeleteOne(v *model.SavedVideo) deleteOutcome {
	outcome := deleteOutcome{ID: v.ID, VideoID: v.VideoID}

	if err := h.SavedVideoService.HardDeleteWithSteps(v.ID, v.VideoID); err != nil {
		outcome.Note = fmt.Sprintf("数据库删除失败: %v", err)
		return outcome
	}

	removed, err := h.removeVideoFiles(v.VideoID)
	switch {
	case err != nil:
		outcome.Note = fmt.Sprintf("数据库已删除，但本地文件清理失败: %v", err)
	case removed:
		outcome.FilesRemoved = true
	default:
		outcome.Note = "数据库已删除，未发现本地媒体目录"
	}
	return outcome
}

// deleteVideo 硬删除单条视频（供视频详情页 DELETE /videos/:id 使用）
func (h *VideoHandler) deleteVideo(c *gin.Context) {
	idStr := c.Param("id")

	var savedVideo *model.SavedVideo
	var err error
	if id, parseErr := strconv.ParseUint(idStr, 10, 32); parseErr == nil {
		savedVideo, err = h.SavedVideoService.GetByID(uint(id))
	} else {
		savedVideo, err = h.SavedVideoService.GetVideoByVideoID(idStr)
	}

	if err != nil {
		c.JSON(http.StatusNotFound, VideoListResponse{Code: 404, Message: "视频不存在"})
		return
	}

	if isRunningStatus(savedVideo.Status) {
		c.JSON(http.StatusBadRequest, VideoListResponse{
			Code:    400,
			Message: fmt.Sprintf("当前状态 %s 为运行中，请等待任务完成或失败后再删除", savedVideo.Status),
		})
		return
	}

	h.App.Logger.Infof("🗑️ 硬删除视频: %s (ID: %d)", savedVideo.VideoID, savedVideo.ID)
	outcome := h.hardDeleteOne(savedVideo)
	if outcome.Note != "" && !outcome.FilesRemoved && strings.HasPrefix(outcome.Note, "数据库删除失败") {
		c.JSON(http.StatusInternalServerError, VideoListResponse{Code: 500, Message: outcome.Note})
		return
	}

	c.JSON(http.StatusOK, VideoListResponse{Code: 200, Message: "视频删除成功", Data: outcome})
}

// BatchDeleteRequest 批量删除请求：由前端显式给出要删除的任务 ID 列表（所见即所删）
type BatchDeleteRequest struct {
	IDs []uint `json:"ids"`
}

// batchDelete 按显式 ID 列表批量硬删除任务。运行中任务被跳过并在响应中说明；
// 删除哪些完全由前端决定，后端不自行推断"历史"范围，避免与后台新建任务产生竞态。
func (h *VideoHandler) batchDeleteVideos(c *gin.Context) {
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, VideoListResponse{Code: 400, Message: "请提供要删除的任务 ID 列表"})
		return
	}

	videos, err := h.SavedVideoService.GetByIDs(req.IDs)
	if err != nil {
		h.App.Logger.Errorf("查询待删除视频失败: %v", err)
		c.JSON(http.StatusInternalServerError, VideoListResponse{Code: 500, Message: "查询待删除视频失败"})
		return
	}

	// 标记查询到的 ID，找出请求里不存在的 ID
	found := make(map[uint]struct{}, len(videos))
	for i := range videos {
		found[videos[i].ID] = struct{}{}
	}

	deleted := make([]deleteOutcome, 0, len(videos))
	skipped := make([]skipOutcome, 0)

	for _, id := range req.IDs {
		if _, ok := found[id]; !ok {
			skipped = append(skipped, skipOutcome{ID: id, Reason: "记录不存在"})
		}
	}

	for i := range videos {
		v := videos[i]
		if isRunningStatus(v.Status) {
			skipped = append(skipped, skipOutcome{ID: v.ID, VideoID: v.VideoID, Status: v.Status, Reason: "任务运行中，已跳过"})
			continue
		}
		h.App.Logger.Infof("🗑️ 批量删除: %s (ID: %d)", v.VideoID, v.ID)
		deleted = append(deleted, h.hardDeleteOne(&v))
	}

	h.App.Logger.Infof("✅ 批量删除完成: 成功 %d 条, 跳过 %d 条", len(deleted), len(skipped))

	c.JSON(http.StatusOK, VideoListResponse{
		Code:    200,
		Message: fmt.Sprintf("已删除 %d 条任务，跳过 %d 条", len(deleted), len(skipped)),
		Data: gin.H{
			"deleted": deleted,
			"skipped": skipped,
		},
	})
}

// removeVideoFiles 删除视频的本地媒体目录，返回是否删除了目录。
// 数据布局为 {FileUpDir}/{YYYY-MM-DD}/{videoID}/，日期目录不确定，用 glob 匹配所有日期目录。
func (h *VideoHandler) removeVideoFiles(videoID string) (bool, error) {
	baseDir := h.App.Config.FileUpDir
	pattern := filepath.Join(baseDir, "*", videoID)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return false, err
	}

	removedAny := false
	var firstErr error
	for _, dir := range matches {
		info, statErr := os.Stat(dir)
		if statErr != nil || !info.IsDir() {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		h.App.Logger.Infof("✅ 已删除本地视频目录: %s", dir)
		removedAny = true
	}
	return removedAny, firstErr
}

// getVideoFiles 获取视频相关文件列表
func (h *VideoHandler) getVideoFiles(c *gin.Context) {
	idStr := c.Param("id")

	// 尝试解析为数字ID，如果失败则当作video_id处理
	var savedVideo *model.SavedVideo
	var err error

	if id, parseErr := strconv.ParseUint(idStr, 10, 32); parseErr == nil {
		savedVideo, err = h.SavedVideoService.GetByID(uint(id))
	} else {
		savedVideo, err = h.SavedVideoService.GetVideoByVideoID(idStr)
	}

	if err != nil {
		h.App.Logger.Errorf("获取视频详情失败: %v", err)
		c.JSON(http.StatusNotFound, VideoListResponse{
			Code:    404,
			Message: "视频不存在",
		})
		return
	}

	// 获取视频文件目录
	videoDir := h.getVideoDirectory(savedVideo.VideoID)
	files := h.listVideoFiles(videoDir)

	c.JSON(http.StatusOK, VideoListResponse{
		Code:    200,
		Message: "success",
		Data: gin.H{
			"video_id":  savedVideo.VideoID,
			"directory": videoDir,
			"files":     files,
		},
	})
}

// getVideoMetaData 获取视频元数据
func (h *VideoHandler) getVideoMetaData(videoID string) map[string]interface{} {
	videoDir := h.getVideoDirectory(videoID)
	metaPath := filepath.Join(videoDir, "meta.json")

	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(metaPath)
	if err != nil {
		h.App.Logger.Errorf("读取meta.json失败: %v", err)
		return nil
	}

	var metaData map[string]interface{}
	if err := json.Unmarshal(data, &metaData); err != nil {
		h.App.Logger.Errorf("解析meta.json失败: %v", err)
		return nil
	}

	return metaData
}

// getVideoCoverImage 获取视频封面图片路径
func (h *VideoHandler) getVideoCoverImage(videoID string) string {
	videoDir := h.getVideoDirectory(videoID)
	candidates := []string{
		"cover.jpg",
		"cover.jpeg",
		"cover.png",
		"cover.webp",
		"maxresdefault.jpg",
		"sddefault.jpg",
		"hqdefault.jpg",
		"mqdefault.jpg",
		"default.jpg",
	}

	for _, name := range candidates {
		coverPath := filepath.Join(videoDir, name)
		if info, err := os.Stat(coverPath); err == nil && !info.IsDir() {
			return h.fileDownloadPath(videoID, name)
		}
	}

	entries, err := os.ReadDir(videoDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() || h.getFileType(entry.Name()) != "image" {
			continue
		}
		return h.fileDownloadPath(videoID, entry.Name())
	}

	return ""
}

// getVideoDirectory 获取视频文件目录
func (h *VideoHandler) getVideoDirectory(videoID string) string {
	// 根据配置获取文件上传目录
	baseDir := h.App.Config.FileUpDir

	pattern := filepath.Join(baseDir, "*", videoID)
	matches, err := filepath.Glob(pattern)
	if err == nil {
		for i := len(matches) - 1; i >= 0; i-- {
			dir := matches[i]
			if info, statErr := os.Stat(dir); statErr == nil && info.IsDir() {
				return dir
			}
		}
	}

	return pattern
}

// listVideoFiles 列出视频目录中的所有文件
func (h *VideoHandler) listVideoFiles(dirPattern string) []map[string]interface{} {
	var files []map[string]interface{}

	// 使用glob匹配目录
	matches, err := filepath.Glob(dirPattern)
	if err != nil || len(matches) == 0 {
		return files
	}

	dir := matches[0] // 取第一个匹配的目录
	videoID := filepath.Base(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		h.App.Logger.Errorf("读取目录失败: %v", err)
		return files
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileType := h.getFileType(entry.Name())
		files = append(files, map[string]interface{}{
			"name":       entry.Name(),
			"path":       h.fileDownloadPath(videoID, entry.Name()),
			"size":       info.Size(),
			"type":       fileType,
			"modified":   info.ModTime().Format("2006-01-02 15:04:05"),
			"created_at": info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	return files
}

func (h *VideoHandler) serveVideoFile(c *gin.Context) {
	videoID := c.Param("id")
	filename := c.Param("filename")
	if !isSafeVideoFilename(filename) {
		c.JSON(http.StatusBadRequest, VideoListResponse{
			Code:    400,
			Message: "invalid file name",
		})
		return
	}

	videoDir := h.getVideoDirectory(videoID)
	targetPath := filepath.Join(videoDir, filename)
	if !isPathInsideDirectory(videoDir, targetPath) {
		c.JSON(http.StatusBadRequest, VideoListResponse{
			Code:    400,
			Message: "invalid file path",
		})
		return
	}

	info, err := os.Stat(targetPath)
	if err != nil || info.IsDir() {
		c.JSON(http.StatusNotFound, VideoListResponse{
			Code:    404,
			Message: "file not found",
		})
		return
	}

	c.File(targetPath)
}

func (h *VideoHandler) fileDownloadPath(videoID, filename string) string {
	return fmt.Sprintf("/api/v1/videos/%s/files/%s", videoID, filename)
}

func isSafeVideoFilename(filename string) bool {
	return filename != "" &&
		filename == filepath.Base(filename) &&
		!strings.Contains(filename, "/") &&
		!strings.Contains(filename, "\\") &&
		filename != "." &&
		filename != ".."
}

func isPathInsideDirectory(dir, targetPath string) bool {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, absTarget)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// getFileType 根据文件扩展名判断文件类型
func (h *VideoHandler) getFileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".mp4", ".flv", ".mkv", ".webm", ".avi", ".mov":
		return "video"
	case ".srt", ".vtt":
		return "subtitle"
	case ".jpg", ".jpeg", ".png", ".webp":
		return "image"
	case ".json":
		return "metadata"
	case ".mp3", ".wav", ".m4a":
		return "audio"
	default:
		return "other"
	}
}

// manualUploadVideo 手动触发视频上传
func (h *VideoHandler) manualUploadVideo(c *gin.Context) {
	idStr := c.Param("id")

	// 尝试解析为数字ID，如果失败则当作video_id处理
	var savedVideo *model.SavedVideo
	var err error

	if id, parseErr := strconv.ParseUint(idStr, 10, 32); parseErr == nil {
		savedVideo, err = h.SavedVideoService.GetByID(uint(id))
	} else {
		savedVideo, err = h.SavedVideoService.GetVideoByVideoID(idStr)
	}

	if err != nil {
		h.App.Logger.Errorf("获取视频详情失败: %v", err)
		c.JSON(http.StatusNotFound, VideoListResponse{
			Code:    404,
			Message: "视频不存在",
		})
		return
	}

	// 检查视频状态是否允许上传
	if savedVideo.Status != "200" && savedVideo.Status != "299" {
		c.JSON(http.StatusBadRequest, VideoListResponse{
			Code:    400,
			Message: fmt.Sprintf("当前状态 %s 不允许上传视频，只有状态为 200(准备就绪) 或 299(上传失败) 的视频才能上传", savedVideo.Status),
		})
		return
	}

	// 检查上传调度器是否已设置
	if h.UploadScheduler == nil {
		c.JSON(http.StatusInternalServerError, VideoListResponse{
			Code:    500,
			Message: "上传调度器未初始化",
		})
		return
	}

	h.App.Logger.Infof("🚀 用户手动触发视频上传: %s (%s)", savedVideo.VideoID, savedVideo.Title)

	// 更新状态为上传中
	updated, err := h.SavedVideoService.UpdateStatusIfCurrent(savedVideo.ID, savedVideo.Status, "201")
	if err != nil {
		h.App.Logger.Errorf("更新视频状态失败: %v", err)
		c.JSON(http.StatusInternalServerError, VideoListResponse{
			Code:    500,
			Message: "更新视频状态失败",
		})
		return
	}
	if !updated {
		c.JSON(http.StatusConflict, VideoListResponse{
			Code:    http.StatusConflict,
			Message: "任务状态已变化，可能已被其他上传执行器处理，请刷新后重试",
		})
		return
	}

	// 异步执行上传任务
	go func() {
		if err := h.UploadScheduler.ExecuteManualUpload(savedVideo.VideoID, "video"); err != nil {
			h.App.Logger.Errorf("手动上传视频失败: %v", err)
			// 上传失败，更新状态为 299
			h.SavedVideoService.UpdateStatus(savedVideo.ID, "299")
		} else {
			h.App.Logger.Infof("✅ 手动上传视频成功: %s", savedVideo.VideoID)
			// 上传成功，更新状态为 300
			h.SavedVideoService.UpdateStatus(savedVideo.ID, "300")
		}
	}()

	c.JSON(http.StatusOK, VideoListResponse{
		Code:    200,
		Message: "视频上传任务已启动",
		Data: gin.H{
			"video_id": savedVideo.VideoID,
			"status":   "201",
			"message":  "视频正在后台上传中，请稍后刷新查看结果",
		},
	})
}

// manualUploadSubtitle 手动触发字幕上传
func (h *VideoHandler) manualUploadSubtitle(c *gin.Context) {
	idStr := c.Param("id")

	// 尝试解析为数字ID，如果失败则当作video_id处理
	var savedVideo *model.SavedVideo
	var err error

	if id, parseErr := strconv.ParseUint(idStr, 10, 32); parseErr == nil {
		savedVideo, err = h.SavedVideoService.GetByID(uint(id))
	} else {
		savedVideo, err = h.SavedVideoService.GetVideoByVideoID(idStr)
	}

	if err != nil {
		h.App.Logger.Errorf("获取视频详情失败: %v", err)
		c.JSON(http.StatusNotFound, VideoListResponse{
			Code:    404,
			Message: "视频不存在",
		})
		return
	}

	// 检查视频状态是否允许上传字幕
	if savedVideo.Status != "300" && savedVideo.Status != "399" {
		c.JSON(http.StatusBadRequest, VideoListResponse{
			Code:    400,
			Message: fmt.Sprintf("当前状态 %s 不允许上传字幕，只有状态为 300(视频已上传) 或 399(字幕上传失败) 的视频才能上传字幕", savedVideo.Status),
		})
		return
	}

	// 检查是否已有BVID
	if savedVideo.BiliBVID == "" {
		c.JSON(http.StatusBadRequest, VideoListResponse{
			Code:    400,
			Message: "视频尚未上传到Bilibili，无法上传字幕",
		})
		return
	}

	// 检查上传调度器是否已设置
	if h.UploadScheduler == nil {
		c.JSON(http.StatusInternalServerError, VideoListResponse{
			Code:    500,
			Message: "上传调度器未初始化",
		})
		return
	}

	h.App.Logger.Infof("🚀 用户手动触发字幕上传: %s (%s)", savedVideo.VideoID, savedVideo.Title)

	// 更新状态为上传字幕中
	updated, err := h.SavedVideoService.UpdateStatusIfCurrent(savedVideo.ID, savedVideo.Status, "301")
	if err != nil {
		h.App.Logger.Errorf("更新视频状态失败: %v", err)
		c.JSON(http.StatusInternalServerError, VideoListResponse{
			Code:    500,
			Message: "更新视频状态失败",
		})
		return
	}
	if !updated {
		c.JSON(http.StatusConflict, VideoListResponse{
			Code:    http.StatusConflict,
			Message: "任务状态已变化，可能已被其他上传执行器处理，请刷新后重试",
		})
		return
	}

	// 异步执行上传字幕任务
	go func() {
		if err := h.UploadScheduler.ExecuteManualUpload(savedVideo.VideoID, "subtitle"); err != nil {
			h.App.Logger.Errorf("手动上传字幕失败: %v", err)
			// 上传失败，更新状态为 399
			h.SavedVideoService.UpdateStatus(savedVideo.ID, "399")
		} else {
			h.App.Logger.Infof("✅ 手动上传字幕成功: %s", savedVideo.VideoID)
			// 上传成功，更新状态为 400
			h.SavedVideoService.UpdateStatus(savedVideo.ID, "400")
		}
	}()

	c.JSON(http.StatusOK, VideoListResponse{
		Code:    200,
		Message: "字幕上传任务已启动",
		Data: gin.H{
			"video_id": savedVideo.VideoID,
			"status":   "301",
			"message":  "字幕正在后台上传中，请稍后刷新查看结果",
		},
	})
}
