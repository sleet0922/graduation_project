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
	ErrMessageEmpty      = errors.New("消息内容不能为空")
	ErrMessagePermission = errors.New("只能给好友发送消息")
)

type DeliveryFunc func(message *model.ChatMessage, offline bool) error

type ChatService interface {
	RegisterConnection(userID uint, deliver DeliveryFunc) string
	UnregisterConnection(userID uint, connectionID string)
	SendMessage(fromUserID, toUserID uint, messageType string, content string) (*model.ChatMessage, error)
	GetHistory(userID, friendID uint) ([]*model.ChatMessage, error)
	GetAllHistory(userID uint) ([]*model.ChatMessage, error)
	DeleteHistory(userID, friendID uint) error
	DeleteAllHistory(userID uint) error
}

type chatConnection struct {
	id      string
	deliver DeliveryFunc
}

type chatService struct {
	friendRepo  repo.FriendRepository
	chatRepo    repo.ChatRepository
	mu          sync.RWMutex
	sequence    uint64
	connections map[uint]map[string]*chatConnection
	offline     map[uint][]*model.ChatMessage
}

func NewChatService(friendRepo repo.FriendRepository, chatRepo repo.ChatRepository) ChatService {
	return &chatService{
		friendRepo:  friendRepo,
		chatRepo:    chatRepo,
		connections: make(map[uint]map[string]*chatConnection),
		offline:     make(map[uint][]*model.ChatMessage),
	}
}

func (s *chatService) RegisterConnection(userID uint, deliver DeliveryFunc) string {
	connectionID := fmt.Sprintf("%d", atomic.AddUint64(&s.sequence, 1))
	s.mu.Lock()
	if s.connections[userID] == nil {
		s.connections[userID] = make(map[string]*chatConnection)
	}
	s.connections[userID][connectionID] = &chatConnection{
		id:      connectionID,
		deliver: deliver,
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

func (s *chatService) SendMessage(fromUserID, toUserID uint, messageType string, content string) (*model.ChatMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrMessageEmpty
	}
	if messageType == "" {
		messageType = "text"
	}
	if !s.friendRepo.CheckFriendship(fromUserID, toUserID) {
		return nil, ErrMessagePermission
	}
	message := &model.ChatMessage{

		ID:          fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddUint64(&s.sequence, 1)),
		FromUserID:  fromUserID,
		ToUserID:    toUserID,
		MessageType: messageType,
		Content:     content,
		CreatedAt:   time.Now(),
	}

	s.mu.RLock()
	userConnections := s.connections[toUserID]

	connections := make([]*chatConnection, 0, len(userConnections))
	for _, connection := range userConnections {
		connections = append(connections, connection)
	}
	s.mu.RUnlock()

	if s.chatRepo != nil {
		s.chatRepo.Save(message)
	}

	if redis.RedisClient != nil {
		msgBytes, _ := json.Marshal(message)
		pushKey := fmt.Sprintf("chat:push:%d", toUserID)
		redis.RedisClient.RPush(context.Background(), pushKey, msgBytes)
		redis.RedisClient.Expire(context.Background(), pushKey, 3*24*time.Hour)
	}
	if len(connections) == 0 {
		s.enqueueOfflineMessage(toUserID, message)
		return message, nil
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
		if currentConnections, ok := s.connections[toUserID]; ok {
			for _, connectionID := range failedConnectionIDs {
				delete(currentConnections, connectionID)
			}
			if len(currentConnections) == 0 {
				delete(s.connections, toUserID)
			}
		}
		s.mu.Unlock()
	}
	if successCount == 0 {
		s.enqueueOfflineMessage(toUserID, message)
	}
	return message, nil
}

func (s *chatService) GetHistory(userID, friendID uint) ([]*model.ChatMessage, error) {
	if s.chatRepo == nil {
		return nil, nil
	}
	return s.chatRepo.GetHistory(userID, friendID)
}

func (s *chatService) GetAllHistory(userID uint) ([]*model.ChatMessage, error) {
	if s.chatRepo == nil {
		return nil, nil
	}
	return s.chatRepo.GetAllHistory(userID)
}

func (s *chatService) DeleteHistory(userID, friendID uint) error {
	if s.chatRepo == nil {
		return nil
	}
	return s.chatRepo.DeleteHistory(userID, friendID)
}

func (s *chatService) DeleteAllHistory(userID uint) error {
	if s.chatRepo == nil {
		return nil
	}
	return s.chatRepo.DeleteAllHistory(userID)
}

func (s *chatService) enqueueOfflineMessage(userID uint, message *model.ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.offline[userID] = append(s.offline[userID], message)
}
