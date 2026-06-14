package handler

import (
	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

// BaseHandler 基础Handler
type BaseHandler struct {
	App *core.AppServer
}

// APIResponse 统一API响应格式
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SuccessResponse 成功响应
func SuccessResponse(data interface{}) APIResponse {
	return APIResponse{
		Code:    200,
		Message: "success",
		Data:    data,
	}
}

// ErrorResponse 错误响应
func ErrorResponse(code int, message string) APIResponse {
	return APIResponse{
		Code:    code,
		Message: message,
	}
}

// SendSuccess 发送成功响应
func (h *BaseHandler) SendSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, SuccessResponse(data))
}

// SendError 发送错误响应
func (h *BaseHandler) SendError(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, ErrorResponse(code, message))
}

// GetInt 获取整型参数
func (h *BaseHandler) GetInt(c *gin.Context, key string, defaultValue int) int {
	value := c.Query(key)
	if value == "" {
		return defaultValue
	}
	
	intValue := 0
	if _, err := fmt.Sscanf(value, "%d", &intValue); err != nil {
		return defaultValue
	}
	return intValue
}

// GetString 获取字符串参数
func (h *BaseHandler) GetString(c *gin.Context, key string, defaultValue string) string {
	value := c.Query(key)
	if value == "" {
		return defaultValue
	}
	return value
}
