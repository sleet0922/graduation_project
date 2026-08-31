package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/pkg/logger"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var (
	ErrMessageEmpty            = errors.New("消息内容不能为空")
	ErrMessagePermission       = errors.New("只能给好友发送消息")
	ErrGroupMessagePermission  = errors.New("群聊已解散")
	ErrRecallExpired           = errors.New("消息只能在发出后1分钟内撤回")
	ErrRecallPermission        = errors.New("只能撤回自己发送的消息")
	ErrE2EEMessageMalformed    = errors.New("端到端加密消息格式无效")
	ErrE2EESenderKeyStale      = errors.New("发送方身份密钥已更新，请同步密钥后重试")
	ErrE2EERecipientKeyStale   = errors.New("接收方身份密钥已更新，请重新加密后重试")
	ErrE2EEGroupKeyStale       = errors.New("群聊密钥版本已更新，请重新加密后重试")
	ErrE2EEGroupKeyNotReady    = errors.New("群聊当前密钥尚未完成全员分发，请稍后重试")
	ErrE2EEKeyStateUnavailable = errors.New("暂时无法确认服务端当前密钥状态")
	ErrChatServiceUnavailable  = errors.New("聊天服务依赖不可用")
)

const messageRecallWindow = time.Minute

const chatPushKeyPrefix = "chat:push:v2:"

type chatEnvelopeMetadata struct {
	E2EE           int    `json:"e2ee"`
	Version        string `json:"v"`
	Scope          string `json:"scope"`
	GroupID        uint   `json:"group_id"`
	KeyID          string `json:"key_id"`
	KeyVersion     int    `json:"key_version"`
	SenderKeyID    string `json:"sender_key_id"`
	RecipientKeyID string `json:"recipient_key_id"`
	Nonce          string `json:"nonce"`
	CipherText     string `json:"ct"`
}

type DeliveryFunc func(message *model.ChatMessage, offline bool) error

type SystemDeliveryFunc func(payload any) error

type SystemPushResult struct {
	UserID              uint     // 推给谁
	Online              bool     // 用户是否在线
	ConnectionIDs       []string // 用户所有在线连接
	SuccessfulConnIDs   []string // 哪些连接发成功了
	FailedConnIDs       []string // 哪些连接发失败了
	ErrorMessages       []string // 失败原因
	SuccessfulPushCount int      // 成功发了几条
}

type ChatService interface {
	RegisterConnection(ctx context.Context, userID uint, deliver DeliveryFunc, sysDeliver SystemDeliveryFunc, closeConn func(), opts ...RegisterConnectionOption) string
	UnregisterConnection(userID uint, connectionID string)
	SendMessage(ctx context.Context, fromUserID, toUserID, groupID uint, messageType string, content string) (*model.ChatMessage, error)
	// RecallMessage 通知接收方（或群成员）撤回指定消息
	RecallMessage(ctx context.Context, fromUserID, toUserID, groupID uint, messageID string) error
	// MarkRead 通知发送方消息已被接收方读取
	MarkRead(ctx context.Context, readerID, peerID, groupID uint) error
	BroadcastGroupDissolved(ctx context.Context, groupID uint, userIDs []uint)
	PushSystemEvent(ctx context.Context, userIDs []uint, payload any) []SystemPushResult
	GetConnectionIDs(userID uint) []string
	HasConnectionClient(userID uint, client string) bool
	KickUserConnections(userID uint, reason string)
}

type chatConnection struct {
	id         string
	client     string
	deliver    DeliveryFunc
	sysDeliver SystemDeliveryFunc
	closeFn    func()
}

type queuedSystemEvent struct {
	id      string
	payload any
}

type recentChatMessage struct {
	fromUserID uint
	toUserID   uint
	groupID    uint
	createdAt  time.Time
}

type registerConnectionOptions struct {
	DrainOfflineMessages bool
	Client               string
}

type RegisterConnectionOption func(*registerConnectionOptions)

func WithOfflineDrain(enabled bool) RegisterConnectionOption {
	return func(opts *registerConnectionOptions) {
		opts.DrainOfflineMessages = enabled
	}
}

func WithConnectionClient(client string) RegisterConnectionOption {
	return func(opts *registerConnectionOptions) {
		opts.Client = client
	}
}

func normalizeConnectionClient(client string) string {
	if strings.EqualFold(strings.TrimSpace(client), "background") {
		return "background"
	}
	return "foreground"
}

type chatService struct {
	friendRepo     repo.FriendRepository
	groupRepo      repo.GroupRepository
	e2eeKeyRepo    repo.E2EEKeyRepository
	e2eeGroupRepo  repo.E2EEGroupKeyRepository
	mu             sync.RWMutex
	sequence       uint64
	connections    map[uint]map[string]*chatConnection
	offline        map[uint][]*model.ChatMessage
	systemOffline  map[uint][]*queuedSystemEvent
	recentMessages map[string]recentChatMessage
	redisClient    *goredis.Client
}

type ChatServiceOption func(*chatService)

func WithE2EEMessageValidation(keyRepo repo.E2EEKeyRepository, groupKeyRepo repo.E2EEGroupKeyRepository) ChatServiceOption {
	return func(service *chatService) {
		service.e2eeKeyRepo = keyRepo
		service.e2eeGroupRepo = groupKeyRepo
	}
}

// WithRedisClient injects the offline-message store used by this chat
// service. The client is captured when the service is built, so another
// application instance cannot redirect an existing service by mutating a
// package-global variable.
func WithRedisClient(client *goredis.Client) ChatServiceOption {
	return func(service *chatService) {
		service.redisClient = client
	}
}

func NewChatService(friendRepo repo.FriendRepository, groupRepo repo.GroupRepository, options ...ChatServiceOption) ChatService {
	service := &chatService{
		friendRepo:     friendRepo,
		groupRepo:      groupRepo,
		connections:    make(map[uint]map[string]*chatConnection),
		offline:        make(map[uint][]*model.ChatMessage),
		systemOffline:  make(map[uint][]*queuedSystemEvent),
		recentMessages: make(map[string]recentChatMessage),
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func e2eePublicKeyID(publicKey string) (string, error) {
	decoded, err := decodeBase64URLOrStd(strings.TrimSpace(publicKey))
	if err != nil || len(decoded) != 32 {
		return "", ErrE2EEMessageMalformed
	}
	digest := sha256.Sum256(decoded)
	return hex.EncodeToString(digest[:]), nil
}

func validE2EEHexID(value string, byteLength int) bool {
	if len(value) != byteLength*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateE2EEEnvelopeEncoding(metadata chatEnvelopeMetadata) error {
	nonce, nonceErr := decodeBase64URLOrStd(strings.TrimSpace(metadata.Nonce))
	cipherText, cipherTextErr := decodeBase64URLOrStd(strings.TrimSpace(metadata.CipherText))
	if nonceErr != nil || len(nonce) != 12 || cipherTextErr != nil || len(cipherText) <= 16 {
		return ErrE2EEMessageMalformed
	}
	if !validE2EEHexID(metadata.SenderKeyID, 32) {
		return ErrE2EEMessageMalformed
	}
	return nil
}

func (s *chatService) currentE2EEPublicKeyID(ctx context.Context, userID uint) (string, error) {
	key, err := s.e2eeKeyRepo.GetByUserID(ctx, userID)
	if err != nil || key == nil {
		return "", ErrE2EEKeyStateUnavailable
	}
	keyID, err := e2eePublicKeyID(key.PublicKey)
	if err != nil {
		return "", ErrE2EEKeyStateUnavailable
	}
	return keyID, nil
}

func (s *chatService) validateE2EEMessage(ctx context.Context, fromUserID, toUserID, groupID uint, messageType, content string) error {
	if s.e2eeKeyRepo == nil && s.e2eeGroupRepo == nil {
		return nil
	}
	if messageType == "call" || messageType == "video" {
		return nil
	}
	var metadata chatEnvelopeMetadata
	if err := json.Unmarshal([]byte(content), &metadata); err != nil || metadata.E2EE != 1 {
		return ErrE2EEMessageMalformed
	}
	if s.e2eeKeyRepo == nil || s.e2eeGroupRepo == nil ||
		strings.TrimSpace(metadata.Nonce) == "" || strings.TrimSpace(metadata.CipherText) == "" {
		return ErrE2EEMessageMalformed
	}
	if err := validateE2EEEnvelopeEncoding(metadata); err != nil {
		return err
	}

	senderKeyID, err := s.currentE2EEPublicKeyID(ctx, fromUserID)
	if err != nil {
		return err
	}
	if metadata.SenderKeyID != senderKeyID {
		return ErrE2EESenderKeyStale
	}

	if groupID > 0 {
		if metadata.Version != "group+chacha20poly1305:v1" ||
			metadata.Scope != "group" || metadata.GroupID != groupID ||
			metadata.KeyVersion <= 0 {
			return ErrE2EEMessageMalformed
		}
		currentVersion, versionErr := s.e2eeGroupRepo.GetCurrentVersion(ctx, groupID)
		if versionErr != nil {
			return ErrE2EEKeyStateUnavailable
		}
		if metadata.KeyVersion != currentVersion {
			return ErrE2EEGroupKeyStale
		}
		members, membersErr := s.groupRepo.GetMembersByGroupID(ctx, groupID)
		if membersErr != nil {
			return ErrE2EEKeyStateUnavailable
		}
		boxes, boxesErr := s.e2eeGroupRepo.GetVersionBoxes(ctx, groupID, currentVersion)
		if boxesErr != nil || len(boxes) != len(members) {
			return ErrE2EEGroupKeyNotReady
		}
		memberIDs := make(map[uint]struct{}, len(members))
		for _, member := range members {
			memberIDs[member.UserID] = struct{}{}
		}
		seenBoxUsers := make(map[uint]struct{}, len(boxes))
		for _, box := range boxes {
			if _, isMember := memberIDs[box.UserID]; !isMember || !isSupportedKeyBox(box) {
				return ErrE2EEGroupKeyNotReady
			}
			if _, duplicate := seenBoxUsers[box.UserID]; duplicate {
				return ErrE2EEGroupKeyNotReady
			}
			seenBoxUsers[box.UserID] = struct{}{}
		}
		return nil
	}

	if metadata.Version != "x25519+chacha20poly1305:v1" ||
		metadata.Scope != "" || metadata.GroupID != 0 ||
		!validE2EEHexID(metadata.KeyID, 8) ||
		!validE2EEHexID(metadata.RecipientKeyID, 32) {
		return ErrE2EEMessageMalformed
	}
	recipientKeyID, err := s.currentE2EEPublicKeyID(ctx, toUserID)
	if err != nil {
		return err
	}
	if metadata.RecipientKeyID != recipientKeyID {
		return ErrE2EERecipientKeyStale
	}
	return nil
}

func (s *chatService) trackRecentMessage(message *model.ChatMessage) {
	cutoff := time.Now().Add(-messageRecallWindow)
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, recent := range s.recentMessages {
		if recent.createdAt.Before(cutoff) {
			delete(s.recentMessages, id)
		}
	}
	s.recentMessages[message.ID] = recentChatMessage{
		fromUserID: message.FromUserID,
		toUserID:   message.ToUserID,
		groupID:    message.GroupID,
		createdAt:  message.CreatedAt,
	}
}

func (s *chatService) validateAndConsumeRecall(
	fromUserID, toUserID, groupID uint,
	messageID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	recent, ok := s.recentMessages[messageID]
	if !ok || time.Since(recent.createdAt) > messageRecallWindow {
		delete(s.recentMessages, messageID)
		return ErrRecallExpired
	}
	if recent.fromUserID != fromUserID {
		return ErrRecallPermission
	}
	if recent.groupID > 0 {
		if groupID != recent.groupID {
			return ErrRecallPermission
		}
	} else if groupID > 0 || toUserID != recent.toUserID {
		return ErrRecallPermission
	}
	delete(s.recentMessages, messageID)
	return nil
}

// 从连接map中提取连接id列表
func connectionIDsFromMap(connections map[string]*chatConnection) []string {
	if len(connections) == 0 {
		return nil
	}
	connectionIDs := make([]string, 0, len(connections))
	for connectionID := range connections {
		connectionIDs = append(connectionIDs, connectionID)
	}
	return connectionIDs
}

// 复制一份消息内容
func clonePayload(payload any) (any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var cloned any
	err = json.Unmarshal(data, &cloned)
	if err != nil {
		return nil, err
	}
	return cloned, nil
}

// 推送到redis
func inspectChatEnvelope(content string) chatEnvelopeMetadata {
	var metadata chatEnvelopeMetadata
	if err := json.Unmarshal([]byte(content), &metadata); err != nil || metadata.E2EE != 1 {
		return chatEnvelopeMetadata{}
	}
	metadata.Version = strings.TrimSpace(metadata.Version)
	metadata.KeyID = strings.TrimSpace(metadata.KeyID)
	if len(metadata.Version) > 64 {
		metadata.Version = metadata.Version[:64]
	}
	if len(metadata.KeyID) > 64 {
		metadata.KeyID = metadata.KeyID[:64]
	}
	return metadata
}

func chatMessageLogArgs(message *model.ChatMessage, recipientUserID uint, source, result string) []any {
	metadata := inspectChatEnvelope(message.Content)
	args := []any{
		"message_id", message.ID,
		"from_user_id", message.FromUserID,
		"to_user_id", message.ToUserID,
		"recipient_user_id", recipientUserID,
		"group_id", message.GroupID,
		"message_type", message.MessageType,
		"created_at", message.CreatedAt.UTC().Format(time.RFC3339Nano),
		"delivery_source", source,
		"delivery_result", result,
		"e2ee", metadata.E2EE == 1,
	}
	if metadata.Version != "" {
		args = append(args, "e2ee_version", metadata.Version)
	}
	if metadata.KeyID != "" {
		args = append(args, "key_id", metadata.KeyID)
	}
	if metadata.KeyVersion > 0 {
		args = append(args, "key_version", metadata.KeyVersion)
	}
	return args
}

func logChatMessageDelivery(message *model.ChatMessage, recipientUserID uint, source, result string) {
	logger.Info("chat message delivery", chatMessageLogArgs(message, recipientUserID, source, result)...)
}

func chatPushKey(userID uint) string {
	return fmt.Sprintf("%s%d", chatPushKeyPrefix, userID)
}

func (s *chatService) pushRedisMessage(ctx context.Context, userID uint, message *model.ChatMessage) error {
	if s.redisClient == nil {
		return nil
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}
	pushKey := chatPushKey(userID)
	if err := s.redisClient.RPush(ctx, pushKey, msgBytes).Err(); err != nil {
		return err
	}
	return s.redisClient.Expire(ctx, pushKey, 3*24*time.Hour).Err()
}

// 从redis拉取消息
func (s *chatService) drainRedisMessages(ctx context.Context, userID uint, deliveredIDs map[string]struct{}, deliver DeliveryFunc) {
	if s.redisClient == nil {
		return
	}
	pushKey := chatPushKey(userID)
	rawList, err := s.redisClient.LRange(ctx, pushKey, 0, -1).Result()
	if err != nil || len(rawList) == 0 {
		return
	}
	for _, raw := range rawList {
		var message model.ChatMessage
		if err := json.Unmarshal([]byte(raw), &message); err != nil {
			if removeErr := s.redisClient.LRem(ctx, pushKey, 0, raw).Err(); removeErr != nil {
				logger.Warn("failed to remove malformed offline chat message", "error", removeErr, "user_id", userID)
			}
			continue
		}
		if _, alreadyDelivered := deliveredIDs[message.ID]; alreadyDelivered {
			if removeErr := s.redisClient.LRem(ctx, pushKey, 0, raw).Err(); removeErr != nil {
				logger.Warn("failed to remove duplicate offline chat message", "error", removeErr, "user_id", userID, "message_id", message.ID)
			}
			continue
		}
		if err := deliver(&message, true); err != nil {
			logChatMessageDelivery(&message, userID, "redis_offline", "failed")
			continue
		}
		deliveredIDs[message.ID] = struct{}{}
		if removeErr := s.redisClient.LRem(ctx, pushKey, 0, raw).Err(); removeErr != nil {
			logger.Warn("failed to remove delivered offline chat message", "error", removeErr, "user_id", userID, "message_id", message.ID)
		}
		logChatMessageDelivery(&message, userID, "redis_offline", "delivered")
	}
}

// 将消息加入内存离线队列
func (s *chatService) enqueueOfflineMessage(userID uint, message *model.ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.offline[userID] = append(s.offline[userID], message)
}

// 将系统事件加入内存离线队列
func (s *chatService) enqueueOfflineSystemEvent(userID uint, payload any) {
	clonedPayload, err := clonePayload(payload)
	if err != nil {
		return
	}
	eventID := fmt.Sprintf("sys-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&s.sequence, 1))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.systemOffline[userID] = append(s.systemOffline[userID], &queuedSystemEvent{
		id:      eventID,
		payload: clonedPayload,
	})
}

// 将消息投递给用户的所有连接
func (s *chatService) deliverToUser(ctx context.Context, userID uint, message *model.ChatMessage) {
	s.mu.RLock()
	userConnections := s.connections[userID]
	connections := make([]*chatConnection, 0, len(userConnections))
	for _, connection := range userConnections {
		connections = append(connections, connection)
	}
	s.mu.RUnlock()
	if len(connections) == 0 {
		if err := s.pushRedisMessage(ctx, userID, message); err != nil {
			logger.Warn("failed to persist offline chat message", chatMessageLogArgs(message, userID, "realtime", "redis_failed")...)
		}
		s.enqueueOfflineMessage(userID, message)
		logChatMessageDelivery(message, userID, "realtime", "queued_offline")
		return
	}
	successCount := 0
	failedConnectionIDs := make([]string, 0)
	for _, connection := range connections {
		if connection.deliver == nil {
			failedConnectionIDs = append(failedConnectionIDs, connection.id)
			continue
		}
		err := connection.deliver(message, false)
		if err != nil {
			failedConnectionIDs = append(failedConnectionIDs, connection.id)
			continue
		}
		successCount++
	}
	if len(failedConnectionIDs) > 0 {
		s.mu.Lock()
		if currentConnections, ok := s.connections[userID]; ok {
			for _, connectionID := range failedConnectionIDs {
				delete(currentConnections, connectionID)
			}
			if len(currentConnections) == 0 {
				delete(s.connections, userID)
			}
		}
		s.mu.Unlock()
	}
	if successCount == 0 {
		if err := s.pushRedisMessage(ctx, userID, message); err != nil {
			logger.Warn("failed to persist offline chat message", chatMessageLogArgs(message, userID, "realtime", "redis_failed")...)
		}
		s.enqueueOfflineMessage(userID, message)
		logChatMessageDelivery(message, userID, "realtime", "queued_after_failure")
		return
	}
	logChatMessageDelivery(message, userID, "realtime", "delivered")
}

// 发送群消息
func (s *chatService) sendGroupMessage(ctx context.Context, fromUserID, groupID uint, messageType string, content string) (*model.ChatMessage, error) {
	unlockIdentityState := lockE2EEIdentityRead()
	defer unlockIdentityState()
	unlockGroupState := lockE2EEGroupState(groupID)
	if s == nil || s.groupRepo == nil {
		unlockGroupState()
		return nil, ErrChatServiceUnavailable
	}
	if !s.groupRepo.IsMember(ctx, groupID, fromUserID) {
		unlockGroupState()
		return nil, ErrGroupMessagePermission
	}
	members, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
	if err != nil {
		unlockGroupState()
		return nil, err
	}
	if err := s.validateE2EEMessage(ctx, fromUserID, 0, groupID, messageType, content); err != nil {
		unlockGroupState()
		return nil, err
	}
	message := &model.ChatMessage{
		ID:               fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddUint64(&s.sequence, 1)),
		ConversationType: "group",
		FromUserID:       fromUserID,
		GroupID:          groupID,
		MessageType:      messageType,
		Content:          content,
		CreatedAt:        time.Now(),
	}
	s.trackRecentMessage(message)
	unlockGroupState()
	for _, member := range members {
		if member.UserID == fromUserID {
			continue
		}
		s.deliverToUser(ctx, member.UserID, message)
	}
	return message, nil
}

// ----------公共方法----------
// 发送好友消息
func (s *chatService) RegisterConnection(ctx context.Context, userID uint, deliver DeliveryFunc, sysDeliver SystemDeliveryFunc, closeConn func(), opts ...RegisterConnectionOption) string {
	options := registerConnectionOptions{
		DrainOfflineMessages: true,
		Client:               "foreground",
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	options.Client = normalizeConnectionClient(options.Client)

	connectionID := fmt.Sprintf("%d", atomic.AddUint64(&s.sequence, 1))
	s.mu.Lock()
	if s.connections[userID] == nil {
		s.connections[userID] = make(map[string]*chatConnection)
	}
	s.connections[userID][connectionID] = &chatConnection{
		id:         connectionID,
		client:     options.Client,
		deliver:    deliver,
		sysDeliver: sysDeliver,
		closeFn:    closeConn,
	}
	pending := append([]*model.ChatMessage(nil), s.offline[userID]...)
	pendingSystem := append([]*queuedSystemEvent(nil), s.systemOffline[userID]...)
	connectionIDs := connectionIDsFromMap(s.connections[userID])
	s.mu.Unlock()
	logger.Info("websocket connection registered", "user_id", userID, "connection_ids", connectionIDs, "connection_count", len(connectionIDs), "client", options.Client, "drain_offline", options.DrainOfflineMessages)
	delivered := make(map[string]struct{}, len(pending))
	if options.DrainOfflineMessages && deliver != nil && len(pending) > 0 {
		for _, message := range pending {
			if err := deliver(message, true); err == nil {
				delivered[message.ID] = struct{}{}
				logChatMessageDelivery(message, userID, "memory_offline", "delivered")
			} else {
				logChatMessageDelivery(message, userID, "memory_offline", "failed")
			}
		}
		if len(delivered) > 0 {
			s.mu.Lock()
			queue := s.offline[userID]
			remaining := make([]*model.ChatMessage, 0, len(queue))
			for _, message := range queue {
				if _, ok := delivered[message.ID]; !ok {
					remaining = append(remaining, message)
				}
			}
			if len(remaining) == 0 {
				delete(s.offline, userID)
			} else {
				s.offline[userID] = remaining
			}
			s.mu.Unlock()
		}
	}
	if len(pendingSystem) > 0 && sysDeliver != nil {
		deliveredSystem := make(map[string]struct{}, len(pendingSystem))
		for _, event := range pendingSystem {
			if event == nil {
				continue
			}
			if err := sysDeliver(event.payload); err == nil {
				deliveredSystem[event.id] = struct{}{}
			}
		}
		if len(deliveredSystem) > 0 {
			s.mu.Lock()
			systemQueue := s.systemOffline[userID]
			remainingSystem := make([]*queuedSystemEvent, 0, len(systemQueue))
			for _, event := range systemQueue {
				if event == nil {
					continue
				}
				if _, ok := deliveredSystem[event.id]; !ok {
					remainingSystem = append(remainingSystem, event)
				}
			}
			if len(remainingSystem) == 0 {
				delete(s.systemOffline, userID)
			} else {
				s.systemOffline[userID] = remainingSystem
			}
			s.mu.Unlock()
		}
	}
	if options.DrainOfflineMessages {
		s.drainRedisMessages(ctx, userID, delivered, deliver)
	}

	return connectionID
}

// 注销连接
func (s *chatService) UnregisterConnection(userID uint, connectionID string) {
	s.mu.Lock()
	userConnections, ok := s.connections[userID]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(userConnections, connectionID)
	if len(userConnections) == 0 {
		delete(s.connections, userID)
	}
	connectionIDs := connectionIDsFromMap(userConnections)
	s.mu.Unlock()
	logger.Info("websocket connection unregistered", "user_id", userID, "connection_id", connectionID, "remaining_connection_ids", connectionIDs, "connection_count", len(connectionIDs))
}

// 发送消息
func (s *chatService) SendMessage(ctx context.Context, fromUserID, toUserID, groupID uint, messageType string, content string) (*model.ChatMessage, error) {
	if s == nil || s.friendRepo == nil && groupID == 0 {
		return nil, ErrChatServiceUnavailable
	}
	if content == "" {
		return nil, ErrMessageEmpty
	}
	if messageType == "" {
		messageType = "text"
	}
	if groupID > 0 {
		return s.sendGroupMessage(ctx, fromUserID, groupID, messageType, content)
	}
	if !s.friendRepo.CheckFriendship(ctx, fromUserID, toUserID) {
		return nil, ErrMessagePermission
	}
	unlockIdentityState := lockE2EEIdentityRead()
	defer unlockIdentityState()
	if err := s.validateE2EEMessage(ctx, fromUserID, toUserID, 0, messageType, content); err != nil {
		return nil, err
	}
	message := &model.ChatMessage{
		ID:               fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddUint64(&s.sequence, 1)),
		ConversationType: "single",
		FromUserID:       fromUserID,
		ToUserID:         toUserID,
		MessageType:      messageType,
		Content:          content,
		CreatedAt:        time.Now(),
	}
	s.trackRecentMessage(message)
	s.deliverToUser(ctx, toUserID, message)
	return message, nil
}

// 广播群解散事件
func (s *chatService) BroadcastGroupDissolved(ctx context.Context, groupID uint, userIDs []uint) {
	s.PushSystemEvent(ctx, userIDs, map[string]any{
		"type":     "group_dissolved",
		"group_id": groupID,
	})
}

// RecallMessage 通知接收方（或群成员）撤回指定消息
// 由于消息无服务端持久化，只做信令转发；客户端负责本地删除
func (s *chatService) RecallMessage(ctx context.Context, fromUserID, toUserID, groupID uint, messageID string) error {
	if s == nil {
		return ErrChatServiceUnavailable
	}
	if messageID == "" {
		return fmt.Errorf("message_id 不能为空")
	}
	if err := s.validateAndConsumeRecall(fromUserID, toUserID, groupID, messageID); err != nil {
		return err
	}
	payload := map[string]any{
		"type":       "message_recalled",
		"message_id": messageID,
		"from_user":  fromUserID,
	}
	if groupID > 0 {
		if s.groupRepo == nil {
			return ErrChatServiceUnavailable
		}
		if !s.groupRepo.IsMember(ctx, groupID, fromUserID) {
			return fmt.Errorf("不在该群聊中")
		}
		members, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
		if err != nil {
			return err
		}
		payload["group_id"] = groupID
		targets := make([]uint, 0, len(members))
		for _, m := range members {
			if m.UserID != fromUserID {
				targets = append(targets, m.UserID)
			}
		}
		s.PushSystemEvent(ctx, targets, payload)
	} else {
		if toUserID == 0 {
			return fmt.Errorf("to_user_id 不能为空")
		}
		if s.friendRepo == nil {
			return ErrChatServiceUnavailable
		}
		if !s.friendRepo.CheckFriendship(ctx, fromUserID, toUserID) {
			return fmt.Errorf("只能撤回发给好友的消息")
		}
		s.PushSystemEvent(ctx, []uint{toUserID}, payload)
	}
	return nil
}

// MarkRead 通知发送方消息已被接收方读取
func (s *chatService) MarkRead(ctx context.Context, readerID, peerID, groupID uint) error {
	if s == nil {
		return ErrChatServiceUnavailable
	}
	payload := map[string]any{
		"type":      "read_ack",
		"reader_id": readerID,
	}
	if groupID > 0 {
		if s.groupRepo == nil {
			return ErrChatServiceUnavailable
		}
		payload["group_id"] = groupID
		if !s.groupRepo.IsMember(ctx, groupID, readerID) {
			return nil // 静默忽略非成员的已读
		}
		members, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
		if err != nil {
			return nil
		}
		targets := make([]uint, 0, len(members))
		for _, m := range members {
			if m.UserID != readerID {
				targets = append(targets, m.UserID)
			}
		}
		s.PushSystemEvent(ctx, targets, payload)
	} else {
		if peerID == 0 {
			return nil
		}
		payload["peer_id"] = peerID
		s.PushSystemEvent(ctx, []uint{peerID}, payload)
	}
	return nil
}

// 推送系统事件
func (s *chatService) PushSystemEvent(ctx context.Context, userIDs []uint, payload any) []SystemPushResult {
	results := make([]SystemPushResult, 0, len(userIDs))
	for _, userID := range userIDs {
		s.mu.RLock()
		userConns := s.connections[userID]
		connections := make([]*chatConnection, 0, len(userConns))
		connectionIDs := make([]string, 0, len(userConns))
		for _, conn := range userConns {
			connections = append(connections, conn)
			connectionIDs = append(connectionIDs, conn.id)
		}
		s.mu.RUnlock()

		result := SystemPushResult{
			UserID:        userID,
			Online:        len(connections) > 0,
			ConnectionIDs: connectionIDs,
		}
		if len(connections) == 0 {
			s.enqueueOfflineSystemEvent(userID, payload)
			results = append(results, result)
			continue
		}

		for _, conn := range connections {
			if conn.sysDeliver == nil {
				result.FailedConnIDs = append(result.FailedConnIDs, conn.id)
				result.ErrorMessages = append(result.ErrorMessages, "system delivery unavailable")
				continue
			}
			err := conn.sysDeliver(payload)
			if err != nil {
				result.FailedConnIDs = append(result.FailedConnIDs, conn.id)
				result.ErrorMessages = append(result.ErrorMessages, err.Error())
				continue
			}
			result.SuccessfulConnIDs = append(result.SuccessfulConnIDs, conn.id)
			result.SuccessfulPushCount++
		}

		if len(result.FailedConnIDs) > 0 {
			s.mu.Lock()
			if currentConnections, ok := s.connections[userID]; ok {
				for _, failedID := range result.FailedConnIDs {
					delete(currentConnections, failedID)
				}
				if len(currentConnections) == 0 {
					delete(s.connections, userID)
				}
			}
			s.mu.Unlock()
		}
		if result.SuccessfulPushCount == 0 {
			s.enqueueOfflineSystemEvent(userID, payload)
		}
		results = append(results, result)
	}
	return results
}

// 获取用户的连接ID列表
func (s *chatService) GetConnectionIDs(userID uint) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return connectionIDsFromMap(s.connections[userID])
}

func (s *chatService) HasConnectionClient(userID uint, client string) bool {
	client = normalizeConnectionClient(client)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, connection := range s.connections[userID] {
		if connection.client == client {
			return true
		}
	}
	return false
}

// 踢掉指定用户的所有连接，先推送kicked消息再关闭
func (s *chatService) KickUserConnections(userID uint, reason string) {
	s.mu.Lock()
	userConns := s.connections[userID]
	conns := make([]*chatConnection, 0, len(userConns))
	for _, conn := range userConns {
		conns = append(conns, conn)
	}
	delete(s.connections, userID)
	delete(s.offline, userID)
	delete(s.systemOffline, userID)
	s.mu.Unlock()
	if len(conns) == 0 {
		return
	}
	logger.Info("kicking user connections",
		"user_id", userID,
		"connection_count", len(conns),
		"reason", reason,
	)
	kickPayload := map[string]any{
		"type":   "kicked",
		"reason": reason,
	}
	for _, conn := range conns {
		if conn.sysDeliver != nil {
			if err := conn.sysDeliver(kickPayload); err != nil {
				logger.Warn("failed to send kicked event", "user_id", userID, "connection_id", conn.id, "error", err)
			}
		}
		if conn.closeFn != nil {
			conn.closeFn()
		}
	}
}
