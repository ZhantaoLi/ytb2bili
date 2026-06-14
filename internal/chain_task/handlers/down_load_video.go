package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/base"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/services"
	"github.com/ZhantaoLi/ytb2bili/pkg/cos"
	"github.com/ZhantaoLi/ytb2bili/pkg/utils"
	"gorm.io/gorm"
)

type DownloadVideo struct {
	base.BaseTask
	App               *core.AppServer
	DB                *gorm.DB
	SavedVideoService *services.SavedVideoService
}

type ytDlpAuthAttempt struct {
	label string
	args  []string
}

const defaultYtDlpFormat = "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=1080]+bestaudio/best[height<=1080]/best"

func NewDownloadVideo(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, savedVideoService *services.SavedVideoService) *DownloadVideo {
	return &DownloadVideo{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:               app,
		SavedVideoService: savedVideoService,
	}
}

// findYtDlp 查找系统中的 yt-dlp 可执行文件
func (t *DownloadVideo) findYtDlp() (string, error) {
	// 从配置中获取安装目录
	var installDir string
	if t.App.Config != nil && t.App.Config.YtDlpPath != "" {
		installDir = t.App.Config.YtDlpPath
	}

	// 创建 yt-dlp 管理器
	manager := utils.NewYtDlpManager(t.App.Logger, installDir)

	// 检查是否已安装
	if manager.IsInstalled() {
		path := normalizeExecutablePath(manager.GetBinaryPath())
		t.App.Logger.Debugf("找到 yt-dlp: %s", path)
		return path, nil
	}

	return "", fmt.Errorf("未找到 yt-dlp，请确保已正确安装")
}

func normalizeExecutablePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absPath
}

// findLatestCookiesFile 查找最新的 cookies 文件
func (t *DownloadVideo) findLatestCookiesFile() string {
	// 1. 优先查找 data/cookies/ 目录下最新的用户提交的 cookies
	cookiesDir := filepath.Join(t.App.Config.DataPath, "cookies")

	// 确保路径是绝对路径
	if !filepath.IsAbs(cookiesDir) {
		absPath, err := filepath.Abs(cookiesDir)
		if err == nil {
			cookiesDir = absPath
		}
	}

	if entries, err := os.ReadDir(cookiesDir); err == nil {
		var latestFile string
		var latestTime int64

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !strings.HasPrefix(name, "cookies_") || !strings.HasSuffix(name, ".txt") {
				continue
			}

			filePath := filepath.Join(cookiesDir, name)
			if info, err := entry.Info(); err == nil {
				if info.ModTime().Unix() > latestTime {
					latestTime = info.ModTime().Unix()
					latestFile = filePath
				}
			}
		}

		if latestFile != "" {
			t.App.Logger.Infof("🍪 找到用户提交的最新 cookies 文件: %s", latestFile)
			return latestFile
		}
	} else {
		t.App.Logger.Warnf("⚠️ 无法读取 cookies 目录 %s: %v", cookiesDir, err)
	}

	// 2. 兼容旧逻辑：查找配置文件目录下的 cookies.txt
	configDir := filepath.Dir(t.App.Config.Path)
	cookiesPath := filepath.Join(configDir, "cookies.txt")

	// 确保是绝对路径
	if !filepath.IsAbs(cookiesPath) {
		absPath, err := filepath.Abs(cookiesPath)
		if err == nil {
			cookiesPath = absPath
		}
	}

	if _, err := os.Stat(cookiesPath); err == nil {
		t.App.Logger.Infof("🍪 找到配置目录下的 cookies 文件: %s", cookiesPath)
		return cookiesPath
	}

	// 3. 查找当前目录的 cookies.txt
	currentCookies := "cookies.txt"
	if _, err := os.Stat(currentCookies); err == nil {
		absPath, err := filepath.Abs(currentCookies)
		if err == nil {
			t.App.Logger.Infof("🍪 找到当前目录的 cookies 文件: %s", absPath)
			return absPath
		}
	}

	t.App.Logger.Warn("⚠️ 未找到任何可用的 cookies 文件")
	return ""
}

// getVideoURL 根据 VideoID 构建完整的视频 URL
func (t *DownloadVideo) getVideoURL() string {
	videoID := t.StateManager.VideoID

	// 如果已经是完整 URL，直接返回
	if strings.HasPrefix(videoID, "http://") || strings.HasPrefix(videoID, "https://") {
		return videoID
	}

	// YouTube 短 ID 格式
	if len(videoID) == 11 && !strings.Contains(videoID, "/") {
		return fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	}

	// Bilibili BV 号
	if strings.HasPrefix(videoID, "BV") {
		return fmt.Sprintf("https://www.bilibili.com/video/%s", videoID)
	}

	// 默认作为 YouTube ID 处理
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
}

func (t *DownloadVideo) Execute(context map[string]interface{}) bool {
	t.App.Logger.Info("========================================")
	t.App.Logger.Info("DownloadVideo Handler Version: with-cookies-support-v3") // 版本标记
	t.App.Logger.Infof("开始下载视频: %s", t.StateManager.VideoID)
	t.App.Logger.Info("========================================")

	// 1. 查找 yt-dlp 可执行文件
	ytdlpPath, err := t.findYtDlp()
	if err != nil {
		t.App.Logger.Errorf("❌ %v", err)
		context["error"] = err.Error()
		return false
	}

	// 2. 确保下载目录存在
	if err := os.MkdirAll(t.StateManager.CurrentDir, 0755); err != nil {
		t.App.Logger.Errorf("❌ 创建下载目录失败: %v", err)
		context["error"] = err.Error()
		return false
	}

	// 3. 尝试下载（先用代理，失败后不用代理重试）
	videoURL := t.getVideoURL()
	useProxy := t.App.Config != nil && t.App.Config.ProxyConfig != nil &&
		t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != ""

	// 第一次尝试：使用代理（如果配置了）
	if useProxy {
		t.App.Logger.Info("🔄 尝试使用代理下载...")
		if t.executeDownload(ytdlpPath, videoURL, true, context) {
			return true
		}
		t.App.Logger.Warn("⚠️ 代理下载失败，尝试不使用代理重试...")
	}

	// 第二次尝试：不使用代理
	t.App.Logger.Info("🔄 尝试不使用代理下载...")
	return t.executeDownload(ytdlpPath, videoURL, false, context)
}

// executeDownload 执行实际的下载操作
func (t *DownloadVideo) executeDownload(ytdlpPath, videoURL string, useProxy bool, context map[string]interface{}) bool {
	authAttempts := t.buildYtDlpAuthAttempts()
	var lastErr error
	var lastStderr []string

	for i, authAttempt := range authAttempts {
		command := t.buildDownloadCommand(ytdlpPath, videoURL, useProxy, authAttempt)
		t.App.Logger.Infof("🔐 下载认证方式: %s", authAttempt.label)
		t.App.Logger.Infof("执行命令: %s", strings.Join(command, " "))
		t.App.Logger.Infof("下载目录: %s", t.StateManager.CurrentDir)
		t.App.Logger.Infof("视频URL: %s", videoURL)

		stderrLines, err := t.runDownloadCommand(command)
		if err == nil {
			lastErr = nil
			lastStderr = nil
			break
		}

		lastErr = err
		lastStderr = stderrLines
		t.App.Logger.Errorf("❌ 视频下载失败: %v", err)

		if isYouTubeAuthChallenge(stderrLines) && i+1 < len(authAttempts) {
			t.App.Logger.Warn("⚠️ 当前 cookies 无法通过 YouTube 验证，自动切换到下一种认证方式重试")
			continue
		}

		context["error"] = formatDownloadError(err, stderrLines)
		return false
	}

	if lastErr != nil {
		context["error"] = formatDownloadError(lastErr, lastStderr)
		return false
	}

	// 10. 验证下载的文件
	downloadedFile := t.findDownloadedFile()
	if downloadedFile == "" {
		errMsg := "下载完成但未找到视频文件"
		t.App.Logger.Error("❌ " + errMsg)
		context["error"] = errMsg
		return false
	}

	// 11. 保存文件信息到 context
	context["downloaded_file"] = downloadedFile
	t.App.Logger.Infof("✓ 视频下载成功: %s", downloadedFile)

	// 12. 获取视频元数据（标题、描述等）
	t.App.Logger.Info("📋 获取视频元数据...")
	metadata, err := t.getVideoMetadata(ytdlpPath)
	if err != nil {
		t.App.Logger.Warnf("⚠️ 获取视频元数据失败: %v，将使用默认值", err)
	} else {
		context["original_title"] = metadata.Title
		context["original_description"] = metadata.Description
		t.App.Logger.Infof("✓ 原始标题: %s", metadata.Title)
		if metadata.Description != "" {
			t.App.Logger.Infof("✓ 原始描述: %s", t.truncateString(metadata.Description, 100))
		}

		// 保存到数据库
		if t.SavedVideoService != nil {
			savedVideo, err := t.SavedVideoService.GetVideoByVideoID(t.StateManager.VideoID)
			if err == nil {
				savedVideo.Title = metadata.Title
				savedVideo.Description = metadata.Description
				if err := t.SavedVideoService.UpdateVideo(savedVideo); err != nil {
					t.App.Logger.Errorf("❌ 保存原始元数据到数据库失败: %v", err)
				} else {
					t.App.Logger.Info("✅ 原始元数据已保存到数据库")
				}
			}
		}
	}

	t.App.Logger.Info("========================================")

	return true
}

func (t *DownloadVideo) buildDownloadCommand(ytdlpPath, videoURL string, useProxy bool, authAttempt ytDlpAuthAttempt) []string {
	command := []string{
		ytdlpPath,
		"-P", t.StateManager.CurrentDir,
		"-o", "%(id)s.%(ext)s",
		"-f", defaultYtDlpFormat,
		"--merge-output-format", "mp4",
	}

	command = append(command, authAttempt.args...)
	command = appendYtDlpRuntimeArgs(command, exec.LookPath)

	if useProxy && t.App.Config != nil && t.App.Config.ProxyConfig != nil &&
		t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != "" {
		command = append(command, "--proxy", t.App.Config.ProxyConfig.ProxyHost)
		t.App.Logger.Infof("📡 使用代理: %s", t.App.Config.ProxyConfig.ProxyHost)
	} else if !useProxy {
		t.App.Logger.Info("🌐 不使用代理")
	}

	return append(command, videoURL)
}

func (t *DownloadVideo) runDownloadCommand(command []string) ([]string, error) {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = t.StateManager.CurrentDir

	// 捕获标准输出和标准错误
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.App.Logger.Errorf("❌ 创建标准输出管道失败: %v", err)
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.App.Logger.Errorf("❌ 创建标准错误管道失败: %v", err)
		return nil, err
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		t.App.Logger.Errorf("❌ 启动下载命令失败: %v", err)
		return nil, err
	}

	// 实时读取输出，同时保留 stderr 用于返回给前端排障。
	var outputWG sync.WaitGroup
	var stderrLines []string
	outputWG.Add(2)
	go func() {
		defer outputWG.Done()
		t.logOutput(stdout, "INFO")
	}()
	go func() {
		defer outputWG.Done()
		stderrLines = t.logAndCollectOutput(stderr, "ERROR")
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		outputWG.Wait()
		return stderrLines, err
	}
	outputWG.Wait()

	return stderrLines, nil
}

type executableLookPath func(string) (string, error)

func (t *DownloadVideo) buildYtDlpAuthAttempts() []ytDlpAuthAttempt {
	cookiesPath := t.findLatestCookiesFile()
	if cookiesPath == "" {
		t.App.Logger.Info("🍪 未找到 cookies 文件，尝试从 Chrome 读取 cookies")
		return buildYtDlpAuthAttemptsFromCookies("")
	}

	t.App.Logger.Infof("🍪 使用 Cookies 文件: %s", cookiesPath)
	return buildYtDlpAuthAttemptsFromCookies(cookiesPath)
}

func buildYtDlpAuthAttemptsFromCookies(cookiesPath string) []ytDlpAuthAttempt {
	if cookiesPath == "" {
		return []ytDlpAuthAttempt{{
			label: "Chrome 浏览器 cookies",
			args:  []string{"--cookies-from-browser", "chrome"},
		}}
	}

	return []ytDlpAuthAttempt{
		{
			label: "用户提交的 cookies 文件",
			args:  []string{"--cookies", cookiesPath},
		},
		{
			label: "Chrome 浏览器 cookies",
			args:  []string{"--cookies-from-browser", "chrome"},
		},
	}
}

func appendYtDlpRuntimeArgs(args []string, lookPath executableLookPath) []string {
	nodePath, err := lookPath("node")
	if err != nil || nodePath == "" {
		return args
	}
	return append(args, "--js-runtimes", "node:"+nodePath)
}

// logOutput 实时输出日志
func (t *DownloadVideo) logOutput(reader io.Reader, level string) {
	_ = t.logAndCollectOutput(reader, level)
}

func (t *DownloadVideo) logAndCollectOutput(reader io.Reader, level string) []string {
	scanner := bufio.NewScanner(reader)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		lines = append(lines, line)

		// 解析进度信息
		if strings.Contains(line, "[download]") {
			if strings.Contains(line, "Destination:") {
				t.App.Logger.Infof("📥 %s", line)
			} else if strings.Contains(line, "%") {
				// 进度信息，使用 Debug 级别避免日志过多
				t.App.Logger.Debugf("⏳ %s", line)
			} else {
				t.App.Logger.Infof("📥 %s", line)
			}
		} else if strings.Contains(line, "[ffmpeg]") {
			t.App.Logger.Infof("🔄 %s", line)
		} else {
			if level == "ERROR" {
				t.App.Logger.Warnf("⚠️  %s", line)
			} else {
				t.App.Logger.Debugf("%s", line)
			}
		}
	}
	return lines
}

func formatDownloadError(err error, stderrLines []string) string {
	detail := strings.TrimSpace(strings.Join(stderrLines, "\n"))
	if detail == "" {
		return fmt.Sprintf("下载失败: %v", err)
	}

	for _, line := range stderrLines {
		if strings.Contains(line, "Sign in to confirm") || strings.Contains(line, "not a bot") {
			return fmt.Sprintf("下载失败: YouTube 要求登录验证，需要 YouTube cookies。yt-dlp 输出: %s", strings.TrimSpace(line))
		}
	}

	for _, line := range stderrLines {
		if strings.Contains(line, "ERROR:") {
			return fmt.Sprintf("下载失败: %s", strings.TrimSpace(line))
		}
	}

	if len(stderrLines) > 3 {
		stderrLines = stderrLines[len(stderrLines)-3:]
	}
	return fmt.Sprintf("下载失败: %s", strings.TrimSpace(strings.Join(stderrLines, " | ")))
}

func isYouTubeAuthChallenge(stderrLines []string) bool {
	for _, line := range stderrLines {
		if strings.Contains(line, "Sign in to confirm") || strings.Contains(line, "not a bot") {
			return true
		}
	}
	return false
}

// findDownloadedFile 查找下载的视频文件
func (t *DownloadVideo) findDownloadedFile() string {
	// 查找目录下的 mp4 文件
	files, err := filepath.Glob(filepath.Join(t.StateManager.CurrentDir, "*.mp4"))
	if err != nil || len(files) == 0 {
		// 尝试查找其他视频格式
		for _, ext := range []string{"*.webm", "*.mkv", "*.flv"} {
			files, err = filepath.Glob(filepath.Join(t.StateManager.CurrentDir, ext))
			if err == nil && len(files) > 0 {
				break
			}
		}
	}

	if len(files) > 0 {
		// 返回最新的文件
		latestFile := files[0]
		latestTime := int64(0)

		for _, file := range files {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}
			if info.ModTime().Unix() > latestTime {
				latestTime = info.ModTime().Unix()
				latestFile = file
			}
		}

		return latestFile
	}

	return ""
}

// VideoMetadataInfo 视频元数据信息
type VideoMetadataInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Uploader    string `json:"uploader"`
	Duration    int    `json:"duration"`
}

// getVideoMetadata 使用 yt-dlp 获取视频元数据（带代理回退）
func (t *DownloadVideo) getVideoMetadata(ytdlpPath string) (*VideoMetadataInfo, error) {
	videoURL := t.getVideoURL()
	authAttempts := t.buildYtDlpAuthAttempts()
	useProxy := t.App.Config != nil && t.App.Config.ProxyConfig != nil &&
		t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != ""

	for i, authAttempt := range authAttempts {
		output, err := t.runMetadataCommand(ytdlpPath, videoURL, useProxy, authAttempt)
		if err == nil {
			var metadata VideoMetadataInfo
			if err := json.Unmarshal(output, &metadata); err != nil {
				return nil, fmt.Errorf("解析元数据失败: %v", err)
			}
			return &metadata, nil
		}

		if isYouTubeAuthChallenge([]string{err.Error()}) && i+1 < len(authAttempts) {
			t.App.Logger.Warn("⚠️ 当前 cookies 获取元数据失败，自动切换到下一种认证方式重试")
			continue
		}

		if err != nil && useProxy {
			t.App.Logger.Warnf("⚠️ 使用代理获取元数据失败，尝试不使用代理...")
			output, err = t.runMetadataCommand(ytdlpPath, videoURL, false, authAttempt)
			if err == nil {
				t.App.Logger.Info("✓ 不使用代理成功获取元数据")
				var metadata VideoMetadataInfo
				if err := json.Unmarshal(output, &metadata); err != nil {
					return nil, fmt.Errorf("解析元数据失败: %v", err)
				}
				return &metadata, nil
			}
		}
	}

	return nil, fmt.Errorf("获取元数据失败")
}

func (t *DownloadVideo) runMetadataCommand(ytdlpPath, videoURL string, useProxy bool, authAttempt ytDlpAuthAttempt) ([]byte, error) {
	args := []string{"--dump-json", "--no-download"}
	args = append(args, authAttempt.args...)
	args = appendYtDlpRuntimeArgs(args, exec.LookPath)

	if useProxy && t.App.Config != nil && t.App.Config.ProxyConfig != nil &&
		t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != "" {
		args = append(args, "--proxy", t.App.Config.ProxyConfig.ProxyHost)
		t.App.Logger.Debugf("📡 使用代理获取元数据: %s", t.App.Config.ProxyConfig.ProxyHost)
	}

	args = append(args, videoURL)
	t.App.Logger.Debugf("🔐 元数据认证方式: %s", authAttempt.label)
	cmd := exec.Command(ytdlpPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("获取元数据失败: %v: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

// truncateString 截断字符串用于日志显示
func (t *DownloadVideo) truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
