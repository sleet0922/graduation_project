package middleware

import (
	"log/slog"
	"runtime/debug"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/logger"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

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

// Logger 请求日志中间件
func Logger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		path := c.Path()
		query := maskTokenInQuery(string(c.Request().URI().QueryString()))

		err := c.Next()

		cost := time.Since(start)
		logger.Info(path,
			slog.Int("status", c.Response().StatusCode()),
			slog.String("method", c.Method()),
			slog.String("ip", c.IP()),
			slog.Duration("cost", cost),
			slog.String("query", query),
		)
		return err
	}
}

// Recovery panic 恢复中间件
func Recovery() fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("[Recovery from panic]",
					slog.Any("error", r),
					slog.String("path", c.Path()),
					slog.String("stack", string(debug.Stack())),
				)
				_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"code":    errcode.InternalServerError,
					"message": errcode.GetMsg(errcode.InternalServerError),
					"data":    nil,
				})
			}
		}()
		return c.Next()
	}
}
