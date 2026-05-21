package handler

import (
	"context"
	"log/slog"
	"net/http"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/logger"
	"sleet0922/graduation_project/pkg/response"
	"sleet0922/graduation_project/pkg/snapws"

	"github.com/gin-gonic/gin"
)

type OnlineHandler struct {
	chatService service.ChatService
	upgrader    *snapws.Upgrader
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
	return &OnlineHandler{
		chatService: chatService,
		upgrader:    GetSnapWSUpgrader(),
	}
}

// ----------OnlineHandler 私有方法----------
func (h *OnlineHandler) writeOnlineStatus(ctx context.Context, conn *snapws.Conn, incoming onlineIncomingMessage) error {
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
		return conn.SendJSON(ctx, onlineOutgoingMessage{
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
		return conn.SendJSON(ctx, onlineOutgoingMessage{
			Type:   "online_status",
			UserID: statuses[0].UserID,
			Online: statuses[0].Online,
		})
	}
	return conn.SendJSON(ctx, onlineOutgoingMessage{
		Type:     "online_status",
		Statuses: statuses,
	})
}

// ----------OnlineHandler 方法----------
// 建立在线状态 WebSocket 连接
func (h *OnlineHandler) Connect(c *gin.Context) {
	currentUserID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request)
	if err != nil {
		return
	}
	ctx := context.Background()
	if err := conn.SendJSON(ctx, onlineOutgoingMessage{
		Type:   "connected",
		UserID: currentUserID,
	}); err != nil {
		conn.Close()
		return
	}
	logger.Info("online websocket connected", slog.Any("user_id", currentUserID))
	defer logger.Info("online websocket disconnected", slog.Any("user_id", currentUserID))

	for {
		var incoming onlineIncomingMessage
		if err := conn.ReadJSON(&incoming); err != nil {
			return
		}

		switch incoming.Type {
		case "ping":
			if err := conn.SendJSON(ctx, onlineOutgoingMessage{Type: "pong"}); err != nil {
				return
			}
		case "check_online":
			if err := h.writeOnlineStatus(ctx, conn, incoming); err != nil {
				return
			}
		default:
			logger.Warn("unsupported online message type", slog.Any("user_id", currentUserID), slog.String("type", incoming.Type))
			if err := conn.SendJSON(ctx, onlineOutgoingMessage{
				Type:  "error",
				Error: "不支持的消息类型",
			}); err != nil {
				return
			}
		}
	}
}
