package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/handlers"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/services"
	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"github.com/ZhantaoLi/ytb2bili/pkg/logger"
	"github.com/ZhantaoLi/ytb2bili/pkg/store"
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"github.com/ZhantaoLi/ytb2bili/pkg/utils"
)

type metadataFetcher func() (*handlers.VideoMetadataInfo, error)

type refreshSummary struct {
	VideoID        string `json:"video_id"`
	Directory      string `json:"directory"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	GeneratedTitle string `json:"generated_title"`
	GeneratedDesc  string `json:"generated_desc"`
	GeneratedTags  string `json:"generated_tags"`
	MetaPath       string `json:"meta_path"`
	RefreshedAt    string `json:"refreshed_at"`
}

func main() {
	videoIDFlag := flag.String("video-id", "", "YouTube video id")
	urlFlag := flag.String("url", "", "YouTube video url; used to derive video id when -video-id is empty")
	configFlag := flag.String("config", "config.toml", "config file path")
	allowCloudMetadata := flag.Bool("allow-cloud-metadata", false, "allow configured cloud metadata provider")
	flag.Parse()

	videoID := strings.TrimSpace(*videoIDFlag)
	if videoID == "" && strings.TrimSpace(*urlFlag) != "" {
		videoID = utils.ExtractVideoID(*urlFlag)
	}
	if videoID == "" {
		exitWithError(errors.New("missing -video-id or valid -url"))
	}

	app, savedVideoService, err := bootstrap(*configFlag, *allowCloudMetadata)
	if err != nil {
		exitWithError(err)
	}

	savedVideo, err := savedVideoService.GetVideoByVideoID(videoID)
	if err != nil {
		exitWithError(fmt.Errorf("load saved video %s: %w", videoID, err))
	}
	if strings.TrimSpace(*urlFlag) != "" && savedVideo.URL == "" {
		savedVideo.URL = strings.TrimSpace(*urlFlag)
	}

	fileRoot, err := filepath.Abs(app.Config.FileUpDir)
	if err != nil {
		exitWithError(fmt.Errorf("resolve fileUpDir: %w", err))
	}
	stateManager := manager.NewStateManager(savedVideo.ID, savedVideo.VideoID, fileRoot, savedVideo.CreatedAt)
	fetcher := buildYtDlpFetcher(app, savedVideoService, stateManager)

	summary, err := refreshMetadata(app, savedVideoService, savedVideo, stateManager, fetcher)
	if err != nil {
		exitWithError(err)
	}

	encoded, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		exitWithError(err)
	}
	fmt.Println(string(encoded))
}

func bootstrap(configPath string, allowCloudMetadata bool) (*core.AppServer, *services.SavedVideoService, error) {
	config, err := types.LoadConfig(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	config.Path = configPath
	applyProxyFromEnvironment(config)
	if !allowCloudMetadata {
		disableCloudMetadata(config)
	}

	log, err := logger.NewLogger(config.Debug)
	if err != nil {
		return nil, nil, fmt.Errorf("init logger: %w", err)
	}
	db, err := store.NewDatabase(config)
	if err != nil {
		return nil, nil, fmt.Errorf("connect database: %w", err)
	}
	if err := store.MigrateDatabase(db); err != nil {
		return nil, nil, fmt.Errorf("migrate database: %w", err)
	}

	app := core.NewServer(config, log)
	app.Init(db)
	return app, services.NewSavedVideoService(db), nil
}

func refreshMetadata(app *core.AppServer, savedVideoService *services.SavedVideoService, savedVideo *model.SavedVideo, stateManager *manager.StateManager, fetch metadataFetcher) (*refreshSummary, error) {
	if app == nil || savedVideoService == nil || savedVideo == nil || stateManager == nil {
		return nil, errors.New("refresh metadata dependencies are incomplete")
	}
	if fetch == nil {
		return nil, errors.New("metadata fetcher is nil")
	}

	metadata, err := fetch()
	if err != nil {
		return nil, fmt.Errorf("fetch youtube metadata: %w", err)
	}
	if metadata == nil || strings.TrimSpace(metadata.Title) == "" {
		return nil, errors.New("fetched metadata has empty title")
	}

	current, err := savedVideoService.GetVideoByVideoID(savedVideo.VideoID)
	if err != nil {
		return nil, fmt.Errorf("reload saved video: %w", err)
	}
	current.Title = strings.TrimSpace(metadata.Title)
	current.Description = strings.TrimSpace(metadata.Description)
	if err := savedVideoService.UpdateVideo(current); err != nil {
		return nil, fmt.Errorf("save original metadata: %w", err)
	}

	task := handlers.NewGenerateMetadata("refresh_metadata", app, stateManager, nil, "", savedVideoService.DB, savedVideoService)
	context := map[string]interface{}{
		"video_id":          savedVideo.VideoID,
		"metadata_refresh":  true,
		"original_title":    current.Title,
		"original_desc":     current.Description,
		"started_at":        time.Now().Format(time.RFC3339),
		"upload_suppressed": true,
	}
	if ok := task.Execute(context); !ok {
		if value, exists := context["error"]; exists {
			return nil, fmt.Errorf("generate metadata: %v", value)
		}
		return nil, errors.New("generate metadata failed")
	}

	updated, err := savedVideoService.GetVideoByVideoID(savedVideo.VideoID)
	if err != nil {
		return nil, fmt.Errorf("reload refreshed video: %w", err)
	}
	return &refreshSummary{
		VideoID:        updated.VideoID,
		Directory:      stateManager.CurrentDir,
		Title:          updated.Title,
		Description:    updated.Description,
		GeneratedTitle: updated.GeneratedTitle,
		GeneratedDesc:  updated.GeneratedDesc,
		GeneratedTags:  updated.GeneratedTags,
		MetaPath:       filepath.Join(stateManager.CurrentDir, "meta.json"),
		RefreshedAt:    time.Now().Format(time.RFC3339),
	}, nil
}

func buildYtDlpFetcher(app *core.AppServer, savedVideoService *services.SavedVideoService, stateManager *manager.StateManager) metadataFetcher {
	return func() (*handlers.VideoMetadataInfo, error) {
		task := handlers.NewDownloadVideo("refresh_metadata_fetch", app, stateManager, nil, savedVideoService)
		ytdlpPath, err := task.FindYtDlp()
		if err != nil {
			return nil, err
		}
		return task.GetVideoMetadata(ytdlpPath)
	}
}

func disableCloudMetadata(config *types.AppConfig) {
	if config.DeepSeekTransConfig != nil {
		config.DeepSeekTransConfig.Enabled = false
	}
	if config.GeminiConfig != nil {
		config.GeminiConfig.Enabled = false
		config.GeminiConfig.AnalyzeVideo = false
		config.GeminiConfig.UseForMetadata = false
	}
	if config.OpenAICompatibleConfig != nil {
		config.OpenAICompatibleConfig.Enabled = false
	}
}

func applyProxyFromEnvironment(config *types.AppConfig) {
	proxyURL := firstNonEmpty(os.Getenv("HTTPS_PROXY"), os.Getenv("HTTP_PROXY"), os.Getenv("https_proxy"), os.Getenv("http_proxy"))
	if strings.TrimSpace(proxyURL) == "" {
		return
	}
	if config.ProxyConfig == nil {
		config.ProxyConfig = &types.ProxyConfig{}
	}
	config.ProxyConfig.UseProxy = true
	config.ProxyConfig.ProxyHost = proxyURL
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "refresh_metadata failed: %v\n", err)
	os.Exit(1)
}
