package handler

import (
	"fmt"
	"time"

	"github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/ZhantaoLi/ytb2bili/internal/storage"
	"github.com/ZhantaoLi/ytb2bili/pkg/services"
	"github.com/ZhantaoLi/ytb2bili/pkg/store"
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AccountsHandler 账号绑定处理器
type AccountsHandler struct {
	db             *gorm.DB
	accountService *services.BilibiliAccountService
	bindingCache   *store.CacheDict
	logger         *zap.SugaredLogger
}

// NewAccountsHandler 创建账号绑定处理器
func NewAccountsHandler(db *gorm.DB, accountService *services.BilibiliAccountService, logger *zap.SugaredLogger) *AccountsHandler {
	return &AccountsHandler{
		db:             db,
		accountService: accountService,
		bindingCache:   store.NewCacheDict(),
		logger:         logger,
	}
}

// RegisterRoutes 注册路由
func (h *AccountsHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/qrcode", h.GenerateBindingQRCode)   // 生成绑定二维码
	r.POST("/poll", h.PollBindingStatus)         // 轮询绑定状态
	r.GET("/list", h.GetBindingList)             // 获取绑定列表
	r.DELETE("/:id", h.UnbindAccount)            // 解绑账号
	r.POST("/primary", h.SetPrimaryAccount)      // 设置主账号
	r.POST("/refresh", h.RefreshToken)           // 刷新令牌
}

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(200, Response{
		Code:    200,
		Message: "success",
		Data:    data,
	})
}

// Error 错误响应
func Error(c *gin.Context, code int, message string) {
	c.JSON(code, Response{
		Code:    code,
		Message: message,
	})
}

// BadRequest 400错误
func BadRequest(c *gin.Context, message string) {
	Error(c, 400, message)
}

// NotFound 404错误
func NotFound(c *gin.Context, message string) {
	Error(c, 404, message)
}

// InternalServerError 500错误
func InternalServerError(c *gin.Context, message string) {
	Error(c, 500, message)
}

// NotImplemented 501错误
func NotImplemented(c *gin.Context, message string) {
	Error(c, 501, message)
}

// GenerateBindingQRCodeRequest 生成绑定二维码请求
type GenerateBindingQRCodeRequest struct {
	Platform string `json:"platform" binding:"required"` // 平台: bilibili, youtube, douyin, kuaishou
	UserID   string `json:"user_id" binding:"required"`  // Firebase用户ID
}

// GenerateBindingQRCodeResponse 生成绑定二维码响应
type GenerateBindingQRCodeResponse struct {
	QRCode    string `json:"qr_code"`     // 二维码URL
	QRCodeKey string `json:"qr_code_key"` // 二维码密钥
	ExpiresIn int64  `json:"expires_in"`  // 过期时间（秒）
}

// GenerateBindingQRCode godoc
// @Summary 生成账号绑定二维码
// @Description 为指定平台生成账号绑定二维码，用户扫码后可绑定账号
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param request body GenerateBindingQRCodeRequest true "生成二维码请求"
// @Success 200 {object} Response{data=GenerateBindingQRCodeResponse}
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/accounts/qrcode [post]
func (h *AccountsHandler) GenerateBindingQRCode(c *gin.Context) {
	var req GenerateBindingQRCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 验证平台
	validPlatforms := map[string]bool{
		"bilibili": true,
		"youtube":  true,
		"douyin":   true,
		"kuaishou": true,
	}
	if !validPlatforms[req.Platform] {
		BadRequest(c, "不支持的平台")
		return
	}

	// 检查是否已经绑定过该平台的账号（允许多账号绑定）
	// 这里不做限制，用户可以绑定多个同平台账号

	// 生成二维码key
	qrCodeKey := uuid.New().String()

	// 根据平台生成相应的二维码
	qrCode, authCode, err := h.generatePlatformQRCode(req.Platform, qrCodeKey)
	if err != nil {
		h.logger.Errorf("生成二维码失败: %v", err)
		InternalServerError(c, "生成二维码失败")
		return
	}

	// 将绑定信息存储到临时缓存中，设置5分钟过期
	bindingData := map[string]interface{}{
		"user_id":    req.UserID,
		"platform":   req.Platform,
		"qr_code":    qrCode,
		"auth_code":  authCode,
		"status":     "pending",
		"created_at": time.Now().Unix(),
	}

	h.bindingCache.Set(qrCodeKey, bindingData, 5*time.Minute)

	Success(c, GenerateBindingQRCodeResponse{
		QRCode:    qrCode,
		QRCodeKey: qrCodeKey,
		ExpiresIn: 300,
	})
}

// generatePlatformQRCode 根据平台生成二维码
func (h *AccountsHandler) generatePlatformQRCode(platform, qrCodeKey string) (qrCodeURL string, authCode string, err error) {
	switch platform {
	case "bilibili":
		client := bilibili.NewClient()
		qrResp, err := client.GetQRCode()
		if err != nil {
			return "", "", fmt.Errorf("获取B站二维码失败: %w", err)
		}
		if qrResp.Code != 0 {
			return "", "", fmt.Errorf("B站返回错误: code=%d", qrResp.Code)
		}
		return qrResp.Data.URL, qrResp.Data.AuthCode, nil

	case "youtube":
		// YouTube OAuth2流程需要单独实现
		// 这里返回OAuth URL作为"二维码"
		return "", "", fmt.Errorf("YouTube平台暂未实现")

	case "douyin":
		return "", "", fmt.Errorf("抖音平台暂未实现")

	case "kuaishou":
		return "", "", fmt.Errorf("快手平台暂未实现")

	default:
		return "", "", fmt.Errorf("不支持的平台: %s", platform)
	}
}

// PollBindingStatusRequest 轮询绑定状态请求
type PollBindingStatusRequest struct {
	QRCodeKey string `json:"qr_code_key" binding:"required"`
}

// PollBindingStatusResponse 轮询绑定状态响应
type PollBindingStatusResponse struct {
	Status      string `json:"status"`       // pending, bound, expired
	Platform    string `json:"platform"`     // 平台
	PlatformUID string `json:"platform_uid"` // 平台用户ID
	Username    string `json:"username"`     // 平台用户名
	Avatar      string `json:"avatar"`       // 平台头像
}

// PollBindingStatus godoc
// @Summary 轮询绑定状态
// @Description 轮询二维码扫描和绑定状态，用于前端实时检查绑定进度
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param request body PollBindingStatusRequest true "轮询请求"
// @Success 200 {object} Response{data=PollBindingStatusResponse}
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/accounts/poll [post]
func (h *AccountsHandler) PollBindingStatus(c *gin.Context) {
	var req PollBindingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 从缓存中获取绑定信息
	var bindingData map[string]interface{}
	if err := h.bindingCache.Get(req.QRCodeKey, &bindingData); err != nil {
		// 缓存中不存在或已过期
		Success(c, PollBindingStatusResponse{Status: "expired"})
		return
	}

	platform := fmt.Sprintf("%v", bindingData["platform"])
	authCode := fmt.Sprintf("%v", bindingData["auth_code"])

	// 根据平台轮询扫码状态
	switch platform {
	case "bilibili":
		if authCode != "" {
			loginData, err := h.pollBilibiliQRCode(authCode)
			if err != nil {
				h.logger.Debugf("B站二维码未扫描: %v", err)
				Success(c, PollBindingStatusResponse{Status: "pending", Platform: platform})
				return
			}

			if loginData != nil {
				// 扫码成功，保存绑定信息
				userID := fmt.Sprintf("%v", bindingData["user_id"])
				if err := h.saveBilibiliBinding(userID, loginData); err != nil {
					h.logger.Errorf("保存B站绑定失败: %v", err)
					InternalServerError(c, "保存绑定失败")
					return
				}

				// 清理缓存
				h.bindingCache.Delete(req.QRCodeKey)

				// 从数据库读取刚保存的绑定信息
				var savedBinding model.AccountBinding
				if err := h.db.Where("user_id = ? AND platform = ? AND platform_uid = ?",
					userID, model.PlatformBilibili, fmt.Sprintf("%d", loginData.TokenInfo.Mid)).
					First(&savedBinding).Error; err == nil {
					Success(c, PollBindingStatusResponse{
						Status:      "bound",
						Platform:    platform,
						PlatformUID: savedBinding.PlatformUID,
						Username:    savedBinding.Username,
						Avatar:      savedBinding.Avatar,
					})
				} else {
					// 如果读取失败，使用登录信息中的基本数据
					Success(c, PollBindingStatusResponse{
						Status:      "bound",
						Platform:    platform,
						PlatformUID: fmt.Sprintf("%d", loginData.TokenInfo.Mid),
						Username:    loginData.TokenInfo.Uname,
						Avatar:      loginData.TokenInfo.Face,
					})
				}
				return
			}
		}

	case "youtube":
		// YouTube OAuth流程需要通过回调URL处理
		NotImplemented(c, "YouTube平台暂未实现")
		return

	default:
		NotImplemented(c, fmt.Sprintf("%s平台暂未实现", platform))
		return
	}

	// 未扫码，返回等待状态
	Success(c, PollBindingStatusResponse{
		Status:   "pending",
		Platform: platform,
	})
}

// pollBilibiliQRCode 轮询B站二维码状态
func (h *AccountsHandler) pollBilibiliQRCode(authCode string) (*bilibili.LoginInfo, error) {
	client := bilibili.NewClient()
	loginInfo, err := client.PollQRCode(authCode)
	if err != nil {
		return nil, err
	}
	return loginInfo, nil
}

// saveBilibiliBinding 保存B站绑定
func (h *AccountsHandler) saveBilibiliBinding(userID string, loginData *bilibili.LoginInfo) error {
	if loginData == nil {
		return fmt.Errorf("登录数据无效")
	}

	tokenInfo := loginData.TokenInfo
	if tokenInfo.Mid == 0 {
		return fmt.Errorf("无效的用户Mid")
	}

	// 获取cookies字符串
	cookies := buildCookieString(loginData.CookieInfo)

	// 计算过期时间
	var expiresAt *time.Time
	if tokenInfo.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(tokenInfo.ExpiresIn) * time.Second)
		expiresAt = &expiry
	}

	// 初始化用户信息（使用登录返回的基本信息）
	userName := tokenInfo.Uname
	userAvatar := tokenInfo.Face
	userLevel := 0
	userVip := false

	if userName == "" {
		userName = fmt.Sprintf("用户_%d", tokenInfo.Mid)
	}

	// 尝试获取完整用户信息
	client := bilibili.NewClient()
	myInfo, err := client.GetMyInfoWithRetry(cookies, 2)
	if err == nil && myInfo != nil {
		userName = myInfo.Uname
		userAvatar = myInfo.Face
		userLevel = myInfo.Level
		userVip = false // MyInfoResponse中没有VIP字段

		h.logger.Infof("成功获取B站用户完整信息: username=%s, level=%d", userName, userLevel)
	} else {
		h.logger.Warnf("获取B站用户详细信息失败，使用登录返回的基本信息: %v", err)
	}

	// 构建B站平台数据
	platformData := &model.BiliPlatformData{
		BiliMid:   tokenInfo.Mid,
		BiliLevel: userLevel,
		BiliVip:   userVip,
	}

	// 检查是否已存在绑定
	var existingBinding model.AccountBinding
	result := h.db.Where("user_id = ? AND platform = ? AND platform_uid = ?",
		userID, model.PlatformBilibili, fmt.Sprintf("%d", tokenInfo.Mid)).
		First(&existingBinding)

	now := time.Now()

	if result.Error == gorm.ErrRecordNotFound {
		// 创建新绑定
		binding := &model.AccountBinding{
			UserID:       userID,
			Platform:     model.PlatformBilibili,
			PlatformUID:  fmt.Sprintf("%d", tokenInfo.Mid),
			Username:     userName,
			Avatar:       userAvatar,
			AccessToken:  tokenInfo.AccessToken,
			RefreshToken: tokenInfo.RefreshToken,
			ExpiresAt:    expiresAt,
			Status:       model.BindingStatusBound,
			Cookies:      cookies,
			LastUsedAt:   &now,
		}

		// 设置平台数据
		if err := binding.SetBiliData(platformData); err != nil {
			return fmt.Errorf("设置B站数据失败: %w", err)
		}

		// 检查是否是第一个B站账号，如果是则设为主账号
		var count int64
		h.db.Model(&model.AccountBinding{}).
			Where("user_id = ? AND platform = ? AND status = ?",
				userID, model.PlatformBilibili, model.BindingStatusBound).
			Count(&count)

		// 同步保存到本地存储（兼容旧逻辑，确保后台上传任务可用）
		// 注意：如果有多个账号，这里会覆盖本地存储，只保留最新绑定的那个账号作为默认上传账号
		if err := storage.GetDefaultStore().Save(loginData); err != nil {
			h.logger.Warnf("同步保存登录信息到本地失败: %v", err)
		} else {
			h.logger.Infof("已同步保存B站登录信息到本地存储")
		}
		binding.IsPrimary = (count == 0)

		// 使用加密服务保存
		if err := h.accountService.SaveBinding(binding); err != nil {
			h.logger.Errorf("创建B站绑定失败: %v", err)
			return fmt.Errorf("创建B站绑定失败: %w", err)
		}

		h.logger.Infof("成功创建B站绑定: user_id=%s, bili_mid=%d, username=%s", userID, tokenInfo.Mid, userName)
		return nil
	} else if result.Error != nil {
		return fmt.Errorf("查询绑定记录失败: %w", result.Error)
	}

	// 更新现有绑定
	updates := map[string]interface{}{
		"username":      userName,
		"avatar":        userAvatar,
		"access_token":  tokenInfo.AccessToken,
		"refresh_token": tokenInfo.RefreshToken,
		"expires_at":    expiresAt,
		"status":        model.BindingStatusBound,
		"cookies":       cookies,
		"last_used_at":  &now,
	}

	// 更新平台数据
	existingBinding.SetBiliData(platformData)
	if existingBinding.PlatformData != nil {
		updates["platform_data"] = *existingBinding.PlatformData
	}

	// 使用加密服务更新
	if err := h.accountService.UpdateBinding(&existingBinding, updates); err != nil {
		h.logger.Errorf("更新B站绑定失败: %v", err)
		return fmt.Errorf("更新B站绑定失败: %w", err)
	}

	h.logger.Infof("成功更新B站绑定: user_id=%s, bili_mid=%d, username=%s", userID, tokenInfo.Mid, userName)
	return nil
}

// BindingInfo 绑定信息
type BindingInfo struct {
	ID          uint   `json:"id"`
	Platform    string `json:"platform"`
	PlatformUID string `json:"platform_uid"`
	Username    string `json:"username"`
	Avatar      string `json:"avatar"`
	Status      string `json:"status"`
	IsPrimary   bool   `json:"is_primary"`
	IsActive    bool   `json:"is_active"`
	CreateTime  int64  `json:"create_time"`
	LastUsedAt  *int64 `json:"last_used_at,omitempty"`
}

// GetBindingList godoc
// @Summary 获取账号绑定列表
// @Description 获取指定用户的所有已绑定账号列表
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param user_id query string true "用户ID"
// @Success 200 {object} Response{data=[]BindingInfo}
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/accounts/list [get]
func (h *AccountsHandler) GetBindingList(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		BadRequest(c, "缺少用户ID")
		return
	}

	var bindings []model.AccountBinding
	if err := h.db.Where("user_id = ? AND status = ?", userID, model.BindingStatusBound).
		Order("is_primary DESC, created_at DESC").
		Find(&bindings).Error; err != nil {
		h.logger.Errorf("查询绑定列表失败: %v", err)
		InternalServerError(c, "数据库查询失败")
		return
	}

	bindingInfos := make([]BindingInfo, 0, len(bindings))
	for _, binding := range bindings {
		var lastUsedTimestamp *int64
		if binding.LastUsedAt != nil {
			ts := binding.LastUsedAt.Unix()
			lastUsedTimestamp = &ts
		}

		bindingInfos = append(bindingInfos, BindingInfo{
			ID:          binding.ID,
			Platform:    string(binding.Platform),
			PlatformUID: binding.PlatformUID,
			Username:    binding.Username,
			Avatar:      binding.Avatar,
			Status:      string(binding.Status),
			IsPrimary:   binding.IsPrimary,
			IsActive:    binding.IsActive(),
			CreateTime:  binding.CreatedAt.Unix(),
			LastUsedAt:  lastUsedTimestamp,
		})
	}

	Success(c, bindingInfos)
}

// UnbindAccount godoc
// @Summary 解绑账号
// @Description 解绑指定的平台账号
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param id path int true "绑定ID"
// @Success 200 {object} Response
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/accounts/{id} [delete]
func (h *AccountsHandler) UnbindAccount(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		BadRequest(c, "缺少绑定ID")
		return
	}

	var binding model.AccountBinding
	if err := h.db.First(&binding, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			NotFound(c, "绑定记录不存在")
		} else {
			h.logger.Errorf("查询绑定记录失败: %v", err)
			InternalServerError(c, "数据库查询失败")
		}
		return
	}

	// 更新状态为已解绑（软删除）
	binding.Status = model.BindingStatusUnbound
	if err := h.db.Save(&binding).Error; err != nil {
		h.logger.Errorf("更新绑定状态失败: %v", err)
		InternalServerError(c, "数据库更新失败")
		return
	}

	h.logger.Infof("成功解绑账号: id=%s, platform=%s", id, binding.Platform)
	Success(c, nil)
}

// SetPrimaryAccountRequest 设置主账号请求
type SetPrimaryAccountRequest struct {
	UserID      string `json:"user_id" binding:"required"`
	Platform    string `json:"platform" binding:"required"`
	PlatformUID string `json:"platform_uid" binding:"required"`
}

// SetPrimaryAccount godoc
// @Summary 设置主账号
// @Description 设置指定平台的主账号
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param request body SetPrimaryAccountRequest true "设置主账号请求"
// @Success 200 {object} Response
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/accounts/primary [post]
func (h *AccountsHandler) SetPrimaryAccount(c *gin.Context) {
	var req SetPrimaryAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 使用服务层的事务方法
	if err := h.accountService.SetPrimaryBinding(req.UserID, model.Platform(req.Platform), req.PlatformUID); err != nil {
		h.logger.Errorf("设置主账号失败: %v", err)
		InternalServerError(c, "设置主账号失败")
		return
	}

	h.logger.Infof("成功设置主账号: user_id=%s, platform=%s, platform_uid=%s", req.UserID, req.Platform, req.PlatformUID)
	Success(c, nil)
}

// RefreshTokenRequest 刷新令牌请求
type RefreshTokenRequest struct {
	BindingID uint `json:"binding_id" binding:"required"`
}

// RefreshToken godoc
// @Summary 刷新令牌
// @Description 刷新指定绑定的访问令牌
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "刷新请求"
// @Success 200 {object} Response
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/accounts/refresh [post]
func (h *AccountsHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	var binding model.AccountBinding
	if err := h.db.First(&binding, req.BindingID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			NotFound(c, "绑定记录不存在")
		} else {
			h.logger.Errorf("查询绑定记录失败: %v", err)
			InternalServerError(c, "数据库查询失败")
		}
		return
	}

	// TODO: 根据平台实现令牌刷新逻辑
	switch binding.Platform {
	case model.PlatformBilibili:
		h.logger.Warnf("B站令牌刷新功能待实现")
		NotImplemented(c, "B站令牌刷新功能待实现")
		return
	default:
		BadRequest(c, "不支持的平台")
		return
	}
}
