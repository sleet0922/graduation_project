package middleware

import (
	"fmt"
	"strings"

	"sleet0922/graduation_project/pkg/jwt"
	redisPkg "sleet0922/graduation_project/pkg/redis"
	"sleet0922/graduation_project/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type JWTMiddleware struct {
	jwtManager   *jwt.JWTManager
	sessionStore redisPkg.SessionStore
}

// NewJWTMiddleware constructs an authentication middleware from an explicit
// JWT manager and session store. Authentication is stateful, so silently
// falling back to a process-global Redis client would allow separate app
// instances to interfere with one another; callers must wire the store.
func NewJWTMiddleware(jwtManager *jwt.JWTManager, store redisPkg.SessionStore) (*JWTMiddleware, error) {
	if jwtManager == nil {
		return nil, fmt.Errorf("jwt middleware: jwt manager dependency is nil")
	}
	if store == nil {
		return nil, fmt.Errorf("jwt middleware: session store dependency is nil")
	}
	return &JWTMiddleware{jwtManager: jwtManager, sessionStore: store}, nil
}

// extractAndValidateToken 提取并验证 token，返回 claims，失败时已写入响应
func (m *JWTMiddleware) extractAndValidateToken(c *fiber.Ctx) *jwt.Claims {
	if m == nil || m.jwtManager == nil {
		_ = response.Error(c, fiber.StatusInternalServerError, "认证服务不可用")
		return nil
	}
	var tokenString string
	authHeader := strings.TrimSpace(c.Get("Authorization"))
	if authHeader != "" {
		parts := strings.Fields(authHeader)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			tokenString = parts[1]
		}
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
		if m == nil || m.sessionStore == nil {
			_ = response.Error(c, fiber.StatusInternalServerError, "会话服务不可用")
			return nil
		}
		claims := m.extractAndValidateToken(c)
		if claims == nil {
			return nil
		}
		valid, err := m.sessionStore.IsSessionValid(uint(claims.UserID), claims.SessionID)
		if err != nil {
			_ = response.Error(c, fiber.StatusServiceUnavailable, "会话服务不可用")
			return nil
		}
		if !valid {
			_ = response.Error(c, fiber.StatusUnauthorized, "账号在其他设备登录，请重新登录")
			return nil
		}
		c.Locals("user_id", uint(claims.UserID))
		c.Locals("account", claims.Account)
		c.Locals("session_id", claims.SessionID)
		return c.Next()
	}
}

// WSAuth 复用 Auth 中间件逻辑（WebSocket 与 REST 共用相同的会话校验）
func (m *JWTMiddleware) WSAuth() fiber.Handler {
	return m.Auth()
}
