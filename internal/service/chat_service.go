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
)

type DeliveryFunc func(message *model.ChatMessage, offline bool) error
type SystemDeliveryFunc func(payload any) error

type SystemPushResult struct {
	UserID              uint
	Online              bool
	ConnectionIDs       []string
	SuccessfulConnIDs   []string
	FailedConnIDs       []string
	ErrorMessages       []string
	SuccessfulPushCount int
}

type ChatService interface {
	RegisterConnection(userID uint, deliver DeliveryFunc, sysDeliver SystemDeliveryFunc) string
	UnregisterConnection(userID uint, connectionID string)
	SendMessage(fromUserID, toUserID, groupID uint, messageType string, content string) (*model.ChatMessage, error)
	BroadcastGroupDissolved(groupID uint, userIDs []uint)
	PushSystemEvent(userIDs []uint, payload any) []SystemPushResult
	GetConnectionIDs(userID uint) []string
}

type chatConnection struct {
	id         string
	deliver    DeliveryFunc
	sysDeliver SystemDeliveryFunc
}

type queuedSystemEvent struct {
	id      string
	payload any
}

type chatService struct {
	friendRepo    repo.FriendRepository
	groupRepo     repo.GroupRepository
	mu            sync.RWMutex
	sequence      uint64
	connections   map[uint]map[string]*chatConnection
	offline       map[uint][]*model.ChatMessage
	systemOffline map[uint][]*queuedSystemEvent
}

func NewChatService(friendRepo repo.FriendRepository, groupRepo repo.GroupRepository) ChatService {
	return &chatService{
		friendRepo:    friendRepo,
		groupRepo:     groupRepo,
		connections:   make(map[uint]map[string]*chatConnection),
		offline:       make(map[uint][]*model.ChatMessage),
		systemOffline: make(map[uint][]*queuedSystemEvent),
	}
}

func (s *chatService) RegisterConnection(userID uint, deliver DeliveryFunc, sysDeliver SystemDeliveryFunc) string {
	connectionID := fmt.Sprintf("%d", atomic.AddUint64(&s.sequence, 1))
	s.mu.Lock()
	if s.connections[userID] == nil {
		s.connections[userID] = make(map[string]*chatConnection)
	}
	s.connections[userID][connectionID] = &chatConnection{
		id:         connectionID,
		deliver:    deliver,
		sysDeliver: sysDeliver,
	}
	pending := append([]*model.ChatMessage(nil), s.offline[userID]...)
	pendingSystem := append([]*queuedSystemEvent(nil), s.systemOffline[userID]...)
	connectionIDs := connectionIDsFromMap(s.connections[userID])
	s.mu.Unlock()
	logger.Info("websocket connection registered", "user_id", userID, "connection_ids", connectionIDs, "connection_count", len(connectionIDs))
	if len(pending) == 0 && len(pendingSystem) == 0 {
		return connectionID
	}
	delivered := make(map[string]struct{}, len(pending))
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
	if len(pendingSystem) == 0 || sysDeliver == nil {
		return connectionID
	}
	deliveredSystem := make(map[string]struct{}, len(pendingSystem))
	for _, event := range pendingSystem {
		if event == nil {
			continue
		}
		if err := sysDeliver(event.payload); err == nil {
			deliveredSystem[event.id] = struct{}{}
		}
	}
	if len(deliveredSystem) == 0 {
		return connectionID
	}
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
	return connectionID
}

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

func (s *chatService) SendMessage(fromUserID, toUserID, groupID uint, messageType string, content string) (*model.ChatMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrMessageEmpty
	}
	if messageType == "" {
		messageType = "text"
	}
	if groupID > 0 {
		return s.sendGroupMessage(fromUserID, groupID, messageType, content)
	}
	if !s.friendRepo.CheckFriendship(fromUserID, toUserID) {
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
	s.deliverToUser(toUserID, message)
	return message, nil
}

func (s *chatService) sendGroupMessage(fromUserID, groupID uint, messageType string, content string) (*model.ChatMessage, error) {
	if s.groupRepo == nil || !s.groupRepo.IsMember(groupID, fromUserID) {
		return nil, ErrGroupMessagePermission
	}

	members, err := s.groupRepo.GetMembersByGroupID(groupID)
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

	for _, member := range members {
		if member.UserID == fromUserID {
			continue
		}
		s.deliverToUser(member.UserID, message)
	}
	return message, nil
}

func (s *chatService) BroadcastGroupDissolved(groupID uint, userIDs []uint) {
	s.PushSystemEvent(userIDs, map[string]any{
		"type":     "group_dissolved",
		"group_id": groupID,
	})
}

func (s *chatService) PushSystemEvent(userIDs []uint, payload any) []SystemPushResult {
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

func (s *chatService) GetConnectionIDs(userID uint) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return connectionIDsFromMap(s.connections[userID])
}

func (s *chatService) enqueueOfflineMessage(userID uint, message *model.ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.offline[userID] = append(s.offline[userID], message)
}

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

func (s *chatService) deliverToUser(userID uint, message *model.ChatMessage) {
	s.pushRedisMessage(userID, message)

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

func (s *chatService) pushRedisMessage(userID uint, message *model.ChatMessage) {
	if redis.RedisClient == nil {
		return
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		return
	}
	pushKey := fmt.Sprintf("chat:push:%d", userID)
	redis.RedisClient.RPush(context.Background(), pushKey, msgBytes)
	redis.RedisClient.Expire(context.Background(), pushKey, 3*24*time.Hour)
}

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
