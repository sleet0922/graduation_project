package middleware

import (
	"sleet0922/graduation_project/pkg/jwt"
	redisPkg "sleet0922/graduation_project/pkg/redis"
	"sleet0922/graduation_project/pkg/response"
	"strings"

	"github.com/gofiber/fiber/v2"
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
func (m *JWTMiddleware) extractAndValidateToken(c *fiber.Ctx) *jwt.Claims {
	var tokenString string
	authHeader := c.Get("Authorization")
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
		_ = response.Error(c, fiber.StatusUnauthorized, "缺少认证信息")
		return nil
	}

	claims, err := m.jwtManager.ParseToken(tokenString)
	if err != nil {
		_ = response.Error(c, fiber.StatusUnauthorized, "无效的token")
		return nil
	}
	if claims.TokenType == jwt.TokenTypeRefresh {
		_ = response.Error(c, fiber.StatusUnauthorized, "token类型错误")
		return nil
	}
	return claims
}

func (m *JWTMiddleware) Auth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := m.extractAndValidateToken(c)
		if claims == nil {
			return nil
		}
		if !redisPkg.IsSessionValid(uint(claims.UserID), claims.SessionID) {
			_ = response.Error(c, fiber.StatusUnauthorized, "账号在其他设备登录，请重新登录")
			return nil
		}
		c.Locals("user_id", uint(claims.UserID))
		c.Locals("account", claims.Account)
		c.Locals("session_id", claims.SessionID)
		return c.Next()
	}
}

// WSAuth WebSocket 专用认证：额外校验 session_id 是否仍然有效（未被踢下线）
func (m *JWTMiddleware) WSAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := m.extractAndValidateToken(c)
		if claims == nil {
			return nil
		}
		// 校验 session 是否仍然有效（未被新登录踢下线）
		if !redisPkg.IsSessionValid(uint(claims.UserID), claims.SessionID) {
			_ = response.Error(c, fiber.StatusUnauthorized, "账号在其他设备登录，请重新登录")
			return nil
		}
		c.Locals("user_id", uint(claims.UserID))
		c.Locals("account", claims.Account)
		c.Locals("session_id", claims.SessionID)
		return c.Next()
	}
}
