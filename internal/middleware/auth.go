package middleware

import (
	"net/http"
	"sleet0922/graduation_project/pkg/jwt"
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

func (m *JWTMiddleware) Auth() gin.HandlerFunc {
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
			response.Error(c, http.StatusUnauthorized, "缺少认证信息")
			c.Abort()
			return
		}

		claims, err := m.jwtManager.ParseToken(tokenString)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "无效的token")
			c.Abort()
			return
		}
		if claims.TokenType == jwt.TokenTypeRefresh {
			response.Error(c, http.StatusUnauthorized, "token类型错误")
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
		c.Next()
	}
}
