package main

import (
	"context"
	"github.com/difyz9/ytb2bili/internal/chain_task"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/difyz9/ytb2bili/internal/handler"
	"github.com/difyz9/ytb2bili/internal/web"
	"github.com/difyz9/ytb2bili/pkg/analytics"
	"github.com/difyz9/ytb2bili/pkg/auth"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/logger"
	biliAccountService "github.com/difyz9/ytb2bili/pkg/services"
	"github.com/difyz9/ytb2bili/pkg/store"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// # Unix/Linux/Mac
// openssl rand -base64 32

// AppLifecycle 应用程序生命周期
type AppLifecycle struct {
}

// OnStart 应用程序启动时执行
func (l *AppLifecycle) OnStart(context.Context) error {
	log.Println("AppLifecycle OnStart")
	return nil
}

// OnStop 应用程序停止时执行
func (l *AppLifecycle) OnStop(context.Context) error {
	log.Println("AppLifecycle OnStop")
	return nil
}

func main() {

	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.toml"
	}

	// 加载配置
	config, err := types.LoadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}
	config.Path = configFile

	app := fx.New(
		// 初始化配置应用配置
		fx.Provide(func() *types.AppConfig {
			return config
		}),

		// 日志模块
		fx.Provide(func(config *types.AppConfig) (*zap.SugaredLogger, error) {
			return logger.NewLogger(config.Debug)
		}),

		// 数据库模块
		fx.Provide(store.NewDatabase),

		// 核心模块
		fx.Provide(core.NewServer),
		fx.Provide(cos.NewCosClient),

		// 分析客户端
		fx.Provide(func(config *types.AppConfig, logger *zap.SugaredLogger) (*analytics.Client, error) {
			if config.AnalyticsConfig == nil || !config.AnalyticsConfig.Enabled {
				logger.Info("Analytics is disabled")
				return nil, nil
			}

			analyticsConfig := &analytics.Config{
				ServerURL:     config.AnalyticsConfig.ServerURL,
				APIKey:        config.AnalyticsConfig.APIKey,
				ProductID:     config.AnalyticsConfig.ProductID,
				Debug:         config.AnalyticsConfig.Debug,
				EncryptionKey: config.AnalyticsConfig.EncryptionKey,
			}

			return analytics.NewClient(analyticsConfig, logger)
		}),

		// 分析中间件
		fx.Provide(func(client *analytics.Client, logger *zap.SugaredLogger) *analytics.Middleware {
			return analytics.NewMiddleware(client, logger)
		}),

		// API 认证中间件
		fx.Provide(func(config *types.AppConfig, logger *zap.SugaredLogger) *auth.Middleware {
			// 如果配置了 AppID 和 AppSecret，启用认证
			if config.APIAuth.AppID != "" && config.APIAuth.AppSecret != "" {
				authConfig := &auth.Config{
					Apps: map[string]string{
						config.APIAuth.AppID: config.APIAuth.AppSecret,
					},
				}
				logger.Infof("API Auth middleware enabled for app: %s", config.APIAuth.AppID)
				return auth.NewMiddleware(authConfig, logger)
			}
			logger.Info("API Auth middleware disabled (no credentials configured)")
			return auth.NewMiddleware(nil, logger)
		}),

		// 服务层
		fx.Provide(services.NewVideoService),
		fx.Provide(services.NewSavedVideoService),
		fx.Provide(services.NewTaskStepService),
		fx.Provide(biliAccountService.NewBilibiliAccountService),

		// 注册cron
		fx.Provide(func() *cron.Cron {
			return cron.New(cron.WithSeconds())
		}),

		// fx.Provide(handler.NewCronHandler),
		// fx.Invoke(func(h *handler.CronHandler) {
		// 	h.SetUp()
		// }),

		// 生命周期管理
		fx.Provide(func() *AppLifecycle {
			return &AppLifecycle{}
		}),

		// 初始化数据库
		fx.Invoke(func(db *gorm.DB, logger *zap.SugaredLogger) error {
			logger.Info("Running database migrations...")
			return store.MigrateDatabase(db)
		}),

		fx.Provide(chain_task.NewChainTaskHandler),
		fx.Invoke(func(h *chain_task.ChainTaskHandler) {
			// 设置并启动任务消费者（准备阶段：下载、字幕、翻译、元数据）
			h.SetUp()
		}),

		// 添加上传调度器
		fx.Provide(chain_task.NewUploadScheduler),
		fx.Invoke(func(s *chain_task.UploadScheduler) {
			// 设置并启动上传调度器（上传阶段：每小时上传视频，1小时后上传字幕）
			s.SetUp()
		}),

		// 初始化应用服务器
		fx.Invoke(func(server *core.AppServer, db *gorm.DB) {
			server.Init(db)
		}),

		// 添加分析中间件
		fx.Invoke(func(server *core.AppServer, analyticsMiddleware *analytics.Middleware, logger *zap.SugaredLogger) {
			if analyticsMiddleware != nil {
				server.Engine.Use(analyticsMiddleware.Handler())
				logger.Info("Analytics middleware registered")
			}
		}),

		// 注册 Handlers
		fx.Provide(handler.NewAuthHandler),
		fx.Invoke(func(h *handler.AuthHandler, server *core.AppServer, logger *zap.SugaredLogger) {
			h.RegisterRoutes(server)
			logger.Info("✓ Auth routes registered")
		}),

		fx.Provide(handler.NewUploadHandler),
		fx.Invoke(func(h *handler.UploadHandler, server *core.AppServer, logger *zap.SugaredLogger) {
			h.RegisterRoutes(server)
			logger.Info("✓ Upload routes registered")
		}),

		fx.Provide(handler.NewCategoryHandler),
		fx.Invoke(func(h *handler.CategoryHandler, server *core.AppServer, logger *zap.SugaredLogger) {
			h.RegisterRoutes(server)
			logger.Info("✓ Category routes registered")
		}),

		fx.Provide(handler.NewSubtitleHandler),
		fx.Invoke(func(
			h *handler.SubtitleHandler,
			server *core.AppServer,
			authMiddleware *auth.Middleware,
			appConfig *types.AppConfig,
			logger *zap.SugaredLogger,
		) {
			if authMiddleware.IsEnabled() {
				// 获取 cookies 解密密钥
				decryptKey := appConfig.APIAuth.CookiesDecryptKey
				if decryptKey == "" {
					h.RegisterRoutesWithAuth(server, authMiddleware, "")
					logger.Info("Subtitle routes registered with auth only (cookies decrypt key not configured)")
					return
				}
				h.RegisterRoutesWithAuth(server, authMiddleware, decryptKey)
				logger.Info("✓ Subtitle routes registered with auth and decrypt middleware")
			} else {
				h.RegisterRoutes(server)
				logger.Info("✓ Subtitle routes registered (auth disabled)")
			}
		}),

		fx.Provide(handler.NewConfigHandler),
		fx.Invoke(func(h *handler.ConfigHandler, server *core.AppServer, logger *zap.SugaredLogger) {
			h.RegisterRoutes(server)
			logger.Info("✓ Config routes registered")
		}),

		fx.Provide(handler.NewAccountsHandler),
		fx.Invoke(func(h *handler.AccountsHandler, server *core.AppServer, logger *zap.SugaredLogger) {
			h.RegisterRoutes(server.Engine.Group("/api/v1/accounts"))
			logger.Info("✓ Accounts routes registered")
		}),

		fx.Provide(handler.NewAnalyticsHandler),
		fx.Provide(handler.NewVideoHandler),
		fx.Invoke(func(
			h *handler.VideoHandler,
			server *core.AppServer,
			uploadScheduler *chain_task.UploadScheduler,
			analyticsHandler *handler.AnalyticsHandler,
			logger *zap.SugaredLogger,
		) {
			h.AnalyticsHandler = analyticsHandler
			h.SetUploadScheduler(uploadScheduler)
			h.RegisterRoutes(server.Engine.Group("/api/v1"))
			logger.Info("✓ Video routes registered")
		}),

		// 健康检查和静态文件服务
		fx.Invoke(func(server *core.AppServer, logger *zap.SugaredLogger) {
			// 健康检查
			server.Engine.GET("/health", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"status":  "ok",
					"message": "Bili Up Backend API is running",
					"time":    time.Now().Format(time.RFC3339),
				})
			})

			// 静态文件服务 (嵌入的前端文件)
			logger.Info("Setting up embedded static file server...")
			staticHandler := web.StaticFileHandler()

			// 对于根路径和非 API 路径，提供静态文件
			server.Engine.NoRoute(func(c *gin.Context) {
				path := c.Request.URL.Path
				// 如果不是 API 路径，提供静态文件
				if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/health") {
					staticHandler.ServeHTTP(c.Writer, c.Request)
					return
				}
				// 否则返回 404
				c.JSON(404, gin.H{
					"code":    404,
					"message": "API endpoint not found",
				})
			})

			logger.Info("✓ Static file server configured")
		}),

		fx.Invoke(func(s *core.AppServer, db *gorm.DB) {
			go func() {
				err := s.Run()
				if err != nil {
					os.Exit(0)
				}
			}()
		}),
		// 注册生命周期回调函数
		fx.Invoke(func(lifecycle fx.Lifecycle, lc *AppLifecycle) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					return lc.OnStart(ctx)
				},
				OnStop: func(ctx context.Context) error {
					return lc.OnStop(ctx)
				},
			})
		}),
	)

	// 启动应用程序
	go func() {

		if err := app.Start(context.Background()); err != nil {
			log.Fatal(err)
		}

	}()

	// 监听退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down gracefully...")

	// 关闭应用程序
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Stop(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("✅ Application stopped")

}
