package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sleet0922/graduation_project/internal/handler"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/jwt"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
)

type mockChatService struct{}

func (m *mockChatService) RegisterConnection(userID uint, deliver service.DeliveryFunc) string {
	return "mock-conn-id"
}

func (m *mockChatService) UnregisterConnection(userID uint, connectionID string) {}

func (m *mockChatService) SendMessage(fromUserID, toUserID uint, messageType string, content string) (*model.ChatMessage, error) {
	if content == "" {
		return nil, service.ErrMessageEmpty
	}
	return &model.ChatMessage{
		ID:          "mock-msg-id",
		FromUserID:  fromUserID,
		ToUserID:    toUserID,
		MessageType: messageType,
		Content:     content,
		CreatedAt:   time.Now(),
	}, nil
}

func TestChatHandler_Connect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockChatService := &mockChatService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	chatHandler := handler.NewChatHandler(mockChatService, jwtManager)

	t.Run("NoToken", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest(http.MethodGet, "/chat", nil)
		c.Request = req

		chatHandler.Connect(c)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status Unauthorized, got %v", w.Code)
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest(http.MethodGet, "/chat?token=invalid", nil)
		c.Request = req

		chatHandler.Connect(c)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status Unauthorized, got %v", w.Code)
		}
	})

	t.Run("WebSocketFlow", func(t *testing.T) {
		token, _ := jwtManager.GenerateToken(1, "testuser", 3600*time.Second)

		router := gin.New()
		router.GET("/chat", chatHandler.Connect)
		server := httptest.NewServer(router)
		defer server.Close()

		wsURL := "ws" + server.URL[4:] + "/chat?token=" + token
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect websocket: %v", err)
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		// 1. Check connect message
		var msg map[string]interface{}
		err = wsjson.Read(ctx, conn, &msg)
		if err != nil {
			t.Fatalf("Failed to read connect message: %v", err)
		}
		if msg["type"] != "connected" {
			t.Errorf("Expected connected type, got %v", msg["type"])
		}

		// 2. Send invalid type message
		err = wsjson.Write(ctx, conn, map[string]interface{}{
			"type": "invalid_type",
		})
		if err != nil {
			t.Fatalf("Failed to write: %v", err)
		}
		err = wsjson.Read(ctx, conn, &msg)
		if err != nil {
			t.Fatalf("Failed to read error message: %v", err)
		}
		if msg["type"] != "error" {
			t.Errorf("Expected error type, got %v", msg["type"])
		}

		// 3. Send valid message
		err = wsjson.Write(ctx, conn, map[string]interface{}{
			"type":         "chat",
			"to_user_id":   2,
			"message_type": "text",
			"content":      "hello",
		})
		if err != nil {
			t.Fatalf("Failed to write chat message: %v", err)
		}
		err = wsjson.Read(ctx, conn, &msg)
		if err != nil {
			t.Fatalf("Failed to read sent message: %v", err)
		}
		if msg["type"] != "sent" {
			t.Errorf("Expected sent type, got %v", msg["type"])
		}
	})
}
