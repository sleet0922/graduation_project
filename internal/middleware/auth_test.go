package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"

	"sleet0922/graduation_project/pkg/jwt"
	redisPkg "sleet0922/graduation_project/pkg/redis"
)

func setupMiniRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	oldClient := redisPkg.RedisClient
	redisPkg.RedisClient = goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		redisPkg.RedisClient.Close()
		redisPkg.RedisClient = oldClient
		server.Close()
	})
	return server
}

func authTestResponse(t *testing.T, app *fiber.App, path string, token string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer resp.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	return resp.StatusCode, payload
}

func TestJWTMiddlewareAuth(t *testing.T) {
	setupMiniRedis(t)
	manager := jwt.NewJWTManager("secret")
	middleware := NewJWTMiddleware(manager)
	app := fiber.New()
	app.Get("/protected", middleware.Auth(), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"user_id":    c.Locals("user_id"),
			"account":    c.Locals("account"),
			"session_id": c.Locals("session_id"),
		})
	})

	status, payload := authTestResponse(t, app, "/protected", "")
	if status != fiber.StatusUnauthorized || payload["message"] != "缺少认证信息" {
		t.Fatalf("missing token response = status %d payload %#v", status, payload)
	}

	refreshToken, err := manager.GenerateTokenWithSession(1, "acct", jwt.TokenTypeRefresh, "s1", time.Hour)
	if err != nil {
		t.Fatalf("Generate refresh token failed: %v", err)
	}
	status, payload = authTestResponse(t, app, "/protected", refreshToken)
	if status != fiber.StatusUnauthorized || payload["message"] != "token类型错误" {
		t.Fatalf("refresh token response = status %d payload %#v", status, payload)
	}

	accessToken, err := manager.GenerateTokenWithSession(1, "acct", jwt.TokenTypeAccess, "s1", time.Hour)
	if err != nil {
		t.Fatalf("Generate access token failed: %v", err)
	}
	status, payload = authTestResponse(t, app, "/protected", accessToken)
	if status != fiber.StatusUnauthorized || payload["message"] != "账号在其他设备登录，请重新登录" {
		t.Fatalf("invalid session response = status %d payload %#v", status, payload)
	}

	if _, err := redisPkg.SetUserSession(1, "s1", time.Hour); err != nil {
		t.Fatalf("SetUserSession failed: %v", err)
	}
	status, payload = authTestResponse(t, app, "/protected", accessToken)
	if status != fiber.StatusOK || payload["account"] != "acct" || payload["session_id"] != "s1" || payload["user_id"].(float64) != 1 {
		t.Fatalf("valid auth response = status %d payload %#v", status, payload)
	}
}

func TestJWTMiddlewareAcceptsQueryToken(t *testing.T) {
	setupMiniRedis(t)
	manager := jwt.NewJWTManager("secret")
	token, err := manager.GenerateTokenWithSession(2, "query", jwt.TokenTypeAccess, "session-query", time.Hour)
	if err != nil {
		t.Fatalf("GenerateTokenWithSession failed: %v", err)
	}
	if _, err := redisPkg.SetUserSession(2, "session-query", time.Hour); err != nil {
		t.Fatalf("SetUserSession failed: %v", err)
	}

	app := fiber.New()
	app.Get("/protected", NewJWTMiddleware(manager).Auth(), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"user_id": c.Locals("user_id")})
	})

	status, payload := authTestResponse(t, app, "/protected?token="+token, "")
	if status != fiber.StatusOK || payload["user_id"].(float64) != 2 {
		t.Fatalf("query token response = status %d payload %#v", status, payload)
	}
}
