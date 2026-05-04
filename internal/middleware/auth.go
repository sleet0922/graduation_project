package middleware

import (
	"net/http"
	"sleet0922/graduation_project/pkg/jwt"
	redisPkg "sleet0922/graduation_project/pkg/redis"
	"sleet0922/graduation_project/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
)

type JWTMiddleware struct {
	jwtManager *jwt.JWTManager
}

func NewJWTMiddleware(jwtManager *jwt.JWTManager) *JWTMiddleware {
	return &JWTMiddleware{
		jwtManager: jwtManager,
	}
}

// extractAndValidateToken 提取并验证 token，返回 claims，失败时已写入响应
func (m *JWTMiddleware) extractAndValidateToken(c *gin.Context) *jwt.Claims {
	var tokenString string
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			tokenString = parts[1]
		}
	}
	if tokenString == "" {
		tokenString = c.Query("token")
	}
	if tokenString == "" {
		response.Error(c, http.StatusUnauthorized, "缺少认证信息")
		c.Abort()
		return nil
	}

	claims, err := m.jwtManager.ParseToken(tokenString)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "无效的token")
		c.Abort()
		return nil
	}
	if claims.TokenType == jwt.TokenTypeRefresh {
		response.Error(c, http.StatusUnauthorized, "token类型错误")
		c.Abort()
		return nil
	}
	return claims
}

func (m *JWTMiddleware) Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := m.extractAndValidateToken(c)
		if claims == nil {
			return
		}
		c.Set("user_id", uint(claims.UserID))
		c.Set("account", claims.Account)
		c.Set("session_id", claims.SessionID)
		c.Next()
	}
}

// WSAuth WebSocket 专用认证：额外校验 session_id 是否仍然有效（未被踢下线）
func (m *JWTMiddleware) WSAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := m.extractAndValidateToken(c)
		if claims == nil {
			return
		}
		// 校验 session 是否仍然有效（未被新登录踢下线）
		if !redisPkg.IsSessionValid(uint(claims.UserID), claims.SessionID) {
			response.Error(c, http.StatusUnauthorized, "账号在其他设备登录，请重新登录")
			c.Abort()
			return
		}
		c.Set("user_id", uint(claims.UserID))
		c.Set("account", claims.Account)
		c.Set("session_id", claims.SessionID)
		c.Next()
	}
}

func (m *JWTMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}

		if tokenString == "" {
			tokenString = c.Query("token")
		}

		if tokenString == "" {
			c.Next()
			return
		}

		claims, err := m.jwtManager.ParseToken(tokenString)
		if err != nil {
			c.Next()
			return
		}
		if claims.TokenType == jwt.TokenTypeRefresh {
			c.Next()
			return
		}
		c.Set("user_id", uint(claims.UserID))
		c.Set("account", claims.Account)
		c.Set("session_id", claims.SessionID)
		c.Next()
	}
}
