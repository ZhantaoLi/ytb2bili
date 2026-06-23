package handler

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/models"
	"github.com/ZhantaoLi/ytb2bili/internal/storage"
	"github.com/ZhantaoLi/ytb2bili/pkg/auth"
	"github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/google/uuid"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"gorm.io/gorm"
)

type AuthHandler struct {
	BaseHandler
	JWTManager *auth.JWTManager
}

var errAdminNotConfigured = errors.New("admin user is not configured")

func isForbiddenDefaultAdmin(username, password string) bool {
	return username == "admin" && password == strings.Join([]string{"admin", "123"}, "")
}

func NewAuthHandler(app *core.AppServer) *AuthHandler {
	// 初始化 JWT 管理器
	jwtConfig := &auth.JWTConfig{
		SecretKey:   app.Config.Auth.JWTSecret,
		ExpiryTime:  time.Duration(app.Config.Auth.JWTExpiration) * time.Hour,
		RefreshTime: 30 * 24 * time.Hour,
	}

	handler := &AuthHandler{
		BaseHandler: BaseHandler{App: app},
		JWTManager:  auth.NewJWTManager(jwtConfig),
	}

	return handler
}

// RegisterRoutes 注册认证相关路由
func (h *AuthHandler) RegisterRoutes(server *core.AppServer) {
	publicAuth := server.Engine.Group("/api/v1/auth")
	{
		publicAuth.POST("/admin/login", h.adminLogin)
	}

	server.Engine.Use(h.adminAuthMiddleware())

	authRoutes := server.Engine.Group("/api/v1/auth")
	{
		authRoutes.POST("/admin/logout", h.adminLogout)
		authRoutes.GET("/admin/validate", h.validateToken)
		authRoutes.GET("/qrcode", h.getQRCode)
		authRoutes.GET("/qrcode/image/:authCode", h.getQRCodeImage)
		authRoutes.POST("/poll", h.pollQRCode)
		authRoutes.GET("/login", h.loadLoginInfo)
		authRoutes.GET("/status", h.checkLoginStatus)
		authRoutes.GET("/userinfo", h.getUserInfo)
		authRoutes.POST("/logout", h.logout)
	}
}

func (h *AuthHandler) adminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/api/v1/submit" && h.App != nil && h.App.Config != nil &&
			h.App.Config.APIAuth.AppID != "" && h.App.Config.APIAuth.AppSecret != "" {
			c.Next()
			return
		}
		if !requiresAdminAuth(c.Request.URL.Path) {
			c.Next()
			return
		}

		claims, ok := h.validateAdminRequest(c)
		if !ok {
			return
		}
		c.Set("admin_user_id", claims.UserID)
		c.Set("admin_username", claims.Username)
		c.Next()
	}
}

func requiresAdminAuth(path string) bool {
	if !strings.HasPrefix(path, "/api/v1/") || path == "/api/v1/auth/admin/login" {
		return false
	}

	protectedPrefixes := []string{
		"/api/v1/auth/",
		"/api/v1/submit",
		"/api/v1/upload",
		"/api/v1/config",
		"/api/v1/accounts",
		"/api/v1/videos",
	}
	for _, prefix := range protectedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func (h *AuthHandler) validateAdminRequest(c *gin.Context) (*auth.Claims, bool) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Authorization header required",
		})
		c.Abort()
		return nil, false
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Invalid authorization format",
		})
		c.Abort()
		return nil, false
	}

	claims, err := h.JWTManager.ValidateToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Invalid or expired token",
		})
		c.Abort()
		return nil, false
	}

	return claims, true
}

func (h *AuthHandler) registerRoutesLegacy(server *core.AppServer) {
	panic("legacy auth route registration is disabled")
}

// AdminLoginRequest 管理员登录请求
type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AdminLoginResponse 管理员登录响应
type AdminLoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      gin.H     `json:"user"`
}

// adminLogin 管理员登录
func (h *AuthHandler) adminLogin(c *gin.Context) {
	var req AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters: " + err.Error(),
		})
		return
	}
	if isForbiddenDefaultAdmin(req.Username, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Default admin credentials are disabled. Configure a non-default admin password.",
		})
		return
	}

	// 确保管理员用户存在
	adminUser, err := h.ensureAdminUser()
	if err != nil {
		log.Printf("Failed to ensure admin user: %v", err)
		if errors.Is(err, errAdminNotConfigured) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Admin user is not configured. Set YTB2BILI_ADMIN_USERNAME and YTB2BILI_ADMIN_PASSWORD before first login.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Internal server error",
		})
		return
	}

	// 验证用户名
	if req.Username != adminUser.Username {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Invalid username or password",
		})
		return
	}

	// 验证密码
	if err := adminUser.CheckPassword(req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Invalid username or password",
		})
		return
	}

	// 生成 JWT token
	token, err := h.JWTManager.GenerateToken(adminUser.Id, adminUser.Email, adminUser.Username)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to generate token",
		})
		return
	}

	// 计算过期时间
	expiresAt := time.Now().Add(time.Duration(h.App.Config.Auth.JWTExpiration) * time.Hour)

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "Login successful",
		Data: AdminLoginResponse{
			Token:     token,
			ExpiresAt: expiresAt,
			User: gin.H{
				"id":       adminUser.Id,
				"username": adminUser.Username,
				"email":    adminUser.Email,
				"nickname": adminUser.NickName,
			},
		},
	})
}

// adminLogout 管理员登出
func (h *AuthHandler) adminLogout(c *gin.Context) {
	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "Logout successful",
	})
}

// validateToken 验证 token
func (h *AuthHandler) validateToken(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Authorization header required",
		})
		return
	}

	// 提取 token
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Invalid authorization format",
		})
		return
	}

	// 验证 token
	claims, err := h.JWTManager.ValidateToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Invalid or expired token",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "Token is valid",
		Data: gin.H{
			"user_id":  claims.UserID,
			"username": claims.Username,
			"email":    claims.Email,
		},
	})
}

// ensureAdminUser 确保管理员用户存在，如果不存在则创建
func (h *AuthHandler) ensureAdminUser() (*models.TBUser, error) {
	db := h.App.DB
	username := strings.TrimSpace(os.Getenv("YTB2BILI_ADMIN_USERNAME"))
	password := os.Getenv("YTB2BILI_ADMIN_PASSWORD")
	email := strings.TrimSpace(os.Getenv("YTB2BILI_ADMIN_EMAIL"))

	lookupUsername := username
	if lookupUsername == "" {
		lookupUsername = "admin"
	}

	var adminUser models.TBUser
	result := db.Where("user_name = ?", lookupUsername).First(&adminUser)
	if result.Error == nil {
		return &adminUser, nil
	}
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}

	if username == "" || password == "" {
		return nil, errAdminNotConfigured
	}
	if isForbiddenDefaultAdmin(username, password) {
		return nil, errors.New("refusing weak built-in admin credentials")
	}
	if email == "" {
		email = username + "@ytb2bili.local"
	}

	adminUser = models.TBUser{
		Id:         uuid.New().String(),
		Username:   username,
		Email:      email,
		NickName:   "Administrator",
		Status:     "active",
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
	if err := adminUser.HashPassword(password); err != nil {
		return nil, err
	}
	if err := db.Create(&adminUser).Error; err != nil {
		return nil, err
	}

	log.Printf("Admin user created from explicit environment configuration (username: %s)", username)
	return &adminUser, nil
}

func (h *AuthHandler) ensureAdminUserLegacy() (*models.TBUser, error) {
	return nil, errors.New("legacy admin bootstrap is disabled")
}

// QRCodeRequest 二维码请求
type QRCodeRequest struct{}

// QRCodeResponse 二维码响应
type QRCodeResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	QRCodeURL string `json:"qr_code_url"`
	AuthCode  string `json:"auth_code"`
}

// getQRCode 获取登录二维码
func (h *AuthHandler) getQRCode(c *gin.Context) {
	client := bilibili.NewClient()

	qrResp, err := client.GetQRCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get QR code: " + err.Error(),
		})
		return
	}

	if qrResp.Code != 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    qrResp.Code,
			"message": "Failed to get QR code",
		})
		return
	}

	// 构造完整的二维码URL
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	host := c.Request.Host
	fullQRCodeURL := fmt.Sprintf("%s://%s/api/v1/auth/qrcode/image/%s", scheme, host, qrResp.Data.AuthCode)

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "success",
		Data: gin.H{
			"qrcode_url": fullQRCodeURL,
			"auth_code":  qrResp.Data.AuthCode,
		},
	})
}

// getQRCodeImage 生成二维码图片
func (h *AuthHandler) getQRCodeImage(c *gin.Context) {
	authCode := c.Param("authCode")
	if authCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Auth code is required",
		})
		return
	}

	// 构造B站二维码URL
	qrURL := fmt.Sprintf("https://passport.bilibili.com/x/passport-tv-login/h5/qrcode/auth?auth_code=%s", authCode)

	// 生成二维码图片
	qrCode, err := qrcode.New(qrURL, qrcode.Medium)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to generate QR code: " + err.Error(),
		})
		return
	}

	// 设置二维码颜色
	qrCode.BackgroundColor = color.RGBA{255, 255, 255, 255} // 白色背景
	qrCode.ForegroundColor = color.RGBA{0, 0, 0, 255}       // 黑色前景

	// 生成PNG图片
	img := qrCode.Image(240)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to encode QR code image: " + err.Error(),
		})
		return
	}

	// 设置响应头
	c.Header("Content-Type", "image/png")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// 返回图片数据
	c.Data(http.StatusOK, "image/png", buf.Bytes())
}

// PollQRCodeRequest 轮询二维码请求
type PollQRCodeRequest struct {
	AuthCode string `json:"auth_code" binding:"required"`
}

// PollQRCodeResponse 轮询二维码响应
type PollQRCodeResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// pollQRCode 轮询二维码登录状态
func (h *AuthHandler) pollQRCode(c *gin.Context) {
	var req PollQRCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters: " + err.Error(),
		})
		return
	}

	fmt.Println("--轮询二维码--")

	client := bilibili.NewClient()

	loginInfo, err := client.PollQRCode(req.AuthCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Login failed: " + err.Error(),
		})
		return
	}

	// 获取用户完整信息并补充到LoginInfo中
	var userBasicInfo *storage.UserBasicInfo
	if loginInfo.TokenInfo.Mid > 0 {
		// 构建cookie字符串用于API调用
		cookies := buildCookieString(loginInfo.CookieInfo)

		// 优先使用myinfo API获取完整用户信息 (参考biliup-1.1.16)
		myInfo, err := client.GetMyInfoWithRetry(cookies, 2)
		if err == nil {
			// 使用myinfo API的完整信息
			loginInfo.TokenInfo.Uname = myInfo.Uname
			loginInfo.TokenInfo.Face = myInfo.Face
			if myInfo.Mid > 0 {
				loginInfo.TokenInfo.Mid = myInfo.Mid
			}
			// 转换为存储格式
			userBasicInfo = storage.ConvertMyInfoToUserInfo(myInfo)
		} else {
			log.Printf("Warning: Failed to get myinfo: %v", err)
		}
	} // 登录成功后自动保存到本地（包括用户信息）
	store := storage.GetDefaultStore()
	if userBasicInfo != nil {
		// 保存登录信息和用户信息
		if err := store.SaveWithUserInfo(loginInfo, userBasicInfo); err != nil {
			log.Printf("Warning: Failed to save login info with user info: %v", err)
			// 回退到只保存登录信息
			if err := store.Save(loginInfo); err != nil {
				log.Printf("Warning: Failed to save login info: %v", err)
			}
		}
	} else {
		// 只保存登录信息
		if err := store.Save(loginInfo); err != nil {
			log.Printf("Warning: Failed to save login info: %v", err)
		}
	}

	c.JSON(http.StatusOK, PollQRCodeResponse{
		Code:    0,
		Message: "Login successful",
		Data: gin.H{
			"bilibili_user": publicBilibiliUser(loginInfo),
		},
	})
}

// LoadLoginInfoResponse 加载登录信息响应
type LoadLoginInfoResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// loadLoginInfo 从本地加载已保存的登录信息
func (h *AuthHandler) loadLoginInfo(c *gin.Context) {
	store := storage.GetDefaultStore()

	loginInfo, err := store.Load()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "No saved login info or login expired: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "Login info loaded successfully",
		Data: gin.H{
			"bilibili_connected": true,
			"bilibili_user":      publicBilibiliUser(loginInfo),
		},
	})
}

// CheckLoginStatusResponse 检查登录状态响应
type CheckLoginStatusResponse struct {
	Code              int               `json:"code"`
	Message           string            `json:"message"`
	IsLoggedIn        bool              `json:"is_logged_in"`            // 用户是否登录
	BilibiliConnected bool              `json:"bilibili_connected"`      // B站账号是否已绑定
	BilibiliUser      *BilibiliUserInfo `json:"bilibili_user,omitempty"` // B站用户信息
}

// BilibiliUserInfo B站用户信息
type BilibiliUserInfo struct {
	Mid    string `json:"mid"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// UserInfo B站用户信息
type UserInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Mid    string `json:"mid"`
	Avatar string `json:"avatar"`
}

// checkLoginStatus 检查B站账号登录状态
func publicBilibiliUser(loginInfo *bilibili.LoginInfo) gin.H {
	if loginInfo == nil {
		return gin.H{}
	}

	return gin.H{
		"mid":    fmt.Sprintf("%d", loginInfo.TokenInfo.Mid),
		"name":   loginInfo.TokenInfo.Uname,
		"avatar": loginInfo.TokenInfo.Face,
	}
}

func (h *AuthHandler) checkLoginStatus(c *gin.Context) {
	store := storage.GetDefaultStore()
	bilibiliConnected := store.IsValid()

	// 响应数据
	responseData := gin.H{
		"is_logged_in":       bilibiliConnected,
		"bilibili_connected": bilibiliConnected,
	}

	// 检查B站账号绑定状态，如果已绑定则返回B站用户信息
	if bilibiliConnected {
		// 优先从缓存中获取B站用户信息
		cachedUserInfo, err := store.GetUserInfo()
		if err == nil && cachedUserInfo != nil {
			// 使用缓存的B站用户信息
			responseData["bilibili_user"] = gin.H{
				"mid":    fmt.Sprintf("%d", cachedUserInfo.Mid),
				"name":   cachedUserInfo.Name,
				"avatar": cachedUserInfo.Face,
			}
		} else {
			// 没有缓存的用户信息，从API获取
			loginInfo, _ := store.Load()
			if loginInfo != nil {
				client := bilibili.NewClient()

				// 构建cookie字符串
				cookies := buildCookieString(loginInfo.CookieInfo)

				// 尝试使用myinfo API获取完整用户信息
				userName := fmt.Sprintf("用户_%d", loginInfo.TokenInfo.Mid) // 默认用户名
				userAvatar := ""
				userMid := fmt.Sprintf("%d", loginInfo.TokenInfo.Mid)

				// 如果登录信息中有用户名，使用它
				if loginInfo.TokenInfo.Uname != "" {
					userName = loginInfo.TokenInfo.Uname
				}
				if loginInfo.TokenInfo.Face != "" {
					userAvatar = loginInfo.TokenInfo.Face
				}

				var userBasicInfo *storage.UserBasicInfo

				// 优先使用myinfo API获取最新用户信息
				myInfo, err := client.GetMyInfoWithRetry(cookies, 2)
				if err == nil {
					// 使用myinfo API的完整信息
					userName = myInfo.Uname
					userAvatar = myInfo.Face
					userMid = fmt.Sprintf("%d", myInfo.Mid)

					// 更新并保存登录信息和用户信息
					loginInfo.TokenInfo.Uname = myInfo.Uname
					loginInfo.TokenInfo.Face = myInfo.Face
					if myInfo.Mid > 0 {
						loginInfo.TokenInfo.Mid = myInfo.Mid
					}
					userBasicInfo = storage.ConvertMyInfoToUserInfo(myInfo)
				} else {
					log.Printf("Warning: Failed to get myinfo: %v", err)
				}

				// 保存更新后的信息（包括用户信息）
				if userBasicInfo != nil {
					store.SaveWithUserInfo(loginInfo, userBasicInfo)
				} else {
					store.Save(loginInfo)
				}

				responseData["bilibili_user"] = gin.H{
					"mid":    userMid,
					"name":   userName,
					"avatar": userAvatar,
				}
			}
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "success",
		Data:    responseData,
	})
}

// GetUserInfoResponse 获取用户信息响应
type GetUserInfoResponse struct {
	Code     int                     `json:"code"`
	Message  string                  `json:"message"`
	UserInfo *bilibili.UserBasicInfo `json:"user_info,omitempty"`
}

// getUserInfo 获取当前登录用户的详细信息
func (h *AuthHandler) getUserInfo(c *gin.Context) {
	store := storage.GetDefaultStore()
	if !store.IsValid() {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "User not logged in",
		})
		return
	}

	loginInfo, err := store.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to load login info: " + err.Error(),
		})
		return
	}

	client := bilibili.NewClient()

	// 构建cookie字符串
	cookies := buildCookieString(loginInfo.CookieInfo)

	// 优先使用myinfo API获取用户信息 (参考biliup-1.1.16)
	myInfo, err := client.GetMyInfoWithRetry(cookies, 3)
	if err != nil {
		log.Printf("Failed to get myinfo: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get user info: " + err.Error(),
		})
		return
	}

	// 使用myinfo API返回的完整信息
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    myInfo,
	})
}

// LogoutResponse 登出响应
type LogoutResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// logout 删除本地保存的登录信息（登出）
func (h *AuthHandler) logout(c *gin.Context) {
	store := storage.GetDefaultStore()

	if err := store.Delete(); err != nil {
		log.Printf("Warning: Failed to delete login info: %v", err)
	}

	c.JSON(http.StatusOK, LogoutResponse{
		Code:    0,
		Message: "Logout successful",
	})
}

// buildCookieString 构建正确的cookie字符串
func buildCookieString(cookieInfo map[string]interface{}) string {
	if cookieInfo == nil {
		return ""
	}

	// 检查是否是新的数组格式
	if cookies, ok := cookieInfo["cookies"].([]interface{}); ok {
		cookieParts := []string{}
		for _, cookie := range cookies {
			if cookieMap, ok := cookie.(map[string]interface{}); ok {
				if name, nameOk := cookieMap["name"].(string); nameOk {
					if value, valueOk := cookieMap["value"].(string); valueOk {
						cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", name, value))
					}
				}
			}
		}
		if len(cookieParts) > 0 {
			return strings.Join(cookieParts, "; ")
		}
	}

	// 回退到旧的key-value格式处理
	cookieParts := []string{}
	for key, value := range cookieInfo {
		if key == "cookies" || key == "domains" {
			continue // 跳过特殊字段
		}
		if valueStr, ok := value.(string); ok {
			cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", key, valueStr))
		}
	}

	if len(cookieParts) > 0 {
		return strings.Join(cookieParts, "; ")
	}

	return ""
}
