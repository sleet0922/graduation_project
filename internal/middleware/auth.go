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

// 要求请求必须携带有效的JWT token，否则返回401错误
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

		// 解析JWT token
		claims, err := m.jwtManager.ParseToken(parts[1])
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "无效的token")
			c.Abort()
			return
		}

		// 将用户信息存入上下文，供后续handler使用
		c.Set("user_id", claims.UserID)
		c.Set("account", claims.Account)
		c.Next()
	}
}

// OptionalAuth 可选认证中间件
// 请求可以携带JWT token，也可以不携带
// 如果携带了有效token，会将用户信息存入上下文
// 如果未携带或token无效，继续执行后续handler
func (m *JWTMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Authorization请求头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// 未携带token，继续执行
			c.Next()
			return
		}

		// 解析Authorization头
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			// 格式错误，继续执行
			c.Next()
			return
		}

		// 尝试解析JWT token
		claims, err := m.jwtManager.ParseToken(parts[1])
		if err != nil {
			// token无效，继续执行
			c.Next()
			return
		}

		// token有效，将用户信息存入上下文
		c.Set("user_id", claims.UserID)
		c.Set("account", claims.Account)
		c.Next()
	}
}
