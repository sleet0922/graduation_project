package handler

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"sleet0922/graduation_project/pkg/logger"
	"sleet0922/graduation_project/pkg/snapws"
)

// ErrUserIDNotFound 在 gin.Context 中未找到认证中间件注入的 user_id 时返回
var ErrUserIDNotFound = errors.New("在 context 中未发现 user_id")

// GetUserID 从 gin.Context 中提取认证中间件注入的 user_id
func GetUserID(c *gin.Context) (uint, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		logger.Warn("在 context 中未发现 user_id",
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"ip", c.ClientIP(),
		)
		return 0, fmt.Errorf("%w", ErrUserIDNotFound)
	}
	return userID.(uint), nil
}

// 自动处理心跳
func GetSnapWSUpgrader() *snapws.Upgrader {
	return snapws.NewUpgrader(&snapws.Options{
		WriteWait:              5 * time.Second,
		ReadWait:               60 * time.Second,
		PingEvery:              50 * time.Second,
		MaxMessageSize:         1 << 20,
		ReadBufferSize:         4096,
		WriteBufferSize:        4096,
		BroadcastChannelsSize:  8,
		BroadcastBackpressure:  snapws.BackpressureDrop,
		SkipUTF8Validation:     false,
	})
}
