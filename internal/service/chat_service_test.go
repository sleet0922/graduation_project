package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"sleet0922/graduation_project/internal/model"
)

func TestChatServiceRecallWindowAndOwnership(t *testing.T) {
	ctx := context.Background()
	friendRepo := newFakeFriendRepo()
	friendRepo.friendships[[2]uint{1, 2}] = true
	svc := NewChatService(friendRepo, newFakeGroupRepo())

	var recalledEvent map[string]any
	svc.RegisterConnection(ctx, 2, func(*model.ChatMessage, bool) error {
		return nil
	}, func(payload any) error {
		recalledEvent, _ = payload.(map[string]any)
		return nil
	}, nil)

	message, err := svc.SendMessage(ctx, 1, 2, 0, "text", "hello")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if err := svc.RecallMessage(ctx, 3, 2, 0, message.ID); !errors.Is(err, ErrRecallPermission) {
		t.Fatalf("other user recall error = %v, want ErrRecallPermission", err)
	}
	if err := svc.RecallMessage(ctx, 1, 2, 0, message.ID); err != nil {
		t.Fatalf("owner recall failed: %v", err)
	}
	if recalledEvent["type"] != "message_recalled" || recalledEvent["message_id"] != message.ID {
		t.Fatalf("recall event = %#v", recalledEvent)
	}

	expired, err := svc.SendMessage(ctx, 1, 2, 0, "text", "too old")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	implementation := svc.(*chatService)
	implementation.mu.Lock()
	recent := implementation.recentMessages[expired.ID]
	recent.createdAt = time.Now().Add(-messageRecallWindow - time.Second)
	implementation.recentMessages[expired.ID] = recent
	implementation.mu.Unlock()

	if err := svc.RecallMessage(ctx, 1, 2, 0, expired.ID); !errors.Is(err, ErrRecallExpired) {
		t.Fatalf("expired recall error = %v, want ErrRecallExpired", err)
	}
}

func TestChatServiceSendSingleMessage(t *testing.T) {
	ctx := context.Background()
	friendRepo := newFakeFriendRepo()
	groupRepo := newFakeGroupRepo()
	svc := NewChatService(friendRepo, groupRepo)

	if _, err := svc.SendMessage(ctx, 1, 2, 0, "text", ""); !errors.Is(err, ErrMessageEmpty) {
		t.Fatalf("empty content error = %v, want ErrMessageEmpty", err)
	}
	if _, err := svc.SendMessage(ctx, 1, 2, 0, "text", "hello"); !errors.Is(err, ErrMessagePermission) {
		t.Fatalf("non friend error = %v, want ErrMessagePermission", err)
	}

	friendRepo.friendships[[2]uint{1, 2}] = true
	var delivered []*model.ChatMessage
	connID := svc.RegisterConnection(ctx, 2, func(message *model.ChatMessage, offline bool) error {
		if offline {
			t.Fatal("first live delivery was marked offline")
		}
		delivered = append(delivered, message)
		return nil
	}, nil, nil)
	if connID == "" {
		t.Fatal("RegisterConnection returned empty id")
	}

	message, err := svc.SendMessage(ctx, 1, 2, 0, "", "hello")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if message.ConversationType != "single" || message.MessageType != "text" {
		t.Fatalf("message type = %q/%q, want single/text", message.ConversationType, message.MessageType)
	}
	if len(delivered) != 1 || delivered[0].Content != "hello" {
		t.Fatalf("delivered messages = %#v, want one hello", delivered)
	}

	svc.UnregisterConnection(2, connID)
	if ids := svc.GetConnectionIDs(2); len(ids) != 0 {
		t.Fatalf("connection IDs after unregister = %#v, want empty", ids)
	}
}

func TestChatServiceTracksConnectionClients(t *testing.T) {
	ctx := context.Background()
	svc := NewChatService(newFakeFriendRepo(), newFakeGroupRepo())

	foregroundID := svc.RegisterConnection(ctx, 7, nil, nil, nil, WithConnectionClient("foreground"))
	backgroundID := svc.RegisterConnection(ctx, 7, nil, nil, nil, WithConnectionClient("background"))
	if !svc.HasConnectionClient(7, "foreground") || !svc.HasConnectionClient(7, "background") {
		t.Fatal("registered foreground and background connections were not tracked")
	}

	svc.UnregisterConnection(7, backgroundID)
	if !svc.HasConnectionClient(7, "foreground") {
		t.Fatal("removing background connection removed foreground presence")
	}
	if svc.HasConnectionClient(7, "background") {
		t.Fatal("background connection remained present after unregister")
	}

	svc.UnregisterConnection(7, foregroundID)
	if svc.HasConnectionClient(7, "foreground") {
		t.Fatal("foreground connection remained present after unregister")
	}
}

func TestChatServiceOfflineQueuesAreDrainedOnRegister(t *testing.T) {
	ctx := context.Background()
	friendRepo := newFakeFriendRepo()
	friendRepo.friendships[[2]uint{1, 2}] = true
	svc := NewChatService(friendRepo, newFakeGroupRepo())

	if _, err := svc.SendMessage(ctx, 1, 2, 0, "text", "queued"); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	var offlineFlags []bool
	var contents []string
	svc.RegisterConnection(ctx, 2, func(message *model.ChatMessage, offline bool) error {
		offlineFlags = append(offlineFlags, offline)
		contents = append(contents, message.Content)
		return nil
	}, nil, nil)

	if len(contents) != 1 || contents[0] != "queued" {
		t.Fatalf("offline delivered contents = %#v, want queued", contents)
	}
	if len(offlineFlags) != 1 || !offlineFlags[0] {
		t.Fatalf("offline flags = %#v, want true", offlineFlags)
	}
}

func TestChatServiceGroupMessagePermissionsAndDelivery(t *testing.T) {
	ctx := context.Background()
	groupRepo := newFakeGroupRepo()
	groupRepo.groups[10] = &model.ChatGroup{ID: 10, OwnerID: 1, Name: "group"}
	groupRepo.members[10] = map[uint]*model.ChatGroupMember{
		1: {GroupID: 10, UserID: 1, Role: "owner"},
		2: {GroupID: 10, UserID: 2, Role: "member"},
		3: {GroupID: 10, UserID: 3, Role: "member"},
	}
	svc := NewChatService(newFakeFriendRepo(), groupRepo)

	if _, err := svc.SendMessage(ctx, 99, 0, 10, "text", "hello"); !errors.Is(err, ErrGroupMessagePermission) {
		t.Fatalf("non member group send error = %v, want ErrGroupMessagePermission", err)
	}

	delivered := map[uint]int{}
	svc.RegisterConnection(ctx, 2, func(message *model.ChatMessage, offline bool) error {
		delivered[2]++
		return nil
	}, nil, nil)
	svc.RegisterConnection(ctx, 3, func(message *model.ChatMessage, offline bool) error {
		delivered[3]++
		return nil
	}, nil, nil)

	message, err := svc.SendMessage(ctx, 1, 0, 10, "text", "hi group")
	if err != nil {
		t.Fatalf("group SendMessage failed: %v", err)
	}
	if message.ConversationType != "group" || message.GroupID != 10 {
		t.Fatalf("group message = %#v, want group 10", message)
	}
	if delivered[2] != 1 || delivered[3] != 1 {
		t.Fatalf("delivered counts = %#v, want one per non-sender member", delivered)
	}
}

func TestChatServiceSystemEventsAndKick(t *testing.T) {
	ctx := context.Background()
	svc := NewChatService(newFakeFriendRepo(), newFakeGroupRepo())

	results := svc.PushSystemEvent(ctx, []uint{42}, map[string]any{"type": "notice"})
	if len(results) != 1 || results[0].Online {
		t.Fatalf("offline system push result = %#v, want one offline result", results)
	}

	var systemPayloads []any
	var closed bool
	svc.RegisterConnection(ctx, 42, nil, func(payload any) error {
		systemPayloads = append(systemPayloads, payload)
		return nil
	}, func() {
		closed = true
	})
	if len(systemPayloads) != 1 {
		t.Fatalf("offline system events delivered = %d, want 1", len(systemPayloads))
	}

	svc.KickUserConnections(42, "relogin")
	if !closed {
		t.Fatal("KickUserConnections did not close connection")
	}
	if ids := svc.GetConnectionIDs(42); len(ids) != 0 {
		t.Fatalf("connection IDs after kick = %#v, want empty", ids)
	}
}
