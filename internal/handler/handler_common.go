package handler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"

	"sleet0922/graduation_project/pkg/logger"
)

// ErrUserIDNotFound 在gin.Context中未找到认证中间件注入的user_id时返回
var ErrUserIDNotFound = errors.New("在context中未发现user_id")

// GetUserID 从gin.Context中提取认证中间件注入的 user_id
func GetUserID(c *gin.Context) (uint, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		logger.Warn("在context中未发现user_id",
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"ip", c.ClientIP(),
		)
		return 0, fmt.Errorf("%w", ErrUserIDNotFound)
	}
	return userID.(uint), nil
}

// ---------- 公共 WebSocket Writer ----------

const (
	wsWriteTimeout = 5 * time.Second
	wsPingTimeout  = 3 * time.Second
)

// SocketWriter WebSocket 安全写入器，内嵌互斥锁防止并发写入
type SocketWriter struct {
	Conn *websocket.Conn
	mu   sync.Mutex
}

func (w *SocketWriter) WriteJSON(ctx context.Context, payload any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	writeCtx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
	defer cancel()
	return wsjson.Write(writeCtx, w.Conn, payload)
}

func (w *SocketWriter) Ping(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	pingCtx, cancel := context.WithTimeout(ctx, wsPingTimeout)
	defer cancel()
	return w.Conn.Ping(pingCtx)
}

// 先 ping 验证连接存活，再写入
func (w *SocketWriter) WriteVerified(ctx context.Context, payload any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	pingCtx, cancel := context.WithTimeout(ctx, wsPingTimeout)
	err := w.Conn.Ping(pingCtx)
	cancel()
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
	defer cancel()
	return wsjson.Write(writeCtx, w.Conn, payload)
}
