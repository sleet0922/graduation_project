package handler

import (
	"context"
	"log/slog"
	"net/http"
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
	onlineHeartbeatInterval = 5 * time.Second
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
func (h *OnlineHandler) writeOnlineStatus(ctx context.Context, writer *SocketWriter, incoming onlineIncomingMessage) error {
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
		return writer.WriteJSON(ctx, onlineOutgoingMessage{
			Type:  "error",
			Error: "用户ID不能为空",
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
		return writer.WriteJSON(ctx, onlineOutgoingMessage{
			Type:   "online_status",
			UserID: statuses[0].UserID,
			Online: statuses[0].Online,
		})
	}
	return writer.WriteJSON(ctx, onlineOutgoingMessage{
		Type:     "online_status",
		Statuses: statuses,
	})
}

// ----------OnlineHandler 方法----------
// 建立在线状态WebSocket连接
func (h *OnlineHandler) Connect(c *gin.Context) {
	currentUserID, err := GetUserID(c)
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
		ticker := time.NewTicker(onlineHeartbeatInterval)
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
					logger.Warn("online websocket ping failed", slog.Any("user_id", currentUserID), slog.Any("error", err))
					conn.Close(websocket.StatusGoingAway, "ping failed")
					return
				}
			}
		}
	}()

	if err := writer.WriteJSON(ctx, onlineOutgoingMessage{
		Type:   "connected",
		UserID: currentUserID,
	}); err != nil {
		return
	}

	logger.Info("online websocket connected", slog.Any("user_id", currentUserID))
	defer logger.Info("online websocket disconnected", slog.Any("user_id", currentUserID))

	for {
		var incoming onlineIncomingMessage
		if err := wsjson.Read(ctx, conn, &incoming); err != nil {
			return
		}

		switch incoming.Type {
		case "ping":
			if err := writer.WriteJSON(ctx, onlineOutgoingMessage{Type: "pong"}); err != nil {
				return
			}
		case "check_online":
			if err := h.writeOnlineStatus(ctx, writer, incoming); err != nil {
				return
			}
		default:
			logger.Warn("unsupported online message type", slog.Any("user_id", currentUserID), slog.String("type", incoming.Type))
			if err := writer.WriteJSON(ctx, onlineOutgoingMessage{
				Type:  "error",
				Error: "不支持的消息类型",
			}); err != nil {
				return
			}
		}
	}
}
