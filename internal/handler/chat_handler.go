package handler

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type ChatHandler struct {
	chatService               service.ChatService
	rtcService                service.RTCService
	foregroundDisconnectGrace time.Duration
	wsReadTimeout             time.Duration
	disconnectMu              sync.Mutex
	disconnectTimers          map[uint]*time.Timer
}

const defaultForegroundDisconnectGrace = 3 * time.Second
const defaultWSReadTimeout = 90 * time.Second // WebSocket读超时，客户端应每60秒发送一次ping

type chatIncomingMessage struct {
	Type            string `json:"type"`
	ToUserID        uint   `json:"to_user_id"`
	GroupID         uint   `json:"group_id"`
	MessageType     string `json:"message_type"`
	Content         string `json:"content"`
	MessageID       string `json:"message_id"` // recall 使用
	ClientMessageID string `json:"client_message_id"`
}

type chatOutgoingMessage struct {
	Type            string             `json:"type"`
	UserID          uint               `json:"user_id,omitempty"`
	GroupID         uint               `json:"group_id,omitempty"`
	Message         *model.ChatMessage `json:"message,omitempty"`
	Offline         bool               `json:"offline,omitempty"`
	Error           string             `json:"error,omitempty"`
	ClientMessageID string             `json:"client_message_id,omitempty"`
}

func NewChatHandler(chatService service.ChatService, rtcService service.RTCService) *ChatHandler {
	return &ChatHandler{
		chatService:               chatService,
		rtcService:                rtcService,
		foregroundDisconnectGrace: defaultForegroundDisconnectGrace,
		wsReadTimeout:             defaultWSReadTimeout,
		disconnectTimers:          make(map[uint]*time.Timer),
	}
}

func (h *ChatHandler) cancelForegroundDisconnect(userID uint) {
	h.disconnectMu.Lock()
	defer h.disconnectMu.Unlock()
	if timer := h.disconnectTimers[userID]; timer != nil {
		timer.Stop()
		delete(h.disconnectTimers, userID)
	}
}

func (h *ChatHandler) scheduleForegroundDisconnect(userID uint) {
	if h.rtcService == nil {
		return
	}

	h.disconnectMu.Lock()
	if previous := h.disconnectTimers[userID]; previous != nil {
		previous.Stop()
	}

	// 创建timer，在回调中检查连接状态
	timer := time.AfterFunc(h.foregroundDisconnectGrace, func() {
		// 检查是否还有前台连接
		if h.chatService.HasConnectionClient(userID, "foreground") {
			return
		}
		if err := h.rtcService.HandleParticipantDisconnected(context.Background(), userID); err != nil {
			logger.Warn("failed to terminate rtc call after websocket disconnect", slog.Any("user_id", userID), slog.Any("error", err))
		}

		// 回调完成后清理timer引用
		h.disconnectMu.Lock()
		delete(h.disconnectTimers, userID)
		h.disconnectMu.Unlock()
	})

	h.disconnectTimers[userID] = timer
	h.disconnectMu.Unlock()
}

// 建立聊天 WebSocket 连接
func (h *ChatHandler) Connect() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// 添加panic保护，确保连接清理
		defer func() {
			if r := recover(); r != nil {
				logger.Error("websocket handler panic", slog.Any("panic", r))
			}
		}()

		userID, ok := c.Locals("user_id").(uint)
		if !ok || userID == 0 {
			if err := c.WriteJSON(chatOutgoingMessage{Type: "error", Error: "未授权"}); err != nil {
				logger.Warn("failed to write websocket authentication error", slog.Any("error", err))
			}
			_ = c.Close()
			return
		}
		if h.chatService == nil {
			if err := c.WriteJSON(chatOutgoingMessage{Type: "error", Error: "聊天服务不可用"}); err != nil {
				logger.Warn("failed to write websocket service error", slog.Any("error", err))
			}
			_ = c.Close()
			return
		}
		client := strings.ToLower(strings.TrimSpace(c.Query("client", "foreground")))
		if client != "background" {
			client = "foreground"
		}
		drainOffline := client != "background"

		ctx := context.Background()
		var writeMu sync.Mutex
		writeJSON := func(payload any) error {
			writeMu.Lock()
			defer writeMu.Unlock()
			return c.WriteJSON(payload)
		}
		if err := writeJSON(chatOutgoingMessage{
			Type:   "connected",
			UserID: userID,
		}); err != nil {
			_ = c.Close()
			return
		}

		connectionID := h.chatService.RegisterConnection(ctx, userID, func(message *model.ChatMessage, offline bool) error {
			payload := chatOutgoingMessage{
				Type:    "chat",
				Message: message,
				Offline: offline,
			}
			return writeJSON(payload)
		}, func(payload any) error {
			return writeJSON(payload)
		}, func() {
			_ = c.Close()
		}, service.WithOfflineDrain(drainOffline), service.WithConnectionClient(client))
		if client == "foreground" {
			h.cancelForegroundDisconnect(userID)
		}
		logger.Info("websocket connected", slog.Any("user_id", userID), slog.String("connection_id", connectionID), slog.String("client", client), slog.Bool("drain_offline", drainOffline))

		defer func() {
			h.chatService.UnregisterConnection(userID, connectionID)
			logger.Info("websocket disconnected", slog.Any("user_id", userID), slog.String("connection_id", connectionID))
			if client == "foreground" && !h.chatService.HasConnectionClient(userID, "foreground") {
				h.scheduleForegroundDisconnect(userID)
			}
		}()

		// 设置读超时，防止僵尸连接
		lastActivity := time.Now()

		for {
			// 设置读超时
			if err := c.SetReadDeadline(time.Now().Add(h.wsReadTimeout)); err != nil {
				logger.Warn("failed to set websocket read deadline", slog.Any("user_id", userID), slog.Any("error", err))
				return
			}

			var incoming chatIncomingMessage
			if err := c.ReadJSON(&incoming); err != nil {
				// 检查是否是超时错误
				if time.Since(lastActivity) > h.wsReadTimeout {
					logger.Info("websocket connection timeout", slog.Any("user_id", userID), slog.String("connection_id", connectionID))
				}
				return
			}

			lastActivity = time.Now()

			if incoming.Type != "chat" {
				switch incoming.Type {
				case "ping":
					if err := writeJSON(chatOutgoingMessage{Type: "pong"}); err != nil {
						return
					}
				case "mark_read":
					// 通知对端已读：mark_read { to_user_id | group_id }
					if err := h.chatService.MarkRead(ctx, userID, incoming.ToUserID, incoming.GroupID); err != nil {
						logger.Warn("mark chat message as read failed", slog.Any("user_id", userID), slog.Any("error", err))
					}
				case "recall":
					// 撤回消息：recall { to_user_id | group_id, message_id }
					if err := h.chatService.RecallMessage(ctx, userID, incoming.ToUserID, incoming.GroupID, incoming.MessageID); err != nil {
						logger.Warn("recall message failed", slog.Any("user_id", userID), slog.Any("error", err))
						if writeErr := writeJSON(chatOutgoingMessage{
							Type:  "error",
							Error: err.Error(),
						}); writeErr != nil {
							return
						}
					}
				default:
					logger.Warn("unsupported message type", slog.Any("user_id", userID), slog.String("type", incoming.Type))
					if err := writeJSON(chatOutgoingMessage{
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
				if err := writeJSON(chatOutgoingMessage{
					Type:            "error",
					Error:           "接收方或群聊不能为空",
					ClientMessageID: incoming.ClientMessageID,
				}); err != nil {
					return
				}
				continue
			}

			message, err := h.chatService.SendMessage(ctx, userID, incoming.ToUserID, incoming.GroupID, incoming.MessageType, incoming.Content)
			if err != nil {
				logger.Warn("send message failed", slog.Any("user_id", userID), slog.Any("to_user_id", incoming.ToUserID), slog.Any("group_id", incoming.GroupID), slog.Any("error", err))
				if writeErr := writeJSON(chatOutgoingMessage{
					Type:            "error",
					Error:           err.Error(),
					ClientMessageID: incoming.ClientMessageID,
				}); writeErr != nil {
					return
				}
				continue
			}
			if err := writeJSON(chatOutgoingMessage{
				Type:            "sent",
				Message:         message,
				ClientMessageID: incoming.ClientMessageID,
			}); err != nil {
				return
			}
		}
	})
}
