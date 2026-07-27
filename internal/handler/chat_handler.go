package handler

import (
	"context"
	"log/slog"
	"strings"

	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type ChatHandler struct {
	chatService service.ChatService
}

type chatIncomingMessage struct {
	Type        string `json:"type"`
	ToUserID    uint   `json:"to_user_id"`
	GroupID     uint   `json:"group_id"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
	MessageID   string `json:"message_id"` // recall 使用
}

type chatOutgoingMessage struct {
	Type    string             `json:"type"`
	UserID  uint               `json:"user_id,omitempty"`
	GroupID uint               `json:"group_id,omitempty"`
	Message *model.ChatMessage `json:"message,omitempty"`
	Offline bool               `json:"offline,omitempty"`
	Error   string             `json:"error,omitempty"`
}

func NewChatHandler(chatService service.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

// 建立聊天 WebSocket 连接
func (h *ChatHandler) Connect() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		userID := c.Locals("user_id").(uint)
		client := strings.ToLower(c.Query("client", "foreground"))
		drainOffline := client != "background"

		ctx := context.Background()
		if err := c.WriteJSON(chatOutgoingMessage{
			Type:   "connected",
			UserID: userID,
		}); err != nil {
			c.Close()
			return
		}

		connectionID := h.chatService.RegisterConnection(ctx, userID, func(message *model.ChatMessage, offline bool) error {
			payload := chatOutgoingMessage{
				Type:    "chat",
				Message: message,
				Offline: offline,
			}
			return c.WriteJSON(payload)
		}, func(payload any) error {
			return c.WriteJSON(payload)
		}, func() {
			c.Close()
		}, service.WithOfflineDrain(drainOffline), service.WithConnectionClient(client))
		logger.Info("websocket connected", slog.Any("user_id", userID), slog.String("connection_id", connectionID), slog.String("client", client), slog.Bool("drain_offline", drainOffline))

		defer func() {
			h.chatService.UnregisterConnection(userID, connectionID)
			logger.Info("websocket disconnected", slog.Any("user_id", userID), slog.String("connection_id", connectionID))
		}()

		for {
			var incoming chatIncomingMessage
			if err := c.ReadJSON(&incoming); err != nil {
				return
			}

			if incoming.Type != "chat" {
				switch incoming.Type {
				case "ping":
					if err := c.WriteJSON(chatOutgoingMessage{Type: "pong"}); err != nil {
						return
					}
				case "mark_read":
					// 通知对端已读：mark_read { to_user_id | group_id }
					_ = h.chatService.MarkRead(ctx, userID, incoming.ToUserID, incoming.GroupID)
				case "recall":
					// 撤回消息：recall { to_user_id | group_id, message_id }
					if err := h.chatService.RecallMessage(ctx, userID, incoming.ToUserID, incoming.GroupID, incoming.MessageID); err != nil {
						logger.Warn("recall message failed", slog.Any("user_id", userID), slog.Any("error", err))
						if writeErr := c.WriteJSON(chatOutgoingMessage{
							Type:  "error",
							Error: err.Error(),
						}); writeErr != nil {
							return
						}
					}
				default:
					logger.Warn("unsupported message type", slog.Any("user_id", userID), slog.String("type", incoming.Type))
					if err := c.WriteJSON(chatOutgoingMessage{
						Type:  "error",
						Error: "不支持的消息类型",
					}); err != nil {
						return
					}
				}
				continue
			}

			if incoming.ToUserID == 0 && incoming.GroupID == 0 {
				logger.Warn("empty receiver", slog.Any("user_id", userID))
				if err := c.WriteJSON(chatOutgoingMessage{
					Type:  "error",
					Error: "接收方或群聊不能为空",
				}); err != nil {
					return
				}
				continue
			}

			message, err := h.chatService.SendMessage(ctx, userID, incoming.ToUserID, incoming.GroupID, incoming.MessageType, incoming.Content)
			if err != nil {
				logger.Warn("send message failed", slog.Any("user_id", userID), slog.Any("to_user_id", incoming.ToUserID), slog.Any("group_id", incoming.GroupID), slog.Any("error", err))
				if writeErr := c.WriteJSON(chatOutgoingMessage{
					Type:  "error",
					Error: err.Error(),
				}); writeErr != nil {
					return
				}
				continue
			}
			if err := c.WriteJSON(chatOutgoingMessage{
				Type:    "sent",
				Message: message,
			}); err != nil {
				return
			}
		}
	})
}
