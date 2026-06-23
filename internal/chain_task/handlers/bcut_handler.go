package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/base"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/pkg/cos"
	"gorm.io/gorm"
)

const (
	APIBaseURL      = "https://member.bilibili.com/x/bcut/rubick-interface"
	APIReqUpload    = APIBaseURL + "/resource/create"
	APICommitUpload = APIBaseURL + "/resource/create/complete"
	APICreateTask   = APIBaseURL + "/task"
	APIQueryResult  = APIBaseURL + "/task/result"

	bcutSegmentSeconds = 600
	bcutModelID        = "8"
)

// BcutHandler B站必剪语音转录处理器
type BcutHandler struct {
	base.BaseTask
	App      *core.AppServer
	DB       *gorm.DB
	Language string // 语言代码，如 "zh", "en"

	// 上传相关状态
	uploadID    string
	uploadURLs  []string
	perSize     int
	clips       int
	inBossKey   string
	resourceID  string
	downloadURL string
	etags       []string
	taskID      string
}

// NewBcutHandler 创建B站必剪转录处理器
func NewBcutHandler(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, language string) *BcutHandler {
	if language == "" {
		language = "zh" // 默认中文
	}

	return &BcutHandler{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:      app,
		Language: language,
		etags:    []string{},
	}
}

// Execute 执行B站必剪转录任务
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (h *BcutHandler) Execute(context map[string]interface{}) bool {
	fmt.Println("开始使用 B站必剪 转录音频")

	if ok, err := h.downloadYouTubeSubtitle(context); ok {
		return true
	} else if err != nil {
		fmt.Printf("⚠️ YouTube 字幕下载不可用，回退到 B站必剪 转录: %v\n", err)
	}

	if h.useExistingSubtitle(context, "existing") {
		return true
	}

	// 检查音频文件是否存在
	audioPath := h.StateManager.OriginalWAV
	if audioPath == "" || !fileExists(audioPath) {
		// 如果没有WAV文件，尝试使用MP3音频格式
		audioPath = h.StateManager.OriginalMP3
	}

	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		fmt.Printf("错误: 音频文件不存在: %s\n", audioPath)
		context["error"] = fmt.Sprintf("音频文件不存在: %s", audioPath)
		return false
	}

	fmt.Printf("📝 使用 B站必剪 转录: %s\n", audioPath)
	fmt.Printf("   语言: %s\n", h.Language)

	result, err := h.transcribeAudio(audioPath)
	if err != nil {
		fmt.Printf("❌ B站必剪转录失败: %v\n", err)
		context["error"] = fmt.Sprintf("B站必剪转录失败: %v", err)
		return false
	}

	if err := h.saveSubtitle(result); err != nil {
		fmt.Printf("❌ 保存字幕失败: %v\n", err)
		context["error"] = fmt.Sprintf("保存字幕失败: %v", err)
		return false
	}

	fmt.Printf("✅ B站必剪转录完成，字幕文件保存至: %s\n", h.StateManager.OriginalSRT)
	context["subtitle_path"] = h.StateManager.OriginalSRT
	return true
}

func (h *BcutHandler) useExistingSubtitle(context map[string]interface{}, source string) bool {
	if h == nil || h.StateManager == nil || !fileExists(h.StateManager.OriginalSRT) {
		return false
	}
	fmt.Printf("✅ 复用已有字幕文件: %s\n", h.StateManager.OriginalSRT)
	context["subtitle_path"] = h.StateManager.OriginalSRT
	context["subtitle_source"] = source
	return true
}

type youtubeSubtitleCommandOptions struct {
	YtDlpPath   string
	VideoURL    string
	OutputBase  string
	Languages   []string
	Format      string
	ProxyURL    string
	AuthAttempt ytDlpAuthAttempt
}

func buildYouTubeSubtitleCommand(options youtubeSubtitleCommandOptions) []string {
	format := strings.TrimSpace(options.Format)
	if format == "" {
		format = "srt"
	}
	languages := options.Languages
	if len(languages) == 0 {
		languages = preferredYouTubeSubtitleLanguages()
	}

	command := []string{
		options.YtDlpPath,
		"--ignore-no-formats-error",
		"--skip-download",
		"--write-subs",
		"--write-auto-subs",
		"--sub-langs", strings.Join(languages, ","),
		"--sub-format", format,
		"--convert-subs", format,
		"-o", options.OutputBase + ".%(ext)s",
	}
	command = append(command, options.AuthAttempt.args...)
	command = appendYtDlpRuntimeArgs(command, exec.LookPath)
	if strings.TrimSpace(options.ProxyURL) != "" {
		command = append(command, "--proxy", options.ProxyURL)
	}
	return append(command, options.VideoURL)
}

func preferredYouTubeSubtitleLanguages() []string {
	return []string{"en-orig", "en", "en-US", "en-GB"}
}

func (h *BcutHandler) downloadYouTubeSubtitle(context map[string]interface{}) (bool, error) {
	if h == nil || h.App == nil || h.App.Config == nil || h.StateManager == nil {
		return false, nil
	}
	if strings.TrimSpace(h.StateManager.CurrentDir) == "" || strings.TrimSpace(h.StateManager.OriginalSRT) == "" {
		return false, nil
	}

	downloadTask := NewDownloadVideo("下载YouTube字幕", h.App, h.StateManager, h.Client, nil)
	ytdlpPath, err := downloadTask.findYtDlp()
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(h.StateManager.CurrentDir, 0755); err != nil {
		return false, err
	}

	videoURL := downloadTask.getVideoURL()
	languages := preferredYouTubeSubtitleLanguages()
	outputBase := filepath.Join(h.StateManager.CurrentDir, "youtube_captions")
	proxyURL := ""
	if h.App.Config.ProxyConfig != nil {
		proxyURL = h.App.Config.ProxyConfig.ProxyHost
	}
	useProxy := h.App.Config.ProxyConfig != nil &&
		h.App.Config.ProxyConfig.UseProxy &&
		strings.TrimSpace(proxyURL) != ""

	var lastErr error
	for _, proxyURL := range subtitleProxyAttempts(useProxy, proxyURL) {
		for _, authAttempt := range downloadTask.buildYtDlpAuthAttempts() {
			command := buildYouTubeSubtitleCommand(youtubeSubtitleCommandOptions{
				YtDlpPath:   ytdlpPath,
				VideoURL:    videoURL,
				OutputBase:  outputBase,
				Languages:   languages,
				Format:      "srt",
				ProxyURL:    proxyURL,
				AuthAttempt: authAttempt,
			})
			h.App.Logger.Infof("🔐 YouTube 字幕认证方式: %s", authAttempt.label)
			if proxyURL != "" {
				h.App.Logger.Infof("📡 YouTube 字幕使用代理: %s", proxyURL)
			}

			stderrLines, err := runCommandWithTimeout(
				command,
				h.StateManager.CurrentDir,
				10*time.Minute,
				nil,
				func(reader io.Reader) {
					downloadTask.logOutput(reader, "INFO")
				},
				func(reader io.Reader) []string {
					return downloadTask.logAndCollectOutput(reader, "ERROR")
				},
			)
			if err != nil {
				lastErr = fmt.Errorf("%w: %s", err, strings.TrimSpace(strings.Join(stderrLines, "\n")))
				if isYouTubeAuthChallenge(stderrLines) {
					continue
				}
				continue
			}

			subtitlePath := findPreferredYouTubeSubtitleFile(outputBase, languages, "srt")
			if subtitlePath == "" {
				lastErr = fmt.Errorf("yt-dlp 未生成 YouTube 字幕文件")
				continue
			}
			if err := copyLocalFile(subtitlePath, h.StateManager.OriginalSRT); err != nil {
				return false, err
			}
			h.App.Logger.Infof("✅ 已优先使用 YouTube 字幕: %s", subtitlePath)
			context["subtitle_path"] = h.StateManager.OriginalSRT
			context["subtitle_source"] = "youtube"
			context["youtube_subtitle_path"] = subtitlePath
			return true, nil
		}
	}

	return false, lastErr
}

func subtitleProxyAttempts(useProxy bool, proxyURL string) []string {
	if useProxy && strings.TrimSpace(proxyURL) != "" {
		return []string{proxyURL, ""}
	}
	return []string{""}
}

func findPreferredYouTubeSubtitleFile(outputBase string, languages []string, format string) string {
	for _, language := range languages {
		candidate := fmt.Sprintf("%s.%s.%s", outputBase, language, format)
		if fileExists(candidate) {
			return candidate
		}
	}

	matches, err := filepath.Glob(fmt.Sprintf("%s*.%s", outputBase, format))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	for _, match := range matches {
		if fileExists(match) {
			return match
		}
	}
	return ""
}

func copyLocalFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func (h *BcutHandler) transcribeAudio(audioPath string) (map[string]interface{}, error) {
	duration := probeAudioDuration(audioPath)
	if duration <= bcutSegmentSeconds {
		return h.transcribeAudioFile(audioPath)
	}

	segmentPaths, err := h.splitAudioForBcut(audioPath, bcutSegmentSeconds)
	if err != nil {
		return nil, err
	}

	combined := map[string]interface{}{}
	var combinedUtterances []interface{}
	var language string
	for i, segmentPath := range segmentPaths {
		offsetMS := int64(i * bcutSegmentSeconds * 1000)
		fmt.Printf("🎙️ B站必剪转录片段 %d/%d: %s\n", i+1, len(segmentPaths), filepath.Base(segmentPath))
		result, err := h.transcribeAudioFile(segmentPath)
		if err != nil {
			return nil, fmt.Errorf("片段 %d 转录失败: %w", i+1, err)
		}
		resultJSON, ok := result["result"].(string)
		if !ok {
			return nil, fmt.Errorf("片段 %d 缺少 result 字段", i+1)
		}
		var resultData map[string]interface{}
		if err := json.Unmarshal([]byte(resultJSON), &resultData); err != nil {
			return nil, fmt.Errorf("解析片段 %d 结果失败: %w", i+1, err)
		}
		if lang, ok := resultData["language"].(string); ok && language == "" {
			language = lang
		}
		utterances, ok := resultData["utterances"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("片段 %d 缺少 utterances 数据", i+1)
		}
		for _, item := range utterances {
			utterance, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if start, ok := utterance["start_time"].(float64); ok {
				utterance["start_time"] = start + float64(offsetMS)
			}
			if end, ok := utterance["end_time"].(float64); ok {
				utterance["end_time"] = end + float64(offsetMS)
			}
			combinedUtterances = append(combinedUtterances, utterance)
		}
	}

	resultData := map[string]interface{}{"utterances": combinedUtterances}
	if language != "" {
		resultData["language"] = language
	}
	resultBytes, err := json.Marshal(resultData)
	if err != nil {
		return nil, err
	}
	combined["result"] = string(resultBytes)
	return combined, nil
}

func (h *BcutHandler) transcribeAudioFile(audioPath string) (map[string]interface{}, error) {
	h.resetUploadState()

	fileData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("读取音频文件失败: %w", err)
	}

	if err := h.requestUpload(len(fileData)); err != nil {
		return nil, fmt.Errorf("申请上传失败: %w", err)
	}
	if err := h.uploadParts(fileData); err != nil {
		return nil, fmt.Errorf("上传音频失败: %w", err)
	}
	if err := h.commitUpload(); err != nil {
		return nil, fmt.Errorf("提交上传失败: %w", err)
	}
	if err := h.createTask(); err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}
	result, err := h.queryResultWithRetry(60, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("查询结果失败: %w", err)
	}
	return result, nil
}

func (h *BcutHandler) resetUploadState() {
	h.uploadID = ""
	h.uploadURLs = nil
	h.perSize = 0
	h.clips = 0
	h.inBossKey = ""
	h.resourceID = ""
	h.downloadURL = ""
	h.etags = []string{}
	h.taskID = ""
}

func (h *BcutHandler) splitAudioForBcut(audioPath string, segmentSeconds int) ([]string, error) {
	segmentsDir := filepath.Join(h.StateManager.CurrentDir, "bcut_segments", fmt.Sprintf("%d", os.Getpid()))
	if err := os.MkdirAll(segmentsDir, 0755); err != nil {
		return nil, err
	}

	outputPattern := filepath.Join(segmentsDir, "segment_%04d.mp3")
	command := []string{
		"ffmpeg",
		"-y",
		"-i", audioPath,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-b:a", "64k",
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%d", segmentSeconds),
		"-reset_timestamps", "1",
		outputPattern,
	}
	output, err := runASRCommandWithTimeout(command, defaultASRSplitCommandTimeout, nil)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg 切分必剪音频失败: %v, output=%s", err, strings.TrimSpace(string(output)))
	}

	segments, err := filepath.Glob(filepath.Join(segmentsDir, "segment_*.mp3"))
	if err != nil {
		return nil, err
	}
	if len(segments) == 0 {
		return nil, fmt.Errorf("ffmpeg 未生成必剪音频片段")
	}
	sort.Strings(segments)
	return segments, nil
}

// requestUpload 申请上传
func (h *BcutHandler) requestUpload(fileSize int) error {
	payload := map[string]interface{}{
		"type":             2,
		"name":             "audio.mp3",
		"size":             fileSize,
		"ResourceFileType": "mp3",
		"model_id":         bcutModelID,
	}

	respData, err := h.makeRequest("POST", APIReqUpload, payload)
	if err != nil {
		return err
	}

	data := respData["data"].(map[string]interface{})
	h.uploadID = data["upload_id"].(string)
	h.resourceID = data["resource_id"].(string)
	h.inBossKey = data["in_boss_key"].(string)
	h.perSize = int(data["per_size"].(float64))

	uploadURLs := data["upload_urls"].([]interface{})
	h.uploadURLs = make([]string, len(uploadURLs))
	for i, url := range uploadURLs {
		h.uploadURLs[i] = url.(string)
	}

	h.clips = len(h.uploadURLs)

	fmt.Printf("📤 申请上传成功 - ID: %s, 分片数: %d, 分片大小: %dKB\n",
		h.uploadID, h.clips, h.perSize/1024)

	return nil
}

// uploadParts 上传音频分片
func (h *BcutHandler) uploadParts(fileData []byte) error {
	for i := 0; i < h.clips; i++ {
		start := i * h.perSize
		end := start + h.perSize
		if end > len(fileData) {
			end = len(fileData)
		}

		fmt.Printf("📤 上传分片 %d/%d: %d-%d bytes\n", i+1, h.clips, start, end)

		req, err := http.NewRequest("PUT", h.uploadURLs[i], bytes.NewReader(fileData[start:end]))
		if err != nil {
			return fmt.Errorf("创建上传请求失败: %v", err)
		}

		req.Header.Set("Content-Type", "application/octet-stream")

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("上传分片 %d 失败: %v", i, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("上传分片 %d 失败，状态码: %d", i, resp.StatusCode)
		}

		etag := strings.Trim(resp.Header.Get("ETag"), "\"")
		h.etags = append(h.etags, etag)

		fmt.Printf("✅ 分片 %d 上传成功，ETag: %s\n", i+1, etag)
	}

	return nil
}

// commitUpload 提交上传
func (h *BcutHandler) commitUpload() error {
	payload := map[string]interface{}{
		"InBossKey":  h.inBossKey,
		"ResourceId": h.resourceID,
		"Etags":      strings.Join(h.etags, ","),
		"UploadId":   h.uploadID,
		"model_id":   bcutModelID,
	}

	respData, err := h.makeRequest("POST", APICommitUpload, payload)
	if err != nil {
		return err
	}
	data := respData["data"].(map[string]interface{})
	h.downloadURL = data["download_url"].(string)

	fmt.Println("✅ 上传提交成功")
	return nil
}

// createTask 创建转录任务
func (h *BcutHandler) createTask() error {
	payload := map[string]interface{}{
		"resource": h.downloadURL,
		"model_id": bcutModelID,
	}

	respData, err := h.makeRequest("POST", APICreateTask, payload)
	if err != nil {
		return err
	}

	data := respData["data"].(map[string]interface{})
	h.taskID = data["task_id"].(string)

	fmt.Printf("✅ 任务创建成功 - TaskID: %s\n", h.taskID)
	return nil
}

// queryResult 查询转录结果
func (h *BcutHandler) queryResult() (map[string]interface{}, error) {
	url := bcutQueryResultURL(h.taskID)

	respData, err := h.makeRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	data := respData["data"].(map[string]interface{})
	return data, nil
}

func bcutQueryResultURL(taskID string) string {
	return fmt.Sprintf("%s?model_id=%s&task_id=%s", APIQueryResult, bcutModelID, taskID)
}

// queryResultWithRetry 轮询查询结果
func (h *BcutHandler) queryResultWithRetry(maxRetries int, interval time.Duration) (map[string]interface{}, error) {
	fmt.Printf("🔄 开始查询转录结果，最多重试 %d 次...\n", maxRetries)

	for i := 0; i < maxRetries; i++ {
		result, err := h.queryResult()
		if err != nil {
			return nil, err
		}

		state := int(result["state"].(float64))

		switch state {
		case 4: // 成功
			fmt.Println("✅ 转录成功")
			return result, nil
		case 5: // 失败
			errorCode := "Unknown"
			if ec, ok := result["error_code"]; ok {
				errorCode = fmt.Sprintf("%v", ec)
			}
			return nil, fmt.Errorf("转录任务失败，错误代码: %s", errorCode)
		default:
			fmt.Printf("⏳ 转录处理中... state=%d (%d/%d)\n", state, i+1, maxRetries)
			time.Sleep(interval)
		}
	}

	return nil, fmt.Errorf("查询超时，已重试 %d 次", maxRetries)
}

// saveSubtitle 保存字幕文件
func (h *BcutHandler) saveSubtitle(result map[string]interface{}) error {
	resultJSON := result["result"].(string)

	var resultData map[string]interface{}
	if err := json.Unmarshal([]byte(resultJSON), &resultData); err != nil {
		return fmt.Errorf("解析结果JSON失败: %v", err)
	}

	utterances, ok := resultData["utterances"].([]interface{})
	if !ok {
		return fmt.Errorf("未找到utterances数据")
	}

	// 创建SRT文件
	outFile, err := os.Create(h.StateManager.OriginalSRT)
	if err != nil {
		return fmt.Errorf("创建字幕文件失败: %v", err)
	}
	defer outFile.Close()

	// 写入SRT格式
	for i, u := range utterances {
		utterance := u.(map[string]interface{})
		text := strings.TrimSpace(utterance["transcript"].(string))

		// B站ASR返回的时间戳是毫秒，需要转换为秒
		startTime := int64(utterance["start_time"].(float64))
		endTime := int64(utterance["end_time"].(float64))

		// SRT序号（从1开始）
		fmt.Fprintf(outFile, "%d\n", i+1)

		// SRT时间格式: HH:MM:SS,mmm --> HH:MM:SS,mmm
		fmt.Fprintf(outFile, "%s --> %s\n",
			formatSRTTimeFromMS(startTime),
			formatSRTTimeFromMS(endTime))

		// 字幕文本
		fmt.Fprintf(outFile, "%s\n\n", text)
	}

	// 提取语言信息
	if lang, ok := resultData["language"].(string); ok {
		fmt.Printf("📝 检测到语言: %s\n", lang)
	}

	return nil
}

// makeRequest 发起HTTP请求
func (h *BcutHandler) makeRequest(method, url string, payload interface{}) (map[string]interface{}, error) {
	var body io.Reader

	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("序列化请求数据失败: %v", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Bilibili/1.0.0 (https://www.bilibili.com)")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应JSON失败: %v", err)
	}

	code, ok := result["code"].(float64)
	if !ok || int(code) != 0 {
		message := "未知错误"
		if msg, ok := result["message"].(string); ok {
			message = msg
		}
		return nil, fmt.Errorf("API错误 (code: %.0f): %s, response=%s", code, message, string(respBody))
	}

	return result, nil
}

// formatSRTTimeFromMS 格式化毫秒时间为SRT格式 (HH:MM:SS,mmm)
func formatSRTTimeFromMS(ms int64) string {
	totalSeconds := ms / 1000
	milliseconds := ms % 1000

	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, milliseconds)
}
