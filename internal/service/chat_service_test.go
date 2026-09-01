package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"sleet0922/graduation_project/internal/model"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func mustTestJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test payload: %v", err)
	}
	return string(encoded)
}

func useMiniRedis(t *testing.T) (*miniredis.Miniredis, *goredis.Client) {
	t.Helper()
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})
	return server, client
}

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

func TestChatServiceRejectsMissingRepositoryDependencies(t *testing.T) {
	svc := NewChatService(nil, nil)
	if _, err := svc.SendMessage(context.Background(), 1, 2, 0, "text", "hello"); !errors.Is(err, ErrChatServiceUnavailable) {
		t.Fatalf("SendMessage error = %v, want ErrChatServiceUnavailable", err)
	}
	if err := svc.MarkRead(context.Background(), 1, 2, 0); err != nil {
		t.Fatalf("MarkRead without repository should be a no-op, got %v", err)
	}
}

func TestChatServiceRemovesConnectionsWithoutDeliveryCallback(t *testing.T) {
	friends := newFakeFriendRepo()
	friends.friendships[[2]uint{1, 2}] = true
	svc := NewChatService(friends, newFakeGroupRepo())
	svc.RegisterConnection(context.Background(), 2, nil, nil, nil)
	if _, err := svc.SendMessage(context.Background(), 1, 2, 0, "text", "hello"); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if ids := svc.GetConnectionIDs(2); len(ids) != 0 {
		t.Fatalf("connection with nil delivery callback remained: %#v", ids)
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

func TestChatServiceDoesNotQueueOrReplayLiveDelivery(t *testing.T) {
	server, redisClient := useMiniRedis(t)
	ctx := context.Background()
	friendRepo := newFakeFriendRepo()
	friendRepo.friendships[[2]uint{1, 2}] = true
	svc := NewChatService(friendRepo, newFakeGroupRepo(), WithRedisClient(redisClient))

	deliveryCount := 0
	connectionID := svc.RegisterConnection(ctx, 2, func(*model.ChatMessage, bool) error {
		deliveryCount++
		return nil
	}, nil, nil)
	if _, err := svc.SendMessage(ctx, 1, 2, 0, "text", "live"); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if deliveryCount != 1 {
		t.Fatalf("live delivery count = %d, want 1", deliveryCount)
	}
	if server.Exists(chatPushKey(2)) {
		t.Fatalf("live-delivered message was persisted at %q", chatPushKey(2))
	}

	svc.UnregisterConnection(2, connectionID)
	svc.RegisterConnection(ctx, 2, func(*model.ChatMessage, bool) error {
		deliveryCount++
		return nil
	}, nil, nil)
	if deliveryCount != 1 {
		t.Fatalf("live message was replayed after reconnect; delivery count = %d", deliveryCount)
	}
}

func TestChatServiceOfflineDeliveryClearsRedisCopy(t *testing.T) {
	server, redisClient := useMiniRedis(t)
	ctx := context.Background()
	friendRepo := newFakeFriendRepo()
	friendRepo.friendships[[2]uint{1, 2}] = true
	svc := NewChatService(friendRepo, newFakeGroupRepo(), WithRedisClient(redisClient))

	message, err := svc.SendMessage(ctx, 1, 2, 0, "text", "offline")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	got, err := server.List(chatPushKey(2))
	if err != nil {
		t.Fatalf("read Redis queue: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("queued Redis messages = %d, want 1", len(got))
	}

	var deliveredIDs []string
	connectionID := svc.RegisterConnection(ctx, 2, func(message *model.ChatMessage, offline bool) error {
		if !offline {
			t.Fatal("queued message was not marked offline")
		}
		deliveredIDs = append(deliveredIDs, message.ID)
		return nil
	}, nil, nil)
	if len(deliveredIDs) != 1 || deliveredIDs[0] != message.ID {
		t.Fatalf("delivered IDs = %#v, want [%q]", deliveredIDs, message.ID)
	}
	if server.Exists(chatPushKey(2)) {
		t.Fatalf("Redis copy remained after successful offline delivery at %q", chatPushKey(2))
	}

	svc.UnregisterConnection(2, connectionID)
	svc.RegisterConnection(ctx, 2, func(message *model.ChatMessage, offline bool) error {
		deliveredIDs = append(deliveredIDs, message.ID)
		return nil
	}, nil, nil)
	if len(deliveredIDs) != 1 {
		t.Fatalf("offline message was replayed after reconnect; IDs = %#v", deliveredIDs)
	}
}

func TestInspectChatEnvelope(t *testing.T) {
	metadata := extractChatEnvelopeMetadata(`{"e2ee":1,"v":"x25519+chacha20poly1305:v1","key_id":"0123456789abcdef","nonce":"secret","ct":"secret"}`)
	if metadata.E2EE != 1 || metadata.Version != "x25519+chacha20poly1305:v1" || metadata.KeyID != "0123456789abcdef" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if metadata := extractChatEnvelopeMetadata("plain text"); metadata.E2EE != 0 || metadata.KeyID != "" {
		t.Fatalf("plain text metadata = %#v, want empty", metadata)
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

func TestChatServiceRejectsStaleE2EEKeyStateBeforeDelivery(t *testing.T) {
	ctx := context.Background()
	groups := newFakeGroupRepo()
	groups.groups[10] = &model.ChatGroup{ID: 10, OwnerID: 1, Name: "group"}
	groups.members[10] = map[uint]*model.ChatGroupMember{
		1: {GroupID: 10, UserID: 1, Role: "owner"},
		2: {GroupID: 10, UserID: 2, Role: "member"},
	}
	friends := newFakeFriendRepo()
	friends.friendships[[2]uint{1, 2}] = true

	keyRepo := newFakeE2EEKeyRepo()
	senderKeyBytes := make([]byte, 32)
	senderKeyBytes[0] = 1
	recipientKeyBytes := make([]byte, 32)
	recipientKeyBytes[0] = 2
	keyRepo.keys[1] = &model.E2EEUserPublicKey{UserID: 1, PublicKey: base64.StdEncoding.EncodeToString(senderKeyBytes)}
	keyRepo.keys[2] = &model.E2EEUserPublicKey{UserID: 2, PublicKey: base64.StdEncoding.EncodeToString(recipientKeyBytes)}
	senderKeyID, _ := e2eePublicKeyID(keyRepo.keys[1].PublicKey)
	recipientKeyID, _ := e2eePublicKeyID(keyRepo.keys[2].PublicKey)
	groupKeys := newFakeE2EEGroupKeyRepo()
	groupKeys.versions[10] = 3
	wrappedKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	wrapNonce := base64.StdEncoding.EncodeToString(make([]byte, 12))
	for _, userID := range []uint{1, 2} {
		groupKeys.boxes[boxKey(10, 3, userID)] = &model.E2EEGroupKeyBox{
			GroupID:         10,
			KeyVersion:      3,
			UserID:          userID,
			WrappedGroupKey: wrappedKey,
			WrapNonce:       wrapNonce,
			KeyWrapAlg:      "chacha20poly1305-v1",
			WrappedByUserID: 1,
		}
	}

	svc := NewChatService(
		friends,
		groups,
		WithE2EEMessageValidation(keyRepo, groupKeys),
	)
	delivered := 0
	svc.RegisterConnection(ctx, 2, func(*model.ChatMessage, bool) error {
		delivered++
		return nil
	}, nil, nil)

	validNonce := base64.StdEncoding.EncodeToString(make([]byte, 12))
	validCiphertext := base64.StdEncoding.EncodeToString(make([]byte, 17))
	directEnvelope := map[string]any{
		"e2ee": 1, "v": "x25519+chacha20poly1305:v1",
		"nonce": validNonce, "ct": validCiphertext, "key_id": "0123456789abcdef",
		"sender_key_id": senderKeyID, "recipient_key_id": recipientKeyID,
	}
	if _, err := svc.SendMessage(ctx, 1, 2, 0, "text", mustTestJSON(t, directEnvelope)); err != nil {
		t.Fatalf("valid direct envelope rejected: %v", err)
	}
	directEnvelope["nonce"] = "not-base64"
	if _, err := svc.SendMessage(ctx, 1, 2, 0, "text", mustTestJSON(t, directEnvelope)); !errors.Is(err, ErrE2EEMessageMalformed) {
		t.Fatalf("malformed direct envelope error = %v, want ErrE2EEMessageMalformed", err)
	}
	directEnvelope["nonce"] = validNonce
	directEnvelope["recipient_key_id"] = senderKeyID
	if _, err := svc.SendMessage(ctx, 1, 2, 0, "text", mustTestJSON(t, directEnvelope)); !errors.Is(err, ErrE2EERecipientKeyStale) {
		t.Fatalf("stale direct recipient key error = %v", err)
	}

	groupEnvelope := map[string]any{
		"e2ee": 1, "v": "group+chacha20poly1305:v1", "scope": "group",
		"group_id": 10, "key_version": 3, "nonce": validNonce, "ct": validCiphertext,
		"sender_key_id": senderKeyID,
	}
	if _, err := svc.SendMessage(ctx, 1, 0, 10, "text", mustTestJSON(t, groupEnvelope)); err != nil {
		t.Fatalf("valid group envelope rejected: %v", err)
	}
	groupEnvelope["key_version"] = 2
	if _, err := svc.SendMessage(ctx, 1, 0, 10, "text", mustTestJSON(t, groupEnvelope)); !errors.Is(err, ErrE2EEGroupKeyStale) {
		t.Fatalf("stale group key error = %v", err)
	}
	groupEnvelope["key_version"] = 3
	delete(groupKeys.boxes, boxKey(10, 3, 2))
	if _, err := svc.SendMessage(ctx, 1, 0, 10, "text", mustTestJSON(t, groupEnvelope)); !errors.Is(err, ErrE2EEGroupKeyNotReady) {
		t.Fatalf("incomplete group key boxes error = %v", err)
	}

	if delivered != 2 {
		t.Fatalf("delivery count = %d, want only two valid encrypted messages", delivered)
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
