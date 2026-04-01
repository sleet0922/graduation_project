package handler

import (
	"context"
	"net/http"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/jwt"
	"sleet0922/graduation_project/pkg/logger"
	"sleet0922/graduation_project/pkg/response"
	"strconv"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	chatHeartbeatInterval = 5 * time.Second
	chatPingTimeout       = 3 * time.Second
	chatWriteTimeout      = 5 * time.Second
)

type ChatHandler struct {
	chatService service.ChatService
	jwtManager  *jwt.JWTManager
}

type chatIncomingMessage struct {
	Type        string `json:"type"`
	ToUserID    uint   `json:"to_user_id"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
}

type chatOutgoingMessage struct {
	Type    string             `json:"type"`
	UserID  uint               `json:"user_id,omitempty"`
	Message *model.ChatMessage `json:"message,omitempty"`
	Offline bool               `json:"offline,omitempty"`
	Error   string             `json:"error,omitempty"`
}

type chatSocketWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func NewChatHandler(chatService service.ChatService, jwtManager *jwt.JWTManager) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		jwtManager:  jwtManager,
	}
}

func (h *ChatHandler) Connect(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "未找到用户信息")
		return
	}
	userID := userIDVal.(uint)

	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := &chatSocketWriter{conn: conn}
	go func() {
		ticker := time.NewTicker(chatHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, chatPingTimeout)
				err := writer.Ping(pingCtx)
				pingCancel()
				if err != nil {
					logger.Warn("websocket ping failed", zap.Uint("user_id", userID), zap.Error(err))
					conn.Close(websocket.StatusGoingAway, "ping failed")
					return
				}
			}
		}
	}()

	if err := writer.Write(ctx, chatOutgoingMessage{
		Type:   "connected",
		UserID: userID,
	}); err != nil {
		return
	}

	connectionID := h.chatService.RegisterConnection(userID, func(message *model.ChatMessage, offline bool) error {
		return writer.WriteChat(ctx, chatOutgoingMessage{
			Type:    "chat",
			Message: message,
			Offline: offline,
		}, !offline)
	})
	logger.Info("websocket connected", zap.Uint("user_id", userID), zap.String("connection_id", connectionID))

	defer func() {
		h.chatService.UnregisterConnection(userID, connectionID)
		logger.Info("websocket disconnected", zap.Uint("user_id", userID), zap.String("connection_id", connectionID))
	}()

	for {
		var incoming chatIncomingMessage
		if err := wsjson.Read(ctx, conn, &incoming); err != nil {
		}

		if incoming.Type != "chat" {
			if err := writer.Write(ctx, chatOutgoingMessage{
				Type:  "error",
				Error: "不支持的消息类型",
			}); err != nil {
				return
			}
			continue
		}

		if incoming.ToUserID == 0 {
			if err := writer.Write(ctx, chatOutgoingMessage{
				Type:  "error",
				Error: "接收方不能为空",
			}); err != nil {
				return
			}
			continue
		}

		message, err := h.chatService.SendMessage(userID, incoming.ToUserID, incoming.MessageType, incoming.Content)
		if err != nil {
			if err := writer.Write(ctx, chatOutgoingMessage{
				Type:  "error",
				Error: err.Error(),
			}); err != nil {
				return
			}
			continue
		}
		if err := writer.Write(ctx, chatOutgoingMessage{
			Type:    "sent",
			Message: message,
		}); err != nil {
			return
		}
	}
}

func (w *chatSocketWriter) Write(ctx context.Context, payload chatOutgoingMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	writeCtx, cancel := context.WithTimeout(ctx, chatWriteTimeout)
	defer cancel()
	return wsjson.Write(writeCtx, w.conn, payload)
}

func (w *chatSocketWriter) WriteChat(ctx context.Context, payload chatOutgoingMessage, verifyAlive bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if verifyAlive {
		pingCtx, cancel := context.WithTimeout(ctx, chatPingTimeout)
		err := w.conn.Ping(pingCtx)
		cancel()
		if err != nil {
			return err
		}
	}

	writeCtx, cancel := context.WithTimeout(ctx, chatWriteTimeout)
	defer cancel()
	return wsjson.Write(writeCtx, w.conn, payload)
}

func (w *chatSocketWriter) Ping(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.Ping(ctx)
}

func (h *ChatHandler) GetHistory(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "未找到用户信息")
		return
	}
	userID := userIDVal.(uint)

	friendIDStr := c.Query("friend_id")
	if friendIDStr != "" {
		friendID, err := strconv.ParseUint(friendIDStr, 10, 32)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "无效的friend_id")
			return
		}
		messages, err := h.chatService.GetHistory(userID, uint(friendID))
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "获取记录失败")
			return
		}
		response.Success(c, messages, "获取成功")
		return
	}

	messages, err := h.chatService.GetAllHistory(userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "获取记录失败")
		return
	}
	response.Success(c, messages, "获取成功")
}

func (h *ChatHandler) DeleteHistory(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "未找到用户信息")
		return
	}
	userID := userIDVal.(uint)
	friendIDStr := c.Query("friend_id")
	if friendIDStr != "" {
		friendID, err := strconv.ParseUint(friendIDStr, 10, 32)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "无效的friend_id")
			return
		}
		err = h.chatService.DeleteHistory(userID, uint(friendID))
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "删除失败")
			return
		}
	} else {
		err := h.chatService.DeleteAllHistory(userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "删除失败")
			return
		}
	}
	response.Success(c, nil, "删除成功")
}
