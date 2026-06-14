package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/base"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/services"
	"github.com/ZhantaoLi/ytb2bili/pkg/cos"
	"gorm.io/gorm"
)

type GenerateMetadata struct {
	base.BaseTask
	App               *core.AppServer
	DeepSeekClient    *DeepSeekClient
	GeminiClient      *GeminiClient
	SavedVideoService *services.SavedVideoService
}

func NewGenerateMetadata(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, apiKey string, db *gorm.DB, savedVideoService *services.SavedVideoService) *GenerateMetadata {
	return &GenerateMetadata{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:               app,
		DeepSeekClient:    nil, // 不再固化客户端，运行时动态创建
		SavedVideoService: savedVideoService,
	}
}

// getCurrentDeepSeekClient 获取当前的DeepSeek客户端（使用最新配置）
func (g *GenerateMetadata) getCurrentDeepSeekClient() (*DeepSeekClient, error) {
	if g.App.Config.DeepSeekTransConfig == nil || !g.App.Config.DeepSeekTransConfig.Enabled {
		return nil, fmt.Errorf("DeepSeek 翻译服务未启用")
	}

	apiKey := g.App.Config.DeepSeekTransConfig.ApiKey
	if apiKey == "" {
		return nil, fmt.Errorf("DeepSeek API Key 未配置")
	}

	return NewDeepSeekClient(apiKey), nil
}

type VideoMetadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

func (g *GenerateMetadata) Execute(context map[string]interface{}) bool {
	g.App.Logger.Info("========================================")
	g.App.Logger.Infof("开始生成视频标题和描述: VideoID=%s", g.StateManager.VideoID)
	g.App.Logger.Info("========================================")

	// 0. 检查是否使用 Gemini
	useGemini := false
	if g.App.Config.GeminiConfig != nil && g.App.Config.GeminiConfig.Enabled && g.App.Config.GeminiConfig.UseForMetadata {
		useGemini = true
		g.App.Logger.Info("🤖 使用 Gemini 多模态服务生成元数据")

		// 如果配置了视频分析，尝试使用视频文件
		if g.App.Config.GeminiConfig.AnalyzeVideo {
			if success := g.executeWithGeminiVideo(context); success {
				return true
			}
			g.App.Logger.Warn("⚠️ Gemini 视频分析失败，回退到文本模式")
		}

		// 使用 Gemini 处理字幕文本
		if success := g.executeWithGeminiText(context); success {
			return true
		}
		g.App.Logger.Warn("⚠️ Gemini 文本分析失败，回退到免费默认元数据")
		useGemini = false
	}

	// 只有显式启用 DeepSeek 时才调用云端元数据生成；默认免费流程直接兜底。
	if !useGemini && g.App.Config.OpenAICompatibleConfig != nil && g.App.Config.OpenAICompatibleConfig.Enabled {
		return g.executeWithOpenAICompatible(context)
	}

	if !useGemini && g.App.Config.DeepSeekTransConfig != nil && g.App.Config.DeepSeekTransConfig.Enabled {
		return g.executeWithDeepSeek(context)
	}

	return g.executeWithFreeFallback(context)
}

func (g *GenerateMetadata) executeWithFreeFallback(context map[string]interface{}) bool {
	context["metadata_skipped"] = "cloud_metadata_disabled"
	g.App.Logger.Info("外部元数据 API 未自行配置，免费模式使用默认标题和描述")

	metadata := &VideoMetadata{
		Title:       g.StateManager.VideoID,
		Description: "自动上传的视频",
		Tags:        []string{"视频"},
	}
	if g.SavedVideoService != nil {
		if savedVideo, err := g.SavedVideoService.GetVideoByVideoID(g.StateManager.VideoID); err == nil && savedVideo != nil {
			if title := strings.TrimSpace(savedVideo.Title); title != "" {
				metadata.Title = title
			}
			if desc := strings.TrimSpace(savedVideo.Description); desc != "" {
				metadata.Description = desc
			}
		}
	}

	return g.saveMetadataResults(metadata, context)
}

// executeWithDeepSeek 使用 DeepSeek 生成元数据
func (g *GenerateMetadata) executeWithDeepSeek(context map[string]interface{}) bool {
	// 0. 动态获取最新的DeepSeek客户端
	client, err := g.getCurrentDeepSeekClient()
	if err != nil {
		g.App.Logger.Warnf("外部元数据 API 配置不可用，使用免费默认元数据: %v", err)
		return g.executeWithFreeFallback(context)
	}

	g.App.Logger.Infof("🔑 使用 DeepSeek 配置生成元数据")
	// 更新当前使用的客户端
	g.DeepSeekClient = client

	// 1. 检查中文字幕文件是否存在
	zhSRTPath := filepath.Join(g.StateManager.CurrentDir, "zh.srt")
	if _, err := os.Stat(zhSRTPath); os.IsNotExist(err) {
		g.App.Logger.Warn("⚠️  中文字幕文件不存在，使用默认标题和描述")
		// 使用默认值
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = fmt.Sprintf("包含字幕的视频")
		return true // 没有字幕文件不算失败
	}

	// 2. 读取中文字幕内容
	srtContent, err := os.ReadFile(zhSRTPath)
	if err != nil {
		g.App.Logger.Errorf("❌ 读取中文字幕文件失败: %v", err)
		context["error"] = "读取翻译字幕失败，请确保字幕翻译步骤已完成"
		return false
	}

	// 3. 解析字幕提取文本
	subtitleText := g.extractTextFromSRT(string(srtContent))
	if subtitleText == "" {
		g.App.Logger.Warn("⚠️  字幕内容为空，使用默认标题和描述")
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = fmt.Sprintf("包含字幕的视频")
		return true
	}

	g.App.Logger.Infof("📝 提取到字幕文本，总长度: %d 字符", len(subtitleText))

	// 4. 截取前1000字符用于生成标题和描述（避免token过多）
	maxLength := 1000
	if len(subtitleText) > maxLength {
		subtitleText = subtitleText[:maxLength] + "..."
	}

	// 5. 调用 DeepSeek API 生成标题和描述
	g.App.Logger.Info("🤖 调用 DeepSeek API 生成标题和描述...")
	metadata, err := g.generateMetadataFromDeepSeek(subtitleText)
	if err != nil {
		g.App.Logger.Errorf("❌ 生成标题和描述失败: %v", err)
		g.App.Logger.Warn("⚠️  将使用默认标题和描述，不影响视频上传")
		// 使用默认值
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = fmt.Sprintf("包含字幕的视频")
		return true // API调用失败不算整个任务失败
	}

	// 6. 验证标题长度（Bilibili限制80字符）
	if len([]rune(metadata.Title)) > 80 {
		runes := []rune(metadata.Title)
		metadata.Title = string(runes[:77]) + "..."
		g.App.Logger.Warnf("⚠️  标题过长，已截断为80字符")
	}

	// 7. 保存到 context
	context["video_title"] = metadata.Title
	context["video_description"] = metadata.Description
	context["video_tags"] = metadata.Tags

	// 8. 保存到 meta.json 文件
	g.App.Logger.Info("💾 保存元数据到 meta.json 文件...")
	if err := g.saveMetadataToFile(metadata); err != nil {
		g.App.Logger.Errorf("❌ 保存 meta.json 文件失败: %v", err)
		// 不影响任务继续执行
	} else {
		g.App.Logger.Info("✅ meta.json 文件已保存")
	}

	// 9. 保存到数据库
	g.App.Logger.Info("💾 保存生成的元数据到数据库...")
	savedVideo, err := g.SavedVideoService.GetVideoByVideoID(g.StateManager.VideoID)
	if err != nil {
		g.App.Logger.Errorf("❌ 获取视频记录失败: %v", err)
		// 不影响任务继续执行
	} else {
		// 更新生成的元数据
		savedVideo.GeneratedTitle = metadata.Title
		savedVideo.GeneratedDesc = metadata.Description
		savedVideo.GeneratedTags = strings.Join(metadata.Tags, ",")

		if err := g.SavedVideoService.UpdateVideo(savedVideo); err != nil {
			g.App.Logger.Errorf("❌ 保存元数据到数据库失败: %v", err)
		} else {
			g.App.Logger.Info("✅ 元数据已保存到数据库")
		}
	}

	// 10. 输出生成结果
	g.App.Logger.Info("========================================")
	g.App.Logger.Info("✅ 视频元数据生成成功！")
	g.App.Logger.Infof("📌 标题: %s", metadata.Title)
	g.App.Logger.Infof("📝 描述: %s", g.truncateString(metadata.Description, 100))
	g.App.Logger.Infof("🏷️  标签: %v", metadata.Tags)
	g.App.Logger.Info("========================================")

	return true
}

// extractTextFromSRT 从SRT内容中提取纯文本
func (g *GenerateMetadata) extractTextFromSRT(srtContent string) string {
	lines := strings.Split(srtContent, "\n")
	var textLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行、序号行、时间码行
		if line == "" || isNumber(line) || strings.Contains(line, "-->") {
			continue
		}
		textLines = append(textLines, line)
	}

	return strings.Join(textLines, " ")
}

// isNumber 检查字符串是否为数字
func isNumber(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// generateMetadataFromDeepSeek 调用 DeepSeek API 生成标题和描述
func (g *GenerateMetadata) generateMetadataFromDeepSeek(subtitleText string) (*VideoMetadata, error) {
	prompt := fmt.Sprintf(`请根据以下视频字幕内容，生成一个吸引人的视频标题、详细描述和3-5个相关标签。

字幕内容：
%s

要求：
1. 标题要简洁有力，严格控制在30个字以内（B站限制80字，但建议30字以内更易读），能够准确概括视频主题，吸引观众点击
2. 描述要详细但不要过长，严格控制在600-800字以内，包含视频的主要内容和亮点（注意：B站简介限制2000字，需要预留约200字给原视频链接和分隔线）
3. 标签要准确反映视频内容，3-5个即可
4. 必须使用中文
5. 输出格式必须是JSON，格式如下：
{
  "title": "视频标题",
  "description": "视频描述",
  "tags": ["标签1", "标签2", "标签3"]
}

请直接返回JSON格式的结果，不要包含任何其他说明文字。`, subtitleText)

	// 使用 DeepSeekClient 调用 API
	content, usage, err := g.DeepSeekClient.ChatCompletionWithUsage("你是一个专业的视频内容分析助手，擅长根据视频字幕生成吸引人的标题和描述。", prompt)
	if err != nil {
		return nil, fmt.Errorf("调用 DeepSeek API 失败: %v", err)
	}

	g.App.Logger.Debugf("DeepSeek 原始返回: %s", content)

	metadata, err := parseMetadataResponse(content)
	if err != nil {
		return nil, err
	}

	// Token使用情况
	if usage != nil {
		g.App.Logger.Infof("💰 Token使用: 输入=%d, 输出=%d, 总计=%d",
			usage.PromptTokens,
			usage.CompletionTokens,
			usage.TotalTokens)
	}

	return metadata, nil
}

func (g *GenerateMetadata) executeWithOpenAICompatible(taskContext map[string]interface{}) bool {
	cfg := g.App.Config.OpenAICompatibleConfig
	if cfg == nil || !cfg.Enabled {
		return g.executeWithFreeFallback(taskContext)
	}

	sourceText, err := g.metadataSourceText()
	if err != nil {
		g.App.Logger.Warnf("⚠️ 元数据输入不足，使用默认标题和描述: %v", err)
		return g.executeWithFreeFallback(taskContext)
	}

	maxLength := 2000
	if len([]rune(sourceText)) > maxLength {
		runes := []rune(sourceText)
		sourceText = string(runes[:maxLength]) + "..."
	}

	client := NewOpenAICompatibleClient(&OpenAIClientConfig{
		APIKey:      cfg.APIKey,
		BaseURL:     cfg.BaseURL,
		Model:       cfg.Model,
		Timeout:     cfg.Timeout,
		MaxTokens:   cfg.MaxTokens,
		Temperature: cfg.Temperature,
	})

	prompt := buildMetadataPrompt(sourceText)
	content, usage, err := client.ChatCompletionWithUsage(
		"你是一个专业的视频内容分析助手，擅长根据视频字幕或原始视频信息生成适合 B 站的中文标题、简介和标签。",
		prompt,
	)
	if err != nil {
		g.App.Logger.Errorf("❌ 调用 OpenAI 兼容元数据 API 失败: %v", err)
		return g.executeWithFreeFallback(taskContext)
	}
	if usage != nil {
		g.App.Logger.Infof("💰 OpenAI兼容元数据 Token使用: 输入=%d, 输出=%d, 总计=%d",
			usage.PromptTokens,
			usage.CompletionTokens,
			usage.TotalTokens)
	}

	metadata, err := parseMetadataResponse(content)
	if err != nil {
		g.App.Logger.Errorf("❌ 解析 OpenAI 兼容元数据失败: %v", err)
		return g.executeWithFreeFallback(taskContext)
	}

	return g.saveMetadataResults(metadata, taskContext)
}

func (g *GenerateMetadata) metadataSourceText() (string, error) {
	for _, name := range []string{"zh.srt", fmt.Sprintf("%s.srt", g.StateManager.VideoID), "en.srt"} {
		path := filepath.Join(g.StateManager.CurrentDir, name)
		content, err := os.ReadFile(path)
		if err == nil {
			text := g.extractTextFromSRT(string(content))
			if strings.TrimSpace(text) != "" {
				return text, nil
			}
		}
	}

	if g.SavedVideoService != nil {
		if savedVideo, err := g.SavedVideoService.GetVideoByVideoID(g.StateManager.VideoID); err == nil && savedVideo != nil {
			text := strings.TrimSpace(savedVideo.Title + "\n" + savedVideo.Description)
			if text != "" {
				return text, nil
			}
		}
	}

	return "", fmt.Errorf("未找到字幕或原始视频文本")
}

func buildMetadataPrompt(sourceText string) string {
	return fmt.Sprintf(`请根据以下视频内容信息，生成一个适合发布到 B 站的中文标题、简介和3-5个标签。

内容信息：
%s

要求：
1. 标题简洁有吸引力，严格控制在30个中文字符以内
2. 简介用中文说明视频主要内容和亮点，控制在600字以内
3. 标签准确反映内容，3-5个即可
4. 只返回 JSON，不要返回 Markdown 代码块或额外说明
5. JSON 格式必须是：
{
  "title": "视频标题",
  "description": "视频描述",
  "tags": ["标签1", "标签2", "标签3"]
}`, sourceText)
}

func parseMetadataResponse(content string) (*VideoMetadata, error) {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
	}
	content = strings.TrimSpace(content)

	var metadata VideoMetadata
	if err := json.Unmarshal([]byte(content), &metadata); err != nil {
		return nil, fmt.Errorf("解析元数据JSON失败: %v, 内容: %s", err, content)
	}
	if metadata.Title == "" {
		return nil, fmt.Errorf("生成的标题为空")
	}
	return &metadata, nil
}

// saveMetadataToFile 保存元数据到 meta.json 文件
func (g *GenerateMetadata) saveMetadataToFile(metadata *VideoMetadata) error {
	// 构建文件路径
	metaFilePath := filepath.Join(g.StateManager.CurrentDir, "meta.json")

	// 创建一个包含更多信息的元数据结构
	fileMetadata := map[string]interface{}{
		"video_id":     g.StateManager.VideoID,
		"title":        metadata.Title,
		"description":  metadata.Description,
		"tags":         metadata.Tags,
		"generated_at": time.Now().Format("2006-01-02 15:04:05"),
	}

	// 转换为格式化的JSON
	jsonData, err := json.MarshalIndent(fileMetadata, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化元数据失败: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(metaFilePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入meta.json文件失败: %v", err)
	}

	g.App.Logger.Infof("📁 meta.json 文件已保存: %s", metaFilePath)
	return nil
}

// truncateString 截断字符串用于日志显示
func (g *GenerateMetadata) truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// executeWithGeminiVideo 使用 Gemini 分析视频文件生成元数据
func (g *GenerateMetadata) executeWithGeminiVideo(taskContext map[string]interface{}) bool {
	g.App.Logger.Info("🎬 使用 Gemini 多模态分析视频文件...")

	// 1. 创建 Gemini 客户端
	client, err := NewGeminiClient(
		g.App.Config.GeminiConfig.ApiKey,
		g.App.Config.GeminiConfig.Model,
		g.App.Config.GeminiConfig.Timeout,
		g.App.Config.GeminiConfig.MaxTokens,
	)
	if err != nil {
		g.App.Logger.Errorf("❌ 创建 Gemini 客户端失败: %v", err)
		return false
	}
	defer client.Close()

	// 2. 查找视频文件
	videoFiles := g.findVideoFiles()
	if len(videoFiles) == 0 {
		g.App.Logger.Warn("⚠️ 未找到视频文件")
		return false
	}
	videoPath := videoFiles[0]
	g.App.Logger.Infof("📹 找到视频文件: %s", filepath.Base(videoPath))

	// 3. 上传视频到 Gemini
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(g.App.Config.GeminiConfig.Timeout)*time.Second)
	defer cancel()

	g.App.Logger.Info("⏫ 上传视频到 Gemini...")
	uploadedFile, err := client.UploadFile(ctx, videoPath, filepath.Base(videoPath))
	if err != nil {
		g.App.Logger.Errorf("❌ 上传视频失败: %v", err)
		return false
	}
	g.App.Logger.Infof("✓ 视频上传成功: %s", uploadedFile.Name)

	// 4. 等待文件处理完成
	g.App.Logger.Info("⏳ 等待 Gemini 处理视频...")
	if err := client.WaitForFileProcessing(ctx, uploadedFile); err != nil {
		g.App.Logger.Errorf("❌ 视频处理失败: %v", err)
		return false
	}
	g.App.Logger.Info("✓ 视频处理完成")

	// 5. 生成元数据
	g.App.Logger.Info("🤖 调用 Gemini 生成元数据...")
	metadata, err := client.GenerateMetadataFromVideo(ctx, uploadedFile)
	if err != nil {
		g.App.Logger.Errorf("❌ 生成元数据失败: %v", err)
		return false
	}

	// 6. 保存结果
	return g.saveMetadataResults(metadata, taskContext)
}

// executeWithGeminiText 使用 Gemini 分析字幕文本生成元数据
func (g *GenerateMetadata) executeWithGeminiText(taskContext map[string]interface{}) bool {
	g.App.Logger.Info("📝 使用 Gemini 分析字幕文本...")

	// 1. 检查中文字幕文件
	zhSRTPath := filepath.Join(g.StateManager.CurrentDir, "zh.srt")
	if _, err := os.Stat(zhSRTPath); os.IsNotExist(err) {
		g.App.Logger.Warn("⚠️ 中文字幕文件不存在")
		return false
	}

	// 2. 读取字幕内容
	srtContent, err := os.ReadFile(zhSRTPath)
	if err != nil {
		g.App.Logger.Errorf("❌ 读取字幕文件失败: %v", err)
		return false
	}

	// 3. 提取文本
	subtitleText := g.extractTextFromSRT(string(srtContent))
	if subtitleText == "" {
		g.App.Logger.Warn("⚠️ 字幕内容为空")
		return false
	}

	g.App.Logger.Infof("📝 提取到字幕文本，总长度: %d 字符", len(subtitleText))

	// 4. 截取文本（避免token过多）
	maxLength := 2000
	if len(subtitleText) > maxLength {
		subtitleText = subtitleText[:maxLength] + "..."
	}

	// 5. 创建 Gemini 客户端
	client, err := NewGeminiClient(
		g.App.Config.GeminiConfig.ApiKey,
		g.App.Config.GeminiConfig.Model,
		g.App.Config.GeminiConfig.Timeout,
		g.App.Config.GeminiConfig.MaxTokens,
	)
	if err != nil {
		g.App.Logger.Errorf("❌ 创建 Gemini 客户端失败: %v", err)
		return false
	}
	defer client.Close()

	// 6. 生成元数据
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(g.App.Config.GeminiConfig.Timeout)*time.Second)
	defer cancel()

	g.App.Logger.Info("🤖 调用 Gemini 生成元数据...")
	metadata, err := client.GenerateMetadataFromText(ctx, subtitleText)
	if err != nil {
		g.App.Logger.Errorf("❌ 生成元数据失败: %v", err)
		return false
	}

	// 7. 保存结果
	return g.saveMetadataResults(metadata, taskContext)
}

// saveMetadataResults 保存元数据结果到context和数据库
func (g *GenerateMetadata) saveMetadataResults(metadata *VideoMetadata, taskContext map[string]interface{}) bool {
	// 1. 验证标题长度
	if len([]rune(metadata.Title)) > 80 {
		runes := []rune(metadata.Title)
		metadata.Title = string(runes[:77]) + "..."
		g.App.Logger.Warnf("⚠️ 标题过长，已截断为80字符")
	}

	// 2. 保存到 context
	taskContext["video_title"] = metadata.Title
	taskContext["video_description"] = metadata.Description
	taskContext["video_tags"] = metadata.Tags

	// 3. 保存到 meta.json 文件
	g.App.Logger.Info("💾 保存元数据到 meta.json 文件...")
	if err := g.saveMetadataToFile(metadata); err != nil {
		g.App.Logger.Errorf("❌ 保存 meta.json 文件失败: %v", err)
	} else {
		g.App.Logger.Info("✅ meta.json 文件已保存")
	}

	// 4. 保存到数据库
	if g.SavedVideoService == nil {
		g.App.Logger.Warn("⚠️ 未配置 SavedVideoService，跳过保存元数据到数据库")
		return true
	}

	g.App.Logger.Info("💾 保存生成的元数据到数据库...")
	savedVideo, err := g.SavedVideoService.GetVideoByVideoID(g.StateManager.VideoID)
	if err != nil {
		g.App.Logger.Errorf("❌ 获取视频记录失败: %v", err)
	} else {
		savedVideo.GeneratedTitle = metadata.Title
		savedVideo.GeneratedDesc = metadata.Description
		savedVideo.GeneratedTags = strings.Join(metadata.Tags, ",")

		if err := g.SavedVideoService.UpdateVideo(savedVideo); err != nil {
			g.App.Logger.Errorf("❌ 保存元数据到数据库失败: %v", err)
		} else {
			g.App.Logger.Info("✅ 元数据已保存到数据库")
		}
	}

	// 5. 输出生成结果
	g.App.Logger.Info("========================================")
	g.App.Logger.Info("✅ 视频元数据生成成功！")
	g.App.Logger.Infof("📌 标题: %s", metadata.Title)
	g.App.Logger.Infof("📝 描述: %s", g.truncateString(metadata.Description, 100))
	g.App.Logger.Infof("🏷️ 标签: %v", metadata.Tags)
	g.App.Logger.Info("========================================")

	return true
}

// findVideoFiles 查找视频文件
func (g *GenerateMetadata) findVideoFiles() []string {
	var videoFiles []string
	videoExtensions := []string{".mp4", ".flv", ".mkv", ".webm", ".avi", ".mov"}

	files, err := os.ReadDir(g.StateManager.CurrentDir)
	if err != nil {
		g.App.Logger.Errorf("读取目录失败: %v", err)
		return videoFiles
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(file.Name()))
		for _, videoExt := range videoExtensions {
			if ext == videoExt {
				fullPath := filepath.Join(g.StateManager.CurrentDir, file.Name())
				videoFiles = append(videoFiles, fullPath)
				break
			}
		}
	}

	return videoFiles
}
