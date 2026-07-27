package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/pkg/logger"
	"sleet0922/graduation_project/pkg/redis"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrMessageEmpty           = errors.New("消息内容不能为空")
	ErrMessagePermission      = errors.New("只能给好友发送消息")
	ErrGroupMessagePermission = errors.New("群聊已解散")
	ErrRecallExpired          = errors.New("消息只能在发出后1分钟内撤回")
	ErrRecallPermission       = errors.New("只能撤回自己发送的消息")
)

const messageRecallWindow = time.Minute

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
	mu             sync.RWMutex
	sequence       uint64
	connections    map[uint]map[string]*chatConnection
	offline        map[uint][]*model.ChatMessage
	systemOffline  map[uint][]*queuedSystemEvent
	recentMessages map[string]recentChatMessage
}

func NewChatService(friendRepo repo.FriendRepository, groupRepo repo.GroupRepository) ChatService {
	return &chatService{
		friendRepo:     friendRepo,
		groupRepo:      groupRepo,
		connections:    make(map[uint]map[string]*chatConnection),
		offline:        make(map[uint][]*model.ChatMessage),
		systemOffline:  make(map[uint][]*queuedSystemEvent),
		recentMessages: make(map[string]recentChatMessage),
	}
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
func (s *chatService) pushRedisMessage(ctx context.Context, userID uint, message *model.ChatMessage) {
	if redis.RedisClient == nil {
		return
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		return
	}
	pushKey := fmt.Sprintf("chat:push:%d", userID)
	redis.RedisClient.RPush(ctx, pushKey, msgBytes)
	redis.RedisClient.Expire(ctx, pushKey, 3*24*time.Hour)
}

// 从redis拉取消息
func (s *chatService) drainRedisMessages(ctx context.Context, userID uint, deliveredIDs map[string]struct{}, deliver DeliveryFunc) {
	if redis.RedisClient == nil {
		return
	}
	pushKey := fmt.Sprintf("chat:push:%d", userID)
	rawList, err := redis.RedisClient.LRange(ctx, pushKey, 0, -1).Result()
	if err != nil || len(rawList) == 0 {
		return
	}
	anyDelivered := false
	for _, raw := range rawList {
		var message model.ChatMessage
		if err := json.Unmarshal([]byte(raw), &message); err != nil {
			continue
		}
		if _, alreadyDelivered := deliveredIDs[message.ID]; alreadyDelivered {
			continue
		}
		if err := deliver(&message, true); err != nil {
			continue
		}
		deliveredIDs[message.ID] = struct{}{}
		anyDelivered = true
	}
	if anyDelivered {
		redis.RedisClient.Del(ctx, pushKey)
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
	s.pushRedisMessage(ctx, userID, message)
	s.mu.RLock()
	userConnections := s.connections[userID]
	connections := make([]*chatConnection, 0, len(userConnections))
	for _, connection := range userConnections {
		connections = append(connections, connection)
	}
	s.mu.RUnlock()
	if len(connections) == 0 {
		s.enqueueOfflineMessage(userID, message)
		return
	}
	successCount := 0
	failedConnectionIDs := make([]string, 0)
	for _, connection := range connections {
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
		s.enqueueOfflineMessage(userID, message)
	}
}

// 发送群消息
func (s *chatService) sendGroupMessage(ctx context.Context, fromUserID, groupID uint, messageType string, content string) (*model.ChatMessage, error) {
	if s.groupRepo == nil || !s.groupRepo.IsMember(ctx, groupID, fromUserID) {
		return nil, ErrGroupMessagePermission
	}
	members, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
	if err != nil {
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
	if options.DrainOfflineMessages && len(pending) > 0 {
		for _, message := range pending {
			if err := deliver(message, true); err == nil {
				delivered[message.ID] = struct{}{}
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
		if !s.friendRepo.CheckFriendship(ctx, fromUserID, toUserID) {
			return fmt.Errorf("只能撤回发给好友的消息")
		}
		s.PushSystemEvent(ctx, []uint{toUserID}, payload)
	}
	return nil
}

// MarkRead 通知发送方消息已被接收方读取
func (s *chatService) MarkRead(ctx context.Context, readerID, peerID, groupID uint) error {
	payload := map[string]any{
		"type":      "read_ack",
		"reader_id": readerID,
	}
	if groupID > 0 {
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
			_ = conn.sysDeliver(kickPayload)
		}
		if conn.closeFn != nil {
			conn.closeFn()
		}
	}
}
