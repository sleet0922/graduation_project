package handler

import (
	"context"
	"errors"
	"log/slog"

	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type OnlineHandler struct {
	chatService service.ChatService
}

type onlineIncomingMessage struct {
	Type    string `json:"type"`
	UserID  uint   `json:"user_id"`
	UserIDs []uint `json:"user_ids"`
}

type onlineStatus struct {
	UserID uint `json:"user_id"`
	Online bool `json:"online"`
}

type onlineOutgoingMessage struct {
	Type     string         `json:"type"`
	UserID   uint           `json:"user_id,omitempty"`
	Online   bool           `json:"online,omitempty"`
	Statuses []onlineStatus `json:"statuses,omitempty"`
	Error    string         `json:"error,omitempty"`
}

func NewOnlineHandler(chatService service.ChatService) *OnlineHandler {
	return &OnlineHandler{chatService: chatService}
}

// ----------OnlineHandler 私有方法----------
func (h *OnlineHandler) writeOnlineStatus(ctx context.Context, c *websocket.Conn, incoming onlineIncomingMessage) error {
	if h.chatService == nil {
		return errors.New("聊天服务不可用")
	}
	userIDs := make([]uint, 0, len(incoming.UserIDs)+1)
	if incoming.UserID > 0 {
		userIDs = append(userIDs, incoming.UserID)
	}
	for _, userID := range incoming.UserIDs {
		if userID == 0 {
			continue
		}
		userIDs = append(userIDs, userID)
	}
	if len(userIDs) == 0 {
		return c.WriteJSON(onlineOutgoingMessage{
			Type:  "error",
			Error: "用户 ID 不能为空",
		})
	}

	statuses := make([]onlineStatus, 0, len(userIDs))
	for _, userID := range userIDs {
		statuses = append(statuses, onlineStatus{
			UserID: userID,
			Online: len(h.chatService.GetConnectionIDs(userID)) > 0,
		})
	}

	if len(statuses) == 1 {
		return c.WriteJSON(onlineOutgoingMessage{
			Type:   "online_status",
			UserID: statuses[0].UserID,
			Online: statuses[0].Online,
		})
	}
	return c.WriteJSON(onlineOutgoingMessage{
		Type:     "online_status",
		Statuses: statuses,
	})
}

// ----------OnlineHandler 方法----------
// 建立在线状态 WebSocket 连接
func (h *OnlineHandler) Connect() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		currentUserID, ok := c.Locals("user_id").(uint)
		if !ok || currentUserID == 0 {
			if err := c.WriteJSON(onlineOutgoingMessage{Type: "error", Error: "未授权"}); err != nil {
				logger.Warn("failed to write online websocket authentication error", slog.Any("error", err))
			}
			_ = c.Close()
			return
		}
		if h.chatService == nil {
			if err := c.WriteJSON(onlineOutgoingMessage{Type: "error", Error: "聊天服务不可用"}); err != nil {
				logger.Warn("failed to write online websocket service error", slog.Any("error", err))
			}
			_ = c.Close()
			return
		}

		if err := c.WriteJSON(onlineOutgoingMessage{
			Type:   "connected",
			UserID: currentUserID,
		}); err != nil {
			_ = c.Close()
			return
		}
		logger.Info("online websocket connected", slog.Any("user_id", currentUserID))
		defer logger.Info("online websocket disconnected", slog.Any("user_id", currentUserID))

		ctx := context.Background()
		for {
			var incoming onlineIncomingMessage
			if err := c.ReadJSON(&incoming); err != nil {
				return
			}

			switch incoming.Type {
			case "ping":
				if err := c.WriteJSON(onlineOutgoingMessage{Type: "pong"}); err != nil {
					return
				}
			case "check_online":
				if err := h.writeOnlineStatus(ctx, c, incoming); err != nil {
					return
				}
			default:
				logger.Warn("unsupported online message type", slog.Any("user_id", currentUserID), slog.String("type", incoming.Type))
				if err := c.WriteJSON(onlineOutgoingMessage{
					Type:  "error",
					Error: "不支持的消息类型",
				}); err != nil {
					return
				}
			}
		}
	})
}
