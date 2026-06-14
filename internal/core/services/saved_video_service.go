package services

import (
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"gorm.io/gorm"
)

// SavedVideoService 保存视频服务
type SavedVideoService struct {
	DB *gorm.DB
}

// NewSavedVideoService 创建保存视频服务实例
func NewSavedVideoService(db *gorm.DB) *SavedVideoService {
	return &SavedVideoService{
		DB: db,
	}
}

// GetPendingVideos 获取待处理的视频列表（状态为 001 且 subtitles 不为空）
func (s *SavedVideoService) GetPendingVideos(limit int) ([]model.SavedVideo, error) {
	var videos []model.SavedVideo
	err := s.DB.Where("status = ? AND subtitles IS NOT NULL AND subtitles != ''", "001").
		Order("created_at ASC").
		Limit(limit).
		Find(&videos).Error
	return videos, err
}

// GetVideoByID 根据ID获取视频
func (s *SavedVideoService) GetVideoByID(id uint) (*model.SavedVideo, error) {
	var video model.SavedVideo
	err := s.DB.Where("id = ?", id).First(&video).Error
	if err != nil {
		return nil, err
	}
	return &video, nil
}

// GetVideoByVideoID 根据 VideoID 获取视频
func (s *SavedVideoService) GetVideoByVideoID(videoID string) (*model.SavedVideo, error) {
	var video model.SavedVideo
	err := s.DB.Where("video_id = ?", videoID).First(&video).Error
	if err != nil {
		return nil, err
	}
	return &video, nil
}

// UpdateStatus 更新视频状态
func (s *SavedVideoService) UpdateStatus(id uint, status string) error {
	return s.DB.Model(&model.SavedVideo{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateVideo 更新视频信息
func (s *SavedVideoService) UpdateVideo(video *model.SavedVideo) error {
	return s.DB.Save(video).Error
}

// CreateVideo 创建新视频记录
func (s *SavedVideoService) CreateVideo(video *model.SavedVideo) error {
	return s.DB.Create(video).Error
}

// GetByIDs 按主键批量查询视频记录，用于删除前确认每条任务的状态与目录
func (s *SavedVideoService) GetByIDs(ids []uint) ([]model.SavedVideo, error) {
	var videos []model.SavedVideo
	if len(ids) == 0 {
		return videos, nil
	}
	err := s.DB.Where("id IN ?", ids).Find(&videos).Error
	return videos, err
}

// HardDeleteWithSteps 在单个事务中物理删除一个任务的全部数据库痕迹：
// 任务步骤(tb_task_steps) 与 视频记录(tb_saved_videos)。
// 二者要么一起删除成功，要么一起回滚，避免留下孤儿步骤行。
func (s *SavedVideoService) HardDeleteWithSteps(id uint, videoID string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("video_id = ?", videoID).Delete(&model.TaskStep{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&model.SavedVideo{}, id).Error; err != nil {
			return err
		}
		return nil
	})
}

// ListVideos 获取视频列表（支持分页和状态筛选）
func (s *SavedVideoService) ListVideos(page, pageSize int, status string) ([]model.SavedVideo, int64, error) {
	var videos []model.SavedVideo
	var total int64

	query := s.DB.Model(&model.SavedVideo{})
	
	// 如果指定状态，添加状态筛选
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&videos).Error

	return videos, total, err
}

// GetVideosByPlaylistID 根据播放列表ID获取视频列表
func (s *SavedVideoService) GetVideosByPlaylistID(playlistID string) ([]model.SavedVideo, error) {
	var videos []model.SavedVideo
	err := s.DB.Where("playlist_id = ?", playlistID).
		Order("created_at ASC").
		Find(&videos).Error
	return videos, err
}

// UpdateVideoStatus 批量更新视频状态
func (s *SavedVideoService) UpdateVideoStatus(ids []uint, status string) error {
	return s.DB.Model(&model.SavedVideo{}).
		Where("id IN ?", ids).
		Update("status", status).Error
}

// GetVideosPaginated 获取分页视频列表（用于前端显示）
func (s *SavedVideoService) GetVideosPaginated(offset, limit int) ([]model.SavedVideo, int, error) {
	var videos []model.SavedVideo
	var total int64

	// 获取总数
	if err := s.DB.Model(&model.SavedVideo{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	err := s.DB.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&videos).Error

	return videos, int(total), err
}

// GetByID 根据ID获取视频（别名方法，保持兼容性）
func (s *SavedVideoService) GetByID(id uint) (*model.SavedVideo, error) {
	return s.GetVideoByID(id)
}
