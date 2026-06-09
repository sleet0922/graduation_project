package handler

import (
	"fmt"

	"sleet0922/graduation_project/pkg/logger"

	"github.com/gofiber/fiber/v2"
)

// ErrUserIDNotFound 在 fiber.Ctx 中未找到认证中间件注入的 user_id 时返回
var ErrUserIDNotFound = fmt.Errorf("在 context 中未发现 user_id")

// GetUserID 从 fiber.Ctx 中提取认证中间件注入的 user_id
func GetUserID(c *fiber.Ctx) (uint, error) {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		logger.Warn("在 context 中未发现 user_id",
			"path", c.Path(),
			"method", c.Method(),
			"ip", c.IP(),
		)
		return 0, ErrUserIDNotFound
	}
	return userID, nil
}
