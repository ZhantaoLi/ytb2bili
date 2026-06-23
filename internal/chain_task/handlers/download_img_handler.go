package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/base"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/models"
	"github.com/ZhantaoLi/ytb2bili/pkg/cos"
	"github.com/ZhantaoLi/ytb2bili/pkg/utils"
	"gorm.io/gorm"
)

type DownloadImgHandler struct {
	base.BaseTask
	App *core.AppServer
	DB  *gorm.DB
}

var downloadYouTubeThumbnail = utils.DownloadYouTubeThumbnail

func NewDownloadImgHandler(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient) *DownloadImgHandler {
	return &DownloadImgHandler{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App: app,
	}

}

func (t *DownloadImgHandler) Execute(context map[string]interface{}) bool {

	opt := utils.DownloadOptions{
		SavePath:         t.StateManager.CurrentDir,
		FilenameTemplate: "{quality}",
		Timeout:          10 * time.Second,
		MaxRetries:       3,
		QualityFallback:  true,
		CreateDirs:       true,
		Overwrite:        false,
	}
	if t.App != nil && t.App.Config != nil && t.App.Config.ProxyConfig != nil && t.App.Config.ProxyConfig.UseProxy {
		opt.ProxyURL = strings.TrimSpace(t.App.Config.ProxyConfig.ProxyHost)
	}

	//utils.QualityMax,
	qualities := []utils.ImageQuality{utils.QualityMax, utils.QualityStandard}
	results := downloadYouTubeThumbnail(t.StateManager.VideoID, qualities, opt, "").(map[string]utils.DownloadResult)

	var maxQualityCoverPath string
	successCount := 0
	var failedMessages []string

	for k, v := range results {
		if v.Success {
			successCount++
			fmt.Printf("下载成功: %s - %s (%d bytes)\n", k, v.FilePath, v.FileSize)
			cosKeyName := ""
			if t.Client != nil {
				uploadedKey, err := t.Client.UploadImageToCOS(v.FilePath, "")
				if err != nil {
					t.App.Logger.Warnf("上传封面到 COS 失败，将继续使用本地封面: %v", err)
				} else {
					cosKeyName = uploadedKey
				}
			}

			// 如果是最高质量的封面，保存到context中供后续上传使用
			if k == string(utils.QualityMax) {
				maxQualityCoverPath = v.FilePath
				context["cover_image_path"] = v.FilePath
				t.App.Logger.Infof("✓ 最高质量封面已下载: %s", v.FilePath)
			}

			if t.StateManager.SaveUrlService != nil {
				// 旧 TbVideo 表在当前链路中可能未注入；封面本地路径已写入 context，不能因此中断任务。
				tbVideo := &models.TbVideo{
					Id:      t.StateManager.Id,
					VideoId: t.StateManager.VideoID,
					ImgURL:  cosKeyName,
					Status:  "img",
				}
				err := t.StateManager.UpdateTBVideo(tbVideo)
				if err != nil {
					t.App.Logger.Warnf("更新封面数据库记录失败: %v", err)
				}
			}
		} else {
			fmt.Printf("下载失败: %s - %s\n", k, v.ErrorMessage)
			failedMessages = append(failedMessages, fmt.Sprintf("%s: %s", k, v.ErrorMessage))
		}
	}

	// 如果没有下载到最高质量的封面，使用其他质量的封面
	if maxQualityCoverPath == "" {
		for _, v := range results {
			if v.Success {
				context["cover_image_path"] = v.FilePath
				t.App.Logger.Infof("✓ 备用质量封面已设置: %s", v.FilePath)
				break
			}
		}
	}
	if successCount == 0 {
		errorMsg := "封面下载失败"
		if len(failedMessages) > 0 {
			errorMsg += ": " + strings.Join(failedMessages, "; ")
		}
		context["error"] = errorMsg
		t.App.Logger.Errorf(errorMsg)
		return false
	}

	return true
}
