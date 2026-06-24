package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/base"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"github.com/ZhantaoLi/ytb2bili/pkg/utils"
	"go.uber.org/zap"
)

func TestDownloadImgHandlerContinuesWhenAllThumbnailDownloadsFail(t *testing.T) {
	originalDownloader := downloadYouTubeThumbnail
	defer func() { downloadYouTubeThumbnail = originalDownloader }()

	var gotProxyURL string
	downloadYouTubeThumbnail = func(videoID string, quality utils.QualityInput, opt utils.DownloadOptions, filename string) interface{} {
		gotProxyURL = opt.ProxyURL
		return map[string]utils.DownloadResult{
			string(utils.QualityMax): {
				Success:      false,
				Quality:      string(utils.QualityMax),
				ErrorMessage: "timeout",
			},
			string(utils.QualityStandard): {
				Success:      false,
				Quality:      string(utils.QualityStandard),
				ErrorMessage: "timeout",
			},
		}
	}

	task := &DownloadImgHandler{
		BaseTask: base.BaseTask{
			Name: "下载封面",
			StateManager: &manager.StateManager{
				VideoID:    "video-1",
				CurrentDir: t.TempDir(),
			},
		},
		App: &core.AppServer{
			Config: &types.AppConfig{
				ProxyConfig: &types.ProxyConfig{
					UseProxy:  true,
					ProxyHost: "http://127.0.0.1:10809",
				},
			},
			Logger: zap.NewNop().Sugar(),
		},
	}

	context := map[string]interface{}{}
	if ok := task.Execute(context); !ok {
		t.Fatalf("Execute() = false, want true because cover download is non-blocking; context=%v", context)
	}
	if gotProxyURL != "http://127.0.0.1:10809" {
		t.Fatalf("proxy URL = %q, want configured proxy", gotProxyURL)
	}
	if _, exists := context["error"]; exists {
		t.Fatalf("context error should not be set for non-blocking cover failure, got %v", context["error"])
	}
	warningText, _ := context["cover_download_error"].(string)
	if !strings.Contains(warningText, "封面下载失败") {
		t.Fatalf("cover_download_error = %q, want cover download failure", warningText)
	}
}

func TestDownloadImgHandlerSucceedsWithoutLegacyTBVideoService(t *testing.T) {
	originalDownloader := downloadYouTubeThumbnail
	defer func() { downloadYouTubeThumbnail = originalDownloader }()

	coverPath := filepath.Join(t.TempDir(), "maxresdefault.jpg")
	if err := os.WriteFile(coverPath, []byte("jpg"), 0644); err != nil {
		t.Fatalf("write cover: %v", err)
	}

	downloadYouTubeThumbnail = func(videoID string, quality utils.QualityInput, opt utils.DownloadOptions, filename string) interface{} {
		return map[string]utils.DownloadResult{
			string(utils.QualityMax): {
				Success:  true,
				FilePath: coverPath,
				Quality:  string(utils.QualityMax),
				FileSize: 3,
			},
			string(utils.QualityStandard): {
				Success:      false,
				Quality:      string(utils.QualityStandard),
				ErrorMessage: "not needed",
			},
		}
	}

	task := &DownloadImgHandler{
		BaseTask: base.BaseTask{
			Name: "下载封面",
			StateManager: &manager.StateManager{
				VideoID:    "video-1",
				CurrentDir: t.TempDir(),
			},
		},
		App: &core.AppServer{
			Config: types.NewDefaultConfig(),
			Logger: zap.NewNop().Sugar(),
		},
	}

	context := map[string]interface{}{}
	if ok := task.Execute(context); !ok {
		t.Fatalf("Execute() = false, want true; context=%v", context)
	}
	if context["cover_image_path"] != coverPath {
		t.Fatalf("cover_image_path = %v, want %s", context["cover_image_path"], coverPath)
	}
}
