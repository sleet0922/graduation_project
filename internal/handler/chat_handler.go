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
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
)

const (
	chatHeartbeatInterval = 5 * time.Second
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
	}
}

// ----------ChatHandler 方法----------
func (h *ChatHandler) Connect(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := &SocketWriter{Conn: conn}
	go func() {
		ticker := time.NewTicker(chatHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, wsPingTimeout)
				err := writer.Ping(pingCtx)
				pingCancel()
				if err != nil {
					logger.Warn("websocket ping failed", slog.Any("user_id", userID), slog.Any("error", err))
					conn.Close(websocket.StatusGoingAway, "ping failed")
					return
				}
			}
		}
	}()

	if err := writer.WriteJSON(ctx, chatOutgoingMessage{
		Type:   "connected",
		UserID: userID,
	}); err != nil {
		return
	}

	connectionID := h.chatService.RegisterConnection(userID, func(message *model.ChatMessage, offline bool) error {
		payload := chatOutgoingMessage{
			Type:    "chat",
			Message: message,
			Offline: offline,
		}
		if offline {
			return writer.WriteJSON(ctx, payload)
		}
		return writer.WriteVerified(ctx, payload)
	}, func(payload any) error {
		return writer.WriteVerified(ctx, payload)
	}, func() {
		cancel()
	})
	logger.Info("websocket connected", slog.Any("user_id", userID), slog.String("connection_id", connectionID))

	defer func() {
		h.chatService.UnregisterConnection(userID, connectionID)
		logger.Info("websocket disconnected", slog.Any("user_id", userID), slog.String("connection_id", connectionID))
	}()

	for {
		var incoming chatIncomingMessage
		err := wsjson.Read(ctx, conn, &incoming)
		if err != nil {
			return
		}

		if incoming.Type != "chat" {
			// ping/pong 心跳静默处理，不需打日志或报错
			if incoming.Type == "ping" {
				if err := writer.WriteJSON(ctx, chatOutgoingMessage{Type: "pong"}); err != nil {
					return
				}
				continue
			}
			logger.Warn("unsupported message type", slog.Any("user_id", userID), slog.String("type", incoming.Type))
			err = writer.WriteJSON(ctx, chatOutgoingMessage{
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
			err = writer.WriteJSON(ctx, chatOutgoingMessage{
				Type:  "error",
				Error: "接收方或群聊不能为空",
			})
			if err != nil {
				return
			}
			continue
		}

		message, err := h.chatService.SendMessage(userID, incoming.ToUserID, incoming.GroupID, incoming.MessageType, incoming.Content)
		if err != nil {
			logger.Warn("send message failed", slog.Any("user_id", userID), slog.Any("to_user_id", incoming.ToUserID), slog.Any("group_id", incoming.GroupID), slog.Any("error", err))
			if writeErr := writer.WriteJSON(ctx, chatOutgoingMessage{
				Type:  "error",
				Error: err.Error(),
			}); writeErr != nil {
				return
			}
			continue
		}
		if err := writer.WriteJSON(ctx, chatOutgoingMessage{
			Type:    "sent",
			Message: message,
		}); err != nil {
			return
		}
	}
}
