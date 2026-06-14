package handlers

import (
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/base"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/services"
	"github.com/ZhantaoLi/ytb2bili/internal/storage"
	"github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/ZhantaoLi/ytb2bili/pkg/cos"
	"os"
	"path/filepath"
)

type UploadSubtitleToBilibili struct {
	base.BaseTask
	App               *core.AppServer
	SavedVideoService *services.SavedVideoService
}

func NewUploadSubtitleToBilibili(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, savedVideoService *services.SavedVideoService) *UploadSubtitleToBilibili {
	return &UploadSubtitleToBilibili{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:               app,
		SavedVideoService: savedVideoService,
	}
}

func (t *UploadSubtitleToBilibili) Execute(context map[string]interface{}) bool {
	t.App.Logger.Info("========================================")
	t.App.Logger.Info("开始上传字幕到 Bilibili")
	t.App.Logger.Info("========================================")

	// 1. 检查是否有BVID（视频已上传成功）
	bvid, exists := context["bili_bvid"].(string)
	if !exists || bvid == "" {
		// 尝试从数据库获取BVID
		savedVideo, err := t.SavedVideoService.GetVideoByVideoID(t.StateManager.VideoID)
		if err != nil || savedVideo.BiliBVID == "" {
			t.App.Logger.Warn("⚠️  没有找到BVID，跳过字幕上传")
			return true // 不算失败，只是跳过
		}
		bvid = savedVideo.BiliBVID
	}

	t.App.Logger.Infof("📺 视频BVID: %s", bvid)

	// 2. 检查登录信息
	loginStore := storage.GetDefaultStore()
	if !loginStore.IsValid() {
		t.App.Logger.Error("❌ 没有有效的 Bilibili 登录信息，无法上传字幕")
		context["error"] = "未登录 Bilibili"
		return false
	}

	loginInfo, err := loginStore.Load()
	if err != nil {
		t.App.Logger.Errorf("❌ 加载登录信息失败: %v", err)
		context["error"] = "加载登录信息失败"
		return false
	}

	// 3. 查找字幕文件
	subtitleFiles := t.findSubtitleFiles()
	if len(subtitleFiles) == 0 {
		t.App.Logger.Warn("⚠️  未找到字幕文件，跳过字幕上传")
		return true // 不算失败，只是跳过
	}

	// 4. 创建 Bilibili 客户端和字幕上传器
	client := bilibili.NewClient()
	uploader := bilibili.NewSubtitleUploader(client, loginInfo)

	// 5. 上传字幕文件
	uploadedCount := 0
	for _, subtitleFile := range subtitleFiles {
		t.App.Logger.Infof("📝 正在上传字幕: %s", filepath.Base(subtitleFile.Path))

		err := uploader.UploadSubtitle(bvid, subtitleFile.Path, subtitleFile.Language)
		if err != nil {
			t.App.Logger.Errorf("❌ 上传字幕失败 %s: %v", subtitleFile.Path, err)
			// 继续上传其他字幕文件，不因为一个失败就停止
			continue
		}

		t.App.Logger.Infof("✅ 字幕上传成功: %s (%s)", filepath.Base(subtitleFile.Path), subtitleFile.Language)
		uploadedCount++
	}

	// 6. 记录结果
	if uploadedCount > 0 {
		t.App.Logger.Info("========================================")
		t.App.Logger.Infof("✅ 字幕上传完成！成功上传 %d 个字幕文件", uploadedCount)
		t.App.Logger.Infof("  视频链接: https://www.bilibili.com/video/%s", bvid)
		t.App.Logger.Info("========================================")

		context["subtitle_upload_count"] = uploadedCount
		return true
	} else {
		t.App.Logger.Error("❌ 没有成功上传任何字幕文件")
		context["error"] = "字幕上传失败"
		return false
	}
}

// SubtitleFileInfo 字幕文件信息
type SubtitleFileInfo struct {
	Path     string
	Language string
}

// findSubtitleFiles 查找字幕文件
func (t *UploadSubtitleToBilibili) findSubtitleFiles() []SubtitleFileInfo {
	var subtitleFiles []SubtitleFileInfo

	// 检查常见的字幕文件
	subtitleFilesToCheck := []struct {
		filename string
		language string
	}{
		{"zh_optimized.srt", "zh-Hans"}, // 中文简体
		{"en.srt", "en"},                // 英文
		//{"zh-cn.srt", "zh-Hans"}, // 中文简体
		//{"zh-tw.srt", "zh-Hant"}, // 中文繁体
		//{"ja.srt", "ja"},         // 日文
		//{"ko.srt", "ko"},         // 韩文
	}

	for _, item := range subtitleFilesToCheck {
		fullPath := filepath.Join(t.StateManager.CurrentDir, item.filename)
		if _, err := os.Stat(fullPath); err == nil {
			subtitleFiles = append(subtitleFiles, SubtitleFileInfo{
				Path:     fullPath,
				Language: item.language,
			})
			t.App.Logger.Infof("🎯 找到字幕文件: %s (%s)", item.filename, item.language)
		}
	}

	return subtitleFiles
}
