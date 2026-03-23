package middleware

import (
	"net/http"
	"sleet0922/graduation_project/pkg/jwt"
	"sleet0922/graduation_project/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
)

// JWTMiddleware JWT认证中间件
type JWTMiddleware struct {
	jwtManager *jwt.JWTManager
}

// NewJWTMiddleware 创建JWT中间件实例
func NewJWTMiddleware(jwtManager *jwt.JWTManager) *JWTMiddleware {
	return &JWTMiddleware{
		jwtManager: jwtManager,
	}
}

func (m *JWTMiddleware) Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Authorization请求头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, "缺少认证信息")
			c.Abort()
			return
		}
		// 解析Authorization头，格式应为 "Bearer {token}"
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			response.Error(c, http.StatusUnauthorized, "认证格式错误")
			c.Abort()
			return
		}
		claims, err := m.jwtManager.ParseToken(parts[1])
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "无效的token")
			c.Abort()
			return
		}
		c.Set("user_id", uint(claims.UserID))
		c.Set("account", claims.Account)
		c.Next()
	}
}

func (m *JWTMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// 未携带token，继续执行
			c.Next()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			// 格式错误，继续执行
			c.Next()
			return
		}
		claims, err := m.jwtManager.ParseToken(parts[1])
		if err != nil {
			c.Next()
			return
		}
		c.Set("user_id", uint(claims.UserID))
		c.Set("account", claims.Account)
		c.Next()
	}
}
