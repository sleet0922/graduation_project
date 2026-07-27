package handler

import (
	"context"
	"testing"
	"time"

	"sleet0922/graduation_project/internal/service"
)

func TestChatHandlerForegroundDisconnectGrace(t *testing.T) {
	chat := service.NewChatService(nil, nil)
	disconnected := make(chan uint, 1)
	handler := NewChatHandler(chat, &fakeRTCService{
		disconnectedFn: func(_ context.Context, userID uint) error {
			disconnected <- userID
			return nil
		},
	})
	handler.foregroundDisconnectGrace = 20 * time.Millisecond

	handler.scheduleForegroundDisconnect(42)
	select {
	case userID := <-disconnected:
		if userID != 42 {
			t.Fatalf("disconnected user = %d, want 42", userID)
		}
	case <-time.After(time.Second):
		t.Fatal("foreground disconnect was not handled after grace period")
	}
}

func TestChatHandlerReconnectCancelsDisconnect(t *testing.T) {
	chat := service.NewChatService(nil, nil)
	disconnected := make(chan uint, 1)
	handler := NewChatHandler(chat, &fakeRTCService{
		disconnectedFn: func(_ context.Context, userID uint) error {
			disconnected <- userID
			return nil
		},
	})
	handler.foregroundDisconnectGrace = 30 * time.Millisecond

	handler.scheduleForegroundDisconnect(42)
	chat.RegisterConnection(context.Background(), 42, nil, nil, nil, service.WithConnectionClient("foreground"))
	handler.cancelForegroundDisconnect(42)
	select {
	case userID := <-disconnected:
		t.Fatalf("reconnected user %d was treated as disconnected", userID)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestChatHandlerBackgroundConnectionDoesNotMaskForegroundDisconnect(t *testing.T) {
	chat := service.NewChatService(nil, nil)
	chat.RegisterConnection(context.Background(), 42, nil, nil, nil, service.WithConnectionClient("background"))
	disconnected := make(chan uint, 1)
	handler := NewChatHandler(chat, &fakeRTCService{
		disconnectedFn: func(_ context.Context, userID uint) error {
			disconnected <- userID
			return nil
		},
	})
	handler.foregroundDisconnectGrace = 20 * time.Millisecond

	handler.scheduleForegroundDisconnect(42)
	select {
	case userID := <-disconnected:
		if userID != 42 {
			t.Fatalf("disconnected user = %d, want 42", userID)
		}
	case <-time.After(time.Second):
		t.Fatal("background connection masked foreground disconnect")
	}
}
