package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
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
type SystemDeliveryFunc func(msgType string, groupID uint) error

type ChatService interface {
	RegisterConnection(userID uint, deliver DeliveryFunc, sysDeliver SystemDeliveryFunc) string
	UnregisterConnection(userID uint, connectionID string)
	SendMessage(fromUserID, toUserID, groupID uint, messageType string, content string) (*model.ChatMessage, error)
	GetHistory(userID, friendID, groupID uint) ([]*model.ChatMessage, error)
	GetAllHistory(userID uint) ([]*model.ChatMessage, error)
	DeleteHistory(userID, friendID, groupID uint) error
	DeleteAllHistory(userID uint) error
	BroadcastGroupDissolved(groupID uint, userIDs []uint)
}

type chatConnection struct {
	id         string
	deliver    DeliveryFunc
	sysDeliver SystemDeliveryFunc
}

type chatService struct {
	friendRepo  repo.FriendRepository
	groupRepo   repo.GroupRepository
	chatRepo    repo.ChatRepository
	mu          sync.RWMutex
	sequence    uint64
	connections map[uint]map[string]*chatConnection
	offline     map[uint][]*model.ChatMessage
}

func NewChatService(friendRepo repo.FriendRepository, groupRepo repo.GroupRepository, chatRepo repo.ChatRepository) ChatService {
	return &chatService{
		friendRepo:  friendRepo,
		groupRepo:   groupRepo,
		chatRepo:    chatRepo,
		connections: make(map[uint]map[string]*chatConnection),
		offline:     make(map[uint][]*model.ChatMessage),
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
	s.mu.Unlock()
	if len(pending) == 0 {
		return connectionID
	}
	delivered := make(map[string]struct{}, len(pending))
	for _, message := range pending {

		if err := deliver(message, true); err == nil {
			delivered[message.ID] = struct{}{}
		}
	}
	if len(delivered) == 0 {
		return connectionID
	}
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
	return connectionID
}

func (s *chatService) UnregisterConnection(userID uint, connectionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	userConnections, ok := s.connections[userID]
	if !ok {
		return
	}
	delete(userConnections, connectionID)
	if len(userConnections) == 0 {
		delete(s.connections, userID)
	}
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
	if err := s.persistMessage(message); err != nil {
		return nil, err
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
	if err := s.persistMessage(message); err != nil {
		return nil, err
	}

	for _, member := range members {
		if member.UserID == fromUserID {
			continue
		}
		s.deliverToUser(member.UserID, message)
	}
	return message, nil
}

func (s *chatService) GetHistory(userID, friendID, groupID uint) ([]*model.ChatMessage, error) {
	if s.chatRepo == nil {
		return nil, nil
	}
	if groupID > 0 {
		if s.groupRepo == nil || !s.groupRepo.IsMember(groupID, userID) {
			return nil, ErrGroupMessagePermission
		}
		return s.chatRepo.GetGroupHistory(userID, groupID)
	}
	return s.chatRepo.GetHistory(userID, friendID)
}

func (s *chatService) GetAllHistory(userID uint) ([]*model.ChatMessage, error) {
	if s.chatRepo == nil {
		return nil, nil
	}
	return s.chatRepo.GetAllHistory(userID)
}

func (s *chatService) DeleteHistory(userID, friendID, groupID uint) error {
	if s.chatRepo == nil {
		return nil
	}
	if groupID > 0 {
		if s.groupRepo == nil || !s.groupRepo.IsMember(groupID, userID) {
			return ErrGroupMessagePermission
		}
		return s.chatRepo.DeleteGroupHistory(userID, groupID)
	}
	return s.chatRepo.DeleteHistory(userID, friendID)
}

func (s *chatService) DeleteAllHistory(userID uint) error {
	if s.chatRepo == nil {
		return nil
	}
	return s.chatRepo.DeleteAllHistory(userID)
}

func (s *chatService) BroadcastGroupDissolved(groupID uint, userIDs []uint) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, userID := range userIDs {
		userConns := s.connections[userID]
		for _, conn := range userConns {
			if conn.sysDeliver != nil {
				_ = conn.sysDeliver("group_dissolved", groupID)
			}
		}
	}
}

func (s *chatService) enqueueOfflineMessage(userID uint, message *model.ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.offline[userID] = append(s.offline[userID], message)
}

func (s *chatService) persistMessage(message *model.ChatMessage) error {
	if s.chatRepo != nil {
		if err := s.chatRepo.Save(message); err != nil {
			return err
		}
	}
	return nil
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
		if err := connection.deliver(message, false); err != nil {
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
