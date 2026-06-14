package middleware

import (
	"net/http"
	"strings"

	"github.com/ZhantaoLi/ytb2bili/pkg/auth"
	"github.com/gin-gonic/gin"
)

// JWTAuthMiddleware JWT认证中间件
type JWTAuthMiddleware struct {
	JWTManager *auth.JWTManager
}

// NewJWTAuthMiddleware 创建JWT认证中间件
func NewJWTAuthMiddleware(jwtManager *auth.JWTManager) *JWTAuthMiddleware {
	return &JWTAuthMiddleware{
		JWTManager: jwtManager,
	}
}

// RequireAuth 要求管理员已登录
func (m *JWTAuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未登录，请先登录",
			})
			c.Abort()
			return
		}

		// 提取 token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid authorization format",
			})
			c.Abort()
			return
		}

		// 验证 token
		claims, err := m.JWTManager.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)

		c.Next()
	}
}

// GetUserID 从上下文获取用户ID
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		return userID.(string)
	}
	return ""
}

// GetUsername 从上下文获取用户名
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get("username"); exists {
		return username.(string)
	}
	return ""
}
