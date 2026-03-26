package handler

import (
	"context"
	"log"
	"net/http"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/jwt"
	"sleet0922/graduation_project/pkg/response"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	// 消息发送、连接管理
	chatService service.ChatService
	jwtManager  *jwt.JWTManager
}

// 客户端通过 WebSocket 发送给服务端的 JSON 消息结构。
type chatIncomingMessage struct {
	Type        string `json:"type"`
	ToUserID    uint   `json:"to_user_id"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
}

// 服务端推送给客户端的 JSON 消息结构。
type chatOutgoingMessage struct {
	Type    string             `json:"type"`
	UserID  uint               `json:"user_id,omitempty"`
	Message *model.ChatMessage `json:"message,omitempty"`
	Offline bool               `json:"offline,omitempty"`
	Error   string             `json:"error,omitempty"`
}

// 封装了 WebSocket 连接对象，并通过互斥锁保证并发写入的安全性。
type chatSocketWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// 创建并返回一个新的 ChatHandler 实例。
func NewChatHandler(chatService service.ChatService, jwtManager *jwt.JWTManager) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		jwtManager:  jwtManager,
	}
}

// Connect 处理客户端发起的 WebSocket 聊天连接请求。
// 流程包括：Token 验证、升级 HTTP 协议为 WebSocket、注册连接到 chatService，
// 以及在一个无限循环中读取和处理客户端发送的消息。
func (h *ChatHandler) Connect(c *gin.Context) {
	// 1. 提取并验证用户的认证信息 (Token)
	tokenString := h.extractToken(c)
	if tokenString == "" {
		response.Error(c, http.StatusUnauthorized, "缺少认证信息")
		return
	}

	claims, err := h.jwtManager.ParseToken(tokenString)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "无效的token")
		return
	}

	// 2. 将当前的 HTTP 请求升级为 WebSocket 连接
	// InsecureSkipVerify: true 允许跨域等不安全的来源连接（在生产环境中应严格限制）
	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx := context.Background()
	writer := &chatSocketWriter{conn: conn}

	// 3. 将连接注册到 ChatService 中，并提供一个向此 WebSocket 发送消息的回调函数
	connectionID := h.chatService.RegisterConnection(claims.UserID, func(message *model.ChatMessage, offline bool) error {
		return writer.Write(ctx, chatOutgoingMessage{
			Type:    "chat",
			Message: message,
			Offline: offline,
		})
	})
	log.Printf("websocket connected user_id=%d connection_id=%s", claims.UserID, connectionID)

	// 4. 确保在函数退出（WebSocket 断开）时，注销该连接
	defer func() {
		h.chatService.UnregisterConnection(claims.UserID, connectionID)
		log.Printf("websocket disconnected user_id=%d connection_id=%s", claims.UserID, connectionID)
	}()

	// 5. 向客户端发送连接成功的确认消息
	if err := writer.Write(ctx, chatOutgoingMessage{
		Type:   "connected",
		UserID: claims.UserID,
	}); err != nil {
		return
	}

	// 6. 开始阻塞式读取客户端发送的消息
	for {
		var incoming chatIncomingMessage
		// 读取并反序列化客户端的 JSON 消息
		if err := wsjson.Read(ctx, conn, &incoming); err != nil {
			return // 发生读取错误或客户端断开连接，退出循环
		}

		// 检查消息类型，目前仅处理 "chat" 类型的消息
		if incoming.Type != "chat" {
			if err := writer.Write(ctx, chatOutgoingMessage{
				Type:  "error",
				Error: "不支持的消息类型",
			}); err != nil {
				return
			}
			continue
		}

		// 校验接收方 ID 是否有效
		if incoming.ToUserID == 0 {
			if err := writer.Write(ctx, chatOutgoingMessage{
				Type:  "error",
				Error: "接收方不能为空",
			}); err != nil {
				return
			}
			continue
		}

		// 7. 调用业务逻辑层，发送消息给目标用户
		message, err := h.chatService.SendMessage(claims.UserID, incoming.ToUserID, incoming.MessageType, incoming.Content)
		if err != nil {
			// 如果发送失败（如非好友关系、内容为空），返回错误信息给客户端
			if err := writer.Write(ctx, chatOutgoingMessage{
				Type:  "error",
				Error: err.Error(),
			}); err != nil {
				return
			}
			continue
		}

		// 8. 消息发送成功后，给发送方（即当前客户端）返回 "sent" 确认消息
		if err := writer.Write(ctx, chatOutgoingMessage{
			Type:    "sent",
			Message: message,
		}); err != nil {
			return
		}
	}
}

// extractToken 尝试从请求中提取 JWT token。
// 它会先检查 URL 查询参数 "token"，如果没有找到，则检查 HTTP Header 中的 "Authorization: Bearer <token>" 字段。
func (h *ChatHandler) extractToken(c *gin.Context) string {
	// 优先从查询参数中获取 token
	tokenString := strings.TrimSpace(c.Query("token"))
	if tokenString != "" {
		return tokenString
	}

	// 如果没有，尝试从 Authorization 头中提取
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && parts[0] == "Bearer" {
		return parts[1]
	}

	return ""
}

// Write 向 WebSocket 连接发送 JSON 格式的数据。
// 使用互斥锁 mu 确保在并发推送（如被 ChatService 异步调用）时不会发生写入冲突。
func (w *chatSocketWriter) Write(ctx context.Context, payload chatOutgoingMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return wsjson.Write(ctx, w.conn, payload)
}
