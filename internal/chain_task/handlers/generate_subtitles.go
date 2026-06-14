package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/difyz9/ytb2bili/pkg/utils"
)

type GenerateSubtitles struct {
	base.BaseTask
	App               *core.AppServer
	SavedVideoService *services.SavedVideoService
}

func NewGenerateSubtitles(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, savedVideoService *services.SavedVideoService) *GenerateSubtitles {
	return &GenerateSubtitles{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:               app,
		SavedVideoService: savedVideoService,
	}
}

// formatTime 将秒数转换为 SRT 时间格式 (HH:MM:SS,mmm)
func (t *GenerateSubtitles) formatTime(seconds float64) string {
	hours := int(seconds / 3600)
	minutes := int((seconds - float64(hours*3600)) / 60)
	secs := int(seconds - float64(hours*3600) - float64(minutes*60))
	milliseconds := int((seconds - float64(int(seconds))) * 1000)

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, milliseconds)
}

// generateSRT 生成 SRT 格式字幕内容
func (t *GenerateSubtitles) generateSRT(subtitles []model.SavedVideoSubtitle) string {
	var srtContent strings.Builder

	for i, subtitle := range subtitles {
		// SRT 序号（从1开始）
		srtContent.WriteString(fmt.Sprintf("%d\n", i+1))

		// 时间轴
		startTime := t.formatTime(subtitle.Offset)
		endTime := t.formatTime(subtitle.Offset + subtitle.Duration)
		srtContent.WriteString(fmt.Sprintf("%s --> %s\n", startTime, endTime))

		// 字幕文本
		srtContent.WriteString(subtitle.Text)
		srtContent.WriteString("\n\n")
	}

	return srtContent.String()
}

func (t *GenerateSubtitles) Execute(context map[string]interface{}) bool {
	t.App.Logger.Info("========================================")
	t.App.Logger.Info("开始生成字幕文件")
	t.App.Logger.Info("========================================")

	// 1. 从数据库读取视频信息
	savedVideo, err := t.SavedVideoService.GetVideoByID(t.StateManager.Id)
	if err != nil {
		t.App.Logger.Errorf("❌ 查询视频信息失败: %v", err)
		context["error"] = err.Error()
		return false
	}

	if savedVideo == nil {
		errMsg := "视频信息不存在"
		t.App.Logger.Error("❌ " + errMsg)
		context["error"] = errMsg
		return false
	}

	// 2. 检查字幕数据是否存在
	if isEmptySubtitleJSON(savedVideo.Subtitles) {
		t.App.Logger.Warn("⚠️  视频没有字幕数据，尝试 MiMo ASR 生成字幕")
		return t.generateSubtitlesWithMimoASR(context)
	}

	// 3. 解析字幕 JSON 数据
	var subtitles []model.SavedVideoSubtitle
	if err := json.Unmarshal([]byte(savedVideo.Subtitles), &subtitles); err != nil {
		t.App.Logger.Errorf("❌ 解析字幕数据失败: %v", err)
		context["error"] = fmt.Sprintf("解析字幕数据失败: %v", err)
		return false
	}

	if len(subtitles) == 0 {
		t.App.Logger.Warn("⚠️  字幕数据为空，尝试 MiMo ASR 生成字幕")
		return t.generateSubtitlesWithMimoASR(context)
	}

	t.App.Logger.Infof("📝 找到 %d 条字幕", len(subtitles))

	// 4. 生成 SRT 内容
	srtContent := t.generateSRT(subtitles)

	// 5. 确保输出目录存在
	if err := os.MkdirAll(t.StateManager.CurrentDir, 0755); err != nil {
		t.App.Logger.Errorf("❌ 创建字幕目录失败: %v", err)
		context["error"] = err.Error()
		return false
	}

	// 6. 生成字幕文件路径
	srtFileName := fmt.Sprintf("%s.srt", t.StateManager.VideoID)
	srtFilePath := filepath.Join(t.StateManager.CurrentDir, srtFileName)

	// 7. 写入 SRT 文件
	if err := os.WriteFile(srtFilePath, []byte(srtContent), 0644); err != nil {
		t.App.Logger.Errorf("❌ 写入字幕文件失败: %v", err)
		context["error"] = fmt.Sprintf("写入字幕文件失败: %v", err)
		return false
	}

	// 8. 验证文件是否创建成功
	if _, err := os.Stat(srtFilePath); os.IsNotExist(err) {
		errMsg := "字幕文件创建失败"
		t.App.Logger.Error("❌ " + errMsg)
		context["error"] = errMsg
		return false
	}

	enSrtFileName := fmt.Sprintf("%s.srt", "en")
	enSrtFilePath := filepath.Join(t.StateManager.CurrentDir, enSrtFileName)

	if err := utils.CopyFile(srtFilePath, enSrtFilePath); err != nil {
		t.App.Logger.Errorf("❌ 复制英文字幕文件失败: %v", err)
		context["error"] = fmt.Sprintf("复制英文字幕文件失败: %v", err)
	}

	// 9. 保存字幕文件路径到 context，供后续任务使用
	context["subtitle_file"] = srtFilePath
	context["subtitle_count"] = len(subtitles)

	// 10. 显示字幕预览（前3条）
	previewCount := 3
	if len(subtitles) < previewCount {
		previewCount = len(subtitles)
	}
	t.App.Logger.Info("📋 字幕预览（前3条）：")
	for i := 0; i < previewCount; i++ {
		sub := subtitles[i]
		t.App.Logger.Infof("  [%d] %.2fs-%.2fs: %s",
			i+1,
			sub.Offset,
			sub.Offset+sub.Duration,
			truncateString(sub.Text, 50))
	}

	t.App.Logger.Infof("✓ 字幕文件生成成功: %s", srtFilePath)
	t.App.Logger.Infof("✓ 共生成 %d 条字幕", len(subtitles))
	t.App.Logger.Info("========================================")

	return true
}

func isEmptySubtitleJSON(subtitleJSON string) bool {
	trimmed := strings.TrimSpace(subtitleJSON)
	return trimmed == "" || trimmed == "null" || trimmed == "[]"
}

func (t *GenerateSubtitles) generateSubtitlesWithMimoASR(context map[string]interface{}) bool {
	config := t.App.Config.MimoASRConfig
	if config == nil || !config.Enabled || strings.TrimSpace(config.APIKey) == "" {
		t.App.Logger.Warn("⚠️  MiMo ASR 未启用或未配置 API Key，跳过字幕生成")
		context["subtitle_skipped"] = "mimo_asr_not_configured"
		return true
	}

	videoPath, err := t.findSourceVideo()
	if err != nil {
		t.App.Logger.Errorf("❌ 查找待识别视频失败: %v", err)
		context["error"] = err.Error()
		return false
	}

	segmentSeconds := config.SegmentSeconds
	if segmentSeconds <= 0 {
		segmentSeconds = 90
	}

	segmentPaths, err := t.splitVideoForASR(videoPath, segmentSeconds)
	if err != nil {
		t.App.Logger.Errorf("❌ 切分 ASR 音频失败: %v", err)
		context["error"] = err.Error()
		return false
	}

	language := strings.TrimSpace(config.Language)
	if language == "" {
		language = "auto"
	}
	client := NewMimoASRClient(MimoASRClientConfig{
		APIKey:  config.APIKey,
		BaseURL: config.BaseURL,
		Model:   config.Model,
		Timeout: config.Timeout,
	})

	asrSegments := make([]asrSegment, 0, len(segmentPaths))
	for i, segmentPath := range segmentPaths {
		t.App.Logger.Infof("🎙️  MiMo ASR 转写片段 %d/%d: %s", i+1, len(segmentPaths), filepath.Base(segmentPath))
		text, err := client.TranscribeFile(segmentPath, language)
		if err != nil {
			t.App.Logger.Errorf("❌ MiMo ASR 转写失败: %v", err)
			context["error"] = err.Error()
			return false
		}
		duration := probeAudioDuration(segmentPath)
		if duration <= 0 {
			duration = float64(segmentSeconds)
		}
		asrSegments = append(asrSegments, asrSegment{
			Start:    float64(i * segmentSeconds),
			Duration: duration,
			Text:     text,
		})
	}

	srtContent := buildSRTFromASRSegments(asrSegments)
	if strings.TrimSpace(srtContent) == "" {
		errMsg := "MiMo ASR 未返回可写入字幕的文本"
		t.App.Logger.Error("❌ " + errMsg)
		context["error"] = errMsg
		return false
	}

	srtFilePath, err := t.writeSubtitleFiles(srtContent)
	if err != nil {
		t.App.Logger.Errorf("❌ 写入 ASR 字幕文件失败: %v", err)
		context["error"] = err.Error()
		return false
	}

	context["subtitle_file"] = srtFilePath
	context["subtitle_count"] = len(asrSegments)
	context["asr_provider"] = "mimo"
	context["asr_segments"] = len(segmentPaths)
	t.App.Logger.Infof("✅ MiMo ASR 字幕生成成功: %s", srtFilePath)
	return true
}

func (t *GenerateSubtitles) findSourceVideo() (string, error) {
	candidates := []string{
		t.StateManager.InputVideoPath,
		filepath.Join(t.StateManager.CurrentDir, t.StateManager.VideoID+".mp4"),
	}
	for _, path := range candidates {
		if fileInfo, err := os.Stat(path); err == nil && !fileInfo.IsDir() && fileInfo.Size() > 0 {
			return path, nil
		}
	}

	for _, pattern := range []string{"*.mp4", "*.webm", "*.mkv", "*.flv"} {
		matches, err := filepath.Glob(filepath.Join(t.StateManager.CurrentDir, pattern))
		if err != nil {
			return "", err
		}
		sort.Strings(matches)
		for _, path := range matches {
			if fileInfo, err := os.Stat(path); err == nil && !fileInfo.IsDir() && fileInfo.Size() > 0 {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("未找到可用于 ASR 的视频文件: %s", t.StateManager.CurrentDir)
}

func (t *GenerateSubtitles) splitVideoForASR(videoPath string, segmentSeconds int) ([]string, error) {
	segmentsDir := filepath.Join(t.StateManager.CurrentDir, "asr_segments", fmt.Sprintf("%d", os.Getpid()))
	if err := os.MkdirAll(segmentsDir, 0755); err != nil {
		return nil, err
	}

	outputPattern := filepath.Join(segmentsDir, "segment_%04d.mp3")
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", videoPath,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-b:a", "64k",
		"-f", "segment",
		"-segment_time", strconv.Itoa(segmentSeconds),
		"-reset_timestamps", "1",
		outputPattern,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg 切分失败: %v, output=%s", err, strings.TrimSpace(string(output)))
	}

	segments, err := filepath.Glob(filepath.Join(segmentsDir, "segment_*.mp3"))
	if err != nil {
		return nil, err
	}
	sort.Strings(segments)
	if len(segments) == 0 {
		return nil, fmt.Errorf("ffmpeg 未生成 ASR 音频片段")
	}
	return segments, nil
}

func (t *GenerateSubtitles) writeSubtitleFiles(srtContent string) (string, error) {
	if err := os.MkdirAll(t.StateManager.CurrentDir, 0755); err != nil {
		return "", err
	}

	srtFilePath := filepath.Join(t.StateManager.CurrentDir, fmt.Sprintf("%s.srt", t.StateManager.VideoID))
	if err := os.WriteFile(srtFilePath, []byte(srtContent), 0644); err != nil {
		return "", err
	}

	enSrtFilePath := filepath.Join(t.StateManager.CurrentDir, "en.srt")
	if err := utils.CopyFile(srtFilePath, enSrtFilePath); err != nil {
		return "", err
	}

	return srtFilePath, nil
}

func probeAudioDuration(audioPath string) float64 {
	output, err := exec.Command(
		"ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		audioPath,
	).Output()
	if err != nil {
		return 0
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0
	}
	return duration
}

// truncateString 截断字符串，避免日志过长
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
