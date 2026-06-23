//go:build demo
// +build demo

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/handlers"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/services"
	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"github.com/ZhantaoLi/ytb2bili/internal/storage"
	"github.com/ZhantaoLi/ytb2bili/pkg/store"
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
)

//  ./bin/test_handler_upload -video ./data/001.mp4 -id fT6kGrHtf9k -login login_info.json

func main() {
	// 定义命令行参数
	videoPath := flag.String("video", "", "视频文件路径 (必填)")
	videoID := flag.String("id", "test_video_id", "视频ID (用于数据库查找/创建)")
	configPath := flag.String("config", "config.toml", "配置文件路径")
	loginFile := flag.String("login", "login_info.json", "登录信息文件路径")
	flag.Parse()

	// 检查参数
	if *videoPath == "" {
		fmt.Println("❌ 请提供视频文件路径")
		fmt.Println("用法示例: go run bin/test_handler_upload.go -video ./downloads/video.mp4 -id my_video_001")
		os.Exit(1)
	}

	absVideoPath, err := filepath.Abs(*videoPath)
	if err != nil {
		fmt.Printf("❌ 获取视频绝对路径失败: %v\n", err)
		os.Exit(1)
	}
	videoDir := filepath.Dir(absVideoPath)

	// 1. 初始化日志
	rawLogger, _ := zap.NewDevelopment()
	logger := rawLogger.Sugar()

	// 2. 加载配置
	logger.Infof("正在加载配置文件: %s", *configPath)
	// 这里我们需要手动加载配置，因为 core.AppServer 通常由 main.go 初始化
	// 假设有一个 ConfigLoader 或者直接构造
	// 由于项目结构，我们尝试简单解析或构造默认值
	// 注意：core.LoadConfig 可能不可用或需要 specific path，这里我们简单模拟
	// 如果 core 包有 LoadConfig 方法最好，否则手动构造
	// 查阅之前的 context， config.toml 存在。
	// 这里为了简化，我们尝试构造一个基础配置对象，因为 handlers 需要 AppServer.Config

	// 尝试读取真实的配置，如果失败则使用默认
	// 假设 core.LoadConfig 存在，但我们没有看过它的签名。
	// 替代方案：直接读取 toml 或者构造 dummy Config

	appConfig := &types.AppConfig{
		Database: types.Database{
			Type:     "mysql",
			Host:     "localhost",
			Port:     3306,
			Username: "root",
			Password: "Ab123456", // 注意：这里硬编码了，如果本地不同可能连接失败
			Database: "bili_up",
		},
		BilibiliConfig: &types.BilibiliConfig{
			UseOriginalTitle: true,
			UseOriginalDesc:  true,
		},
	}

	// 尝试覆盖配置（实际生产中应该解析 toml）
	// 这里我们直接连接数据库

	logger.Info("正在连接数据库...")
	db, err := store.NewDatabase(appConfig)
	if err != nil {
		logger.Warnf("⚠️ 数据库连接失败: %v", err)
		logger.Warn("⚠️ 将无法读取/保存视频元数据，可能会导致 handler 报错")
		// 在这种情况下，我们可能无法继续，因为 Handler 强依赖 SavedVideoService
		// 我们可以询问是否继续? 或者直接退出
		// 为了测试方便，如果连不上，也许我们需要 mock？
		// 但为了简单，假设为了测试 handler，必须有环境
		logger.Info("尝试使用 sqlite 作为备选? 不支持。")
	} else {
		logger.Info("✅ 数据库连接成功")
	}

	// 3. 初始化 Service
	savedVideoService := services.NewSavedVideoService(db)

	// 4. 确保数据库中有该视频记录 (Mock Data)
	if db != nil {
		_, err := savedVideoService.GetVideoByVideoID(*videoID)
		if err != nil {
			logger.Infof("未找到视频记录 %s，正在创建临时记录...", *videoID)
			newVideo := &model.SavedVideo{
				VideoID:     *videoID,
				Title:       fmt.Sprintf("测试视频 %s", *videoID),
				Description: "这是一个用于测试 UploadToBilibili Handler 的视频描述。\n包含多行文本。\n测试结束。",
				Status:      "001",
				URL:         "https://www.youtube.com/watch?v=dQw4w9WgXcQ", // Dummy
			}
			if err := savedVideoService.CreateVideo(newVideo); err != nil {
				logger.Errorf("❌ 创建临时视频记录失败: %v", err)
				os.Exit(1)
			}
			logger.Info("✅ 临时视频记录已创建")
		}
	}

	// 5. 构造 AppServer (Mock)
	appServer := &core.AppServer{
		Config: appConfig,
		Logger: logger,
		DB:     db,
	}

	// 6. 构造 StateManager
	stateManager := &manager.StateManager{
		VideoID:    *videoID,
		CurrentDir: videoDir,
		// 其他字段根据handler需要可能要填充，但主要用到的是 VideoID 和 CurrentDir (在 findVideoFiles 中使用)
	}

	// 7. 构造 Handler
	handler := handlers.NewUploadToBilibili("UploadTask", appServer, stateManager, nil, savedVideoService)

	// 7.1 初始化并注入 LoginStore
	storePath := *loginFile
	// 如果默认路径不存在，尝试查找
	if storePath == "login_info.json" {
		if _, err := os.Stat(storePath); os.IsNotExist(err) {
			// 尝试 ~/.bili_up/login.json
			homeDir, _ := os.UserHomeDir()
			defaultSysPath := filepath.Join(homeDir, ".bili_up", "login.json")
			if _, err := os.Stat(defaultSysPath); err == nil {
				storePath = defaultSysPath
			}
		}
	}

	logger.Infof("使用登录信息文件: %s", storePath)
	loginStore := storage.NewLoginStore(storePath)
	if !loginStore.IsValid() {
		logger.Warnf("⚠️ 登录信息无效或文件不存在: %s", storePath)
		logger.Warn("⚠️ 请确保已登录B站，或使用 -login 指定有效的 login.json 文件")
		// 也许我们不应该退出，让 handler 自身去报错，或者在这里就退出?
		// handler 内部也会检查，但为了明确提示用户:
		// logger.Error("无法继续: 需提供有效登录凭证")
		// os.Exit(1)
		// 既然是测试工具，暂时不强退，看handler反应
	}
	handler.LoginStore = loginStore

	// 8. 执行
	context := make(map[string]interface{})
	// 如果有封面，可以在这里通过 context 传入，或者 args
	// context["cover_image_path"] = "/path/to/cover.jpg"

	logger.Info("🚀 开始执行 UploadToBilibili Handler...")
	success := handler.Execute(context)

	if success {
		logger.Info("🎉 Handler 执行成功！")
	} else {
		logger.Errorf("❌ Handler 执行失败: %v", context["error"])
		os.Exit(1)
	}
}
