package handler

import (
	"encoding/json"
	"fmt"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/pkg/auth"
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"github.com/ZhantaoLi/ytb2bili/pkg/utils"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SubtitleHandler struct {
	BaseHandler
}

func NewSubtitleHandler(app *core.AppServer) *SubtitleHandler {

	return &SubtitleHandler{
		BaseHandler: BaseHandler{App: app},
	}
}

// SaveVideoRequest 保存视频请求
type SaveVideoRequest struct {
	URL           string                     `json:"url" binding:"required"`
	Title         string                     `json:"title"`
	Description   string                     `json:"description"`
	OperationType string                     `json:"operationType"`
	Subtitles     []model.SavedVideoSubtitle `json:"subtitles"`
	PlaylistID    string                     `json:"playlistId"`
	Timestamp     string                     `json:"timestamp"`
	SavedAt       string                     `json:"savedAt"`
	Meta          string                     `json:"meta"` // 加密的 cookies 数据
}

// Cookie 结构体（兼容 Chrome cookies API）
type Cookie struct {
	Domain         string  `json:"domain"`
	ExpirationDate float64 `json:"expirationDate"`
	HostOnly       bool    `json:"hostOnly"`
	HTTPOnly       bool    `json:"httpOnly"`
	Name           string  `json:"name"`
	Path           string  `json:"path"`
	SameSite       string  `json:"sameSite"`
	Secure         bool    `json:"secure"`
	Session        bool    `json:"session"`
	StoreID        string  `json:"storeId"`
	Value          string  `json:"value"`
}

func (h *SubtitleHandler) saveVideoSubtitles(c *gin.Context) {
	var req SaveVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request parameters: " + err.Error(),
		})
		return
	}

	fmt.Println("========================================")
	fmt.Println("📥 用户调用保存视频接口")
	fmt.Printf("🔗 URL: %s\n", req.URL)
	fmt.Printf("📺 标题: %s\n", req.Title)
	fmt.Printf("🎬 操作类型: %s\n", req.OperationType)
	fmt.Println("========================================")

	// 从 context 获取解密后的 cookies（由 DecryptCookies 中间件解密）
	if cookiesStr, exists := c.Get("decryptedCookies"); exists {
		if cookies, ok := cookiesStr.(string); ok && cookies != "" {
			// 保存 cookies 到文件
			if err := h.saveCookiesToFile(cookies); err != nil {
				fmt.Printf("⚠️ 保存 cookies 文件失败: %v\n", err)
				// 不阻止视频保存流程，只记录警告
			} else {
				fmt.Printf("✅ Cookies 已保存到文件\n")
			}
		}
	}

	// 从 URL 中提取 videoId
	videoID := utils.ExtractVideoID(req.URL)
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid video URL: cannot extract video ID",
		})
		return
	}
	fmt.Println("Extracted videoId:", videoID)

	// 将字幕数组转换为JSON字符串
	subtitlesJSON, err := json.Marshal(req.Subtitles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to marshal subtitles: " + err.Error(),
		})
		return
	}

	// 检查字幕数据大小
	subtitlesJSONStr := string(subtitlesJSON)
	fmt.Printf("字幕数据长度: %d 字符\n", len(subtitlesJSONStr))
	fmt.Printf("字幕条目数量: %d\n", len(req.Subtitles))

	// 如果数据太大，截断前100个字符用于调试
	if len(subtitlesJSONStr) > 100 {
		fmt.Printf("字幕数据前100字符: %s...\n", subtitlesJSONStr[:100])
	} else {
		fmt.Printf("字幕数据: %s\n", subtitlesJSONStr)
	}

	// 检查是否已存在相同的 videoId（包括已删除的记录）
	var existingVideo model.SavedVideo
	err = h.App.DB.Unscoped().Where("video_id = ?", videoID).First(&existingVideo).Error

	var savedVideo *model.SavedVideo
	isExisting := false

	if err == nil {
		// 找到了记录（可能是已删除的），更新字段
		isExisting = true
		existingVideo.URL = req.URL
		existingVideo.Title = req.Title
		existingVideo.Description = req.Description
		existingVideo.OperationType = req.OperationType
		existingVideo.Subtitles = subtitlesJSONStr
		existingVideo.PlaylistID = req.PlaylistID
		existingVideo.Timestamp = req.Timestamp
		existingVideo.SavedAt = req.SavedAt
		existingVideo.Status = "001"               // 重置状态为待处理
		existingVideo.DeletedAt = gorm.DeletedAt{} // 恢复记录（清除删除标记）

		// 更新到数据库（使用 Unscoped 以便更新已删除的记录）
		if err := h.App.DB.Unscoped().Save(&existingVideo).Error; err != nil {
			fmt.Printf("更新视频失败，字幕数据长度: %d\n", len(subtitlesJSONStr))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to update video: " + err.Error(),
			})
			return
		}
		savedVideo = &existingVideo

		if existingVideo.DeletedAt.Valid {
			fmt.Printf("✅ 恢复已删除的视频: %s\n", videoID)
		}
	} else if err == gorm.ErrRecordNotFound {
		// 记录不存在，创建新记录
		savedVideo = &model.SavedVideo{
			VideoID:       videoID,
			URL:           req.URL,
			Title:         req.Title,
			Status:        "001",
			Description:   req.Description,
			OperationType: req.OperationType,
			Subtitles:     subtitlesJSONStr,
			PlaylistID:    req.PlaylistID,
			Timestamp:     req.Timestamp,
			SavedAt:       req.SavedAt,
		}

		// 保存到数据库
		if err := h.App.DB.Create(savedVideo).Error; err != nil {
			fmt.Printf("创建视频失败，字幕数据长度: %d\n", len(subtitlesJSONStr))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to save video: " + err.Error(),
			})
			return
		}
	} else {
		// 数据库查询出错
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Database error: " + err.Error(),
		})
		return
	}

	// 计算字幕数量
	subtitleCount := len(req.Subtitles)

	message := "Video saved successfully"
	if isExisting {
		message = "Video updated successfully"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data": gin.H{
			"id":            savedVideo.ID,
			"title":         savedVideo.Title,
			"operationType": savedVideo.OperationType,
			"subtitleCount": subtitleCount,
			"isExisting":    isExisting,
		},
	})
}

// RegisterRoutes 注册上传相关路由（无认证）
func (h *SubtitleHandler) RegisterRoutes(server *core.AppServer) {
	api := server.Engine.Group("/api/v1")
	api.POST("/submit", h.saveVideoSubtitles)
}

// RegisterRoutesWithAuth 注册上传相关路由（带认证和解密）
func (h *SubtitleHandler) RegisterRoutesWithAuth(server *core.AppServer, authMiddleware *auth.Middleware, decryptKey string) {
	api := server.Engine.Group("/api/v1")

	// 创建解密中间件
	if decryptKey == "" {
		api.POST("/submit", authMiddleware.Handler(), h.saveVideoSubtitles)
		return
	}
	decryptMiddleware := auth.DecryptCookies(decryptKey)

	// 为 /submit 路由添加认证中间件和解密中间件
	api.POST("/submit", authMiddleware.Handler(), decryptMiddleware, h.saveVideoSubtitles)
}

// saveCookiesToFile 保存 cookies 到文件（Netscape 格式）
func (h *SubtitleHandler) saveCookiesToFile(cookiesStr string) error {
	// 创建 cookies 目录
	cookiesDir := filepath.Join(h.App.Config.DataPath, "cookies")
	if err := os.MkdirAll(cookiesDir, 0755); err != nil {
		return fmt.Errorf("创建 cookies 目录失败: %w", err)
	}

	// 生成文件名（使用时间戳）
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("cookies_%s.txt", timestamp)
	filepath := filepath.Join(cookiesDir, filename)

	// 转换为 Netscape 格式
	netscapeContent, err := h.convertToNetscapeFormat(cookiesStr)
	if err != nil {
		return fmt.Errorf("转换 Netscape 格式失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filepath, []byte(netscapeContent), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	fmt.Printf("📁 Cookies 文件已保存: %s\n", filepath)

	// 清理旧文件（保留最近 10 个）
	h.cleanupOldCookiesFiles(cookiesDir, 10)

	return nil
}

// convertToNetscapeFormat 将 cookies JSON 转换为 Netscape 格式
func (h *SubtitleHandler) convertToNetscapeFormat(cookiesStr string) (string, error) {
	var cookies []Cookie

	// 尝试解析为 JSON 数组
	if err := json.Unmarshal([]byte(cookiesStr), &cookies); err != nil {
		// 解析失败，尝试 fallback 格式（name=value 格式）
		fmt.Printf("⚠️ JSON 解析失败，尝试 fallback 格式: %v\n", err)

		// 简单的 fallback：直接使用原始字符串
		// 假设格式可能是 "name=value; name2=value2"
		lines := []string{
			"# Netscape HTTP Cookie File",
			"# This is a generated file! Do not edit.",
			"",
		}

		// 尝试解析简单的 key=value 格式
		parts := strings.Split(cookiesStr, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			// 分割 name=value
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}

			name := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])

			// Netscape 格式：domain	flag	path	secure	expiration	name	value
			// 使用默认值
			line := fmt.Sprintf(".youtube.com\tTRUE\t/\tFALSE\t0\t%s\t%s", name, value)
			lines = append(lines, line)
		}

		return strings.Join(lines, "\n"), nil
	}

	// 成功解析为 Cookie 数组，转换为 Netscape 格式
	var lines []string
	lines = append(lines, "# Netscape HTTP Cookie File")
	lines = append(lines, "# This is a generated file! Do not edit.")
	lines = append(lines, "")

	for _, cookie := range cookies {
		// Netscape 格式：
		// domain	flag	path	secure	expiration	name	value

		domain := cookie.Domain
		if domain == "" {
			domain = ".youtube.com"
		}

		// flag: TRUE 表示所有子域名都可以访问
		flag := "FALSE"
		if cookie.HostOnly {
			flag = "FALSE"
		} else {
			flag = "TRUE"
		}

		path := cookie.Path
		if path == "" {
			path = "/"
		}

		secure := "FALSE"
		if cookie.Secure {
			secure = "TRUE"
		}

		// 过期时间（Unix 时间戳）
		expiration := "0"
		if cookie.ExpirationDate > 0 {
			expiration = strconv.FormatInt(int64(cookie.ExpirationDate), 10)
		}

		// 构建 Netscape 格式行
		line := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s",
			domain,
			flag,
			path,
			secure,
			expiration,
			cookie.Name,
			cookie.Value,
		)

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

// cleanupOldCookiesFiles 清理旧的 cookies 文件
func (h *SubtitleHandler) cleanupOldCookiesFiles(dir string, keepCount int) {
	// 读取目录
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("⚠️ 读取 cookies 目录失败: %v\n", err)
		return
	}

	// 过滤出 cookies 文件
	var cookiesFiles []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "cookies_") && strings.HasSuffix(entry.Name(), ".txt") {
			cookiesFiles = append(cookiesFiles, entry)
		}
	}

	// 如果文件数量少于等于 keepCount，不需要清理
	if len(cookiesFiles) <= keepCount {
		return
	}

	// 按修改时间排序（从旧到新）
	sort.Slice(cookiesFiles, func(i, j int) bool {
		infoI, errI := cookiesFiles[i].Info()
		infoJ, errJ := cookiesFiles[j].Info()
		if errI != nil || errJ != nil {
			return false
		}
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	// 删除多余的旧文件
	deleteCount := len(cookiesFiles) - keepCount
	for i := 0; i < deleteCount; i++ {
		filePath := filepath.Join(dir, cookiesFiles[i].Name())
		if err := os.Remove(filePath); err != nil {
			fmt.Printf("⚠️ 删除旧 cookies 文件失败: %s, error: %v\n", filePath, err)
		} else {
			fmt.Printf("🗑️  已删除旧 cookies 文件: %s\n", cookiesFiles[i].Name())
		}
	}
}
