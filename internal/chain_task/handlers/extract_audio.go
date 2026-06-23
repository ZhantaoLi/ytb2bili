package handlers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/base"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/pkg/cos"
	"github.com/ZhantaoLi/ytb2bili/pkg/utils"
	"gorm.io/gorm"
)

type ExtractAudio struct {
	base.BaseTask
	App *core.AppServer
	DB  *gorm.DB
}

func NewExtractAudio(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient) *ExtractAudio {
	return &ExtractAudio{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App: app,
	}
}

func (t *ExtractAudio) Execute(context map[string]interface{}) bool {
	sourcePath := t.StateManager.InputVideoPath
	if downloadedFile, ok := context["downloaded_file"].(string); ok && downloadedFile != "" {
		sourcePath = downloadedFile
	}

	if _, err := os.Stat(sourcePath); err != nil {
		context["error"] = fmt.Sprintf("source video not found for audio extraction: %v", err)
		return false
	}

	if err := os.MkdirAll(filepath.Dir(t.StateManager.OriginalMP3), 0755); err != nil {
		context["error"] = fmt.Sprintf("create audio output directory failed: %v", err)
		return false
	}

	if err := utils.ExtractAudio(sourcePath, t.StateManager.OriginalMP3); err != nil {
		context["error"] = fmt.Sprintf("extract audio failed: %v", err)
		return false
	}

	return true
}
