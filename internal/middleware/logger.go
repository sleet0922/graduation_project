package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"runtime/debug"
	"sleet0922/graduation_project/pkg/errcode"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// generateRequestID 生成16字节的随机请求ID
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// maskTokenInQuery 脱敏 query 中的 token 参数，避免泄露到日志
func maskTokenInQuery(raw string) string {
	if raw == "" {
		return raw
	}
	i := strings.Index(raw, "token=")
	if i == -1 {
		return raw
	}
	end := strings.Index(raw[i:], "&")
	if end == -1 {
		return raw[:i+6] + "***"
	}
	return raw[:i+6] + "***" + raw[i+end:]
}

// Logger 请求日志中间件，添加请求ID追踪
func Logger(loggers ...*slog.Logger) fiber.Handler {
	log := slog.Default()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return func(c *fiber.Ctx) error {
		start := time.Now()
		path := c.Path()
		query := maskTokenInQuery(string(c.Request().URI().QueryString()))

		// 生成或使用已有的请求ID
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set("X-Request-ID", requestID)
		c.Locals("request_id", requestID)

		err := c.Next()

		cost := time.Since(start)
		status := c.Response().StatusCode()

		logArgs := []any{
			slog.String("request_id", requestID),
			slog.Int("status", status),
			slog.String("method", c.Method()),
			slog.String("ip", c.IP()),
			slog.Duration("cost", cost),
			slog.String("query", query),
		}

		if status >= 500 {
			log.Error(path, logArgs...)
		} else if status >= 400 {
			log.Warn(path, logArgs...)
		} else {
			log.Info(path, logArgs...)
		}

		return err
	}
}

// Recovery panic 恢复中间件
func Recovery(loggers ...*slog.Logger) fiber.Handler {
	log := slog.Default()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				requestID := c.Locals("request_id")
				log.Error("[Recovery from panic]",
					slog.Any("request_id", requestID),
					slog.Any("error", r),
					slog.String("path", c.Path()),
					slog.String("stack", string(debug.Stack())),
				)
				if err := c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"code":    errcode.InternalServerError,
					"message": errcode.GetMsg(errcode.InternalServerError),
					"data":    nil,
				}); err != nil {
					log.Error("failed to write panic response", slog.Any("error", err))
				}
			}
		}()
		return c.Next()
	}
}
