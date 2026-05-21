package handler

import (
	"context"
	"log/slog"
	"net/http"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/logger"
	"sleet0922/graduation_project/pkg/response"
	"sleet0922/graduation_project/pkg/snapws"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	chatService service.ChatService
	upgrader    *snapws.Upgrader
}

type chatIncomingMessage struct {
	Type        string `json:"type"`
	ToUserID    uint   `json:"to_user_id"`
	GroupID     uint   `json:"group_id"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
}

type chatOutgoingMessage struct {
	Type    string             `json:"type"`
	UserID  uint               `json:"user_id,omitempty"`
	GroupID uint               `json:"group_id,omitempty"`
	Message *model.ChatMessage `json:"message,omitempty"`
	Offline bool               `json:"offline,omitempty"`
	Error   string             `json:"error,omitempty"`
}

// ----------ChatHandler 构造函数----------
func NewChatHandler(chatService service.ChatService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		upgrader:    GetSnapWSUpgrader(),
	}
}

// ----------ChatHandler 方法----------
// 建立聊天 WebSocket 连接
func (h *ChatHandler) Connect(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request)
	if err != nil {
		return
	}

	ctx := context.Background()
	if err := conn.SendJSON(ctx, chatOutgoingMessage{
		Type:   "connected",
		UserID: userID,
	}); err != nil {
		conn.Close()
		return
	}

	connectionID := h.chatService.RegisterConnection(ctx, userID, func(message *model.ChatMessage, offline bool) error {
		payload := chatOutgoingMessage{
			Type:    "chat",
			Message: message,
			Offline: offline,
		}
		return conn.SendJSON(ctx, payload)
	}, func(payload any) error {
		return conn.SendJSON(ctx, payload)
	}, func() {
		conn.Close()
	})
	logger.Info("websocket connected", slog.Any("user_id", userID), slog.String("connection_id", connectionID))

	defer func() {
		h.chatService.UnregisterConnection(userID, connectionID)
		logger.Info("websocket disconnected", slog.Any("user_id", userID), slog.String("connection_id", connectionID))
	}()

	for {
		var incoming chatIncomingMessage
		if err := conn.ReadJSON(&incoming); err != nil {
			return
		}

		if incoming.Type != "chat" {
			if incoming.Type == "ping" {
				if err := conn.SendJSON(ctx, chatOutgoingMessage{Type: "pong"}); err != nil {
					return
				}
				continue
			}
			logger.Warn("unsupported message type", slog.Any("user_id", userID), slog.String("type", incoming.Type))
			err = conn.SendJSON(ctx, chatOutgoingMessage{
				Type:  "error",
				Error: "不支持的消息类型",
			})
			if err != nil {
				return
			}
			continue
		}

		if incoming.ToUserID == 0 && incoming.GroupID == 0 {
			logger.Warn("empty receiver", slog.Any("user_id", userID))
			err = conn.SendJSON(ctx, chatOutgoingMessage{
				Type:  "error",
				Error: "接收方或群聊不能为空",
			})
			if err != nil {
				return
			}
			continue
		}

		message, err := h.chatService.SendMessage(ctx, userID, incoming.ToUserID, incoming.GroupID, incoming.MessageType, incoming.Content)
		if err != nil {
			logger.Warn("send message failed", slog.Any("user_id", userID), slog.Any("to_user_id", incoming.ToUserID), slog.Any("group_id", incoming.GroupID), slog.Any("error", err))
			if writeErr := conn.SendJSON(ctx, chatOutgoingMessage{
				Type:  "error",
				Error: err.Error(),
			}); writeErr != nil {
				return
			}
			continue
		}
		if err := conn.SendJSON(ctx, chatOutgoingMessage{
			Type:    "sent",
			Message: message,
		}); err != nil {
			return
		}
	}
}
