// Package service 包含了应用程序的核心业务逻辑层实现。
// chat_service.go 提供了实时聊天相关的服务，包括连接管理、消息发送和离线消息处理等。
package service

import (
	"errors"
	"fmt"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
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
	// 注册一个新的用户连接。
	// 返回分配给该连接的唯一 connectionID。
	RegisterConnection(userID uint, deliver DeliveryFunc) string

	// 注销指定用户的连接。
	// 在用户断开连接时调用，用于清理连接资源。
	UnregisterConnection(userID uint, connectionID string)

	// 处理消息的发送逻辑。
	// 包括参数校验、权限校验（是否为好友）、构建消息实体，以及将消息推送给接收方的所有在线连接。
	// 如果接收方不在线，则将消息存入离线队列。
	SendMessage(fromUserID, toUserID uint, messageType string, content string) (*model.ChatMessage, error)
}

// 与客户端建立的聊天连接。
type chatConnection struct {
	id string
	// 回调函数，用于向该连接对应的客户端推送消息。
	deliver DeliveryFunc
}

// ChatService 接口的具体实现。
type chatService struct {
	// 查询用户间的好友关系。
	friendRepo repo.FriendRepository
	// mu 用于保护 connections 和 offline 字典的并发读写安全。
	mu sync.RWMutex
	// sequence 用于生成唯一的连接 ID 和消息 ID。
	sequence uint64
	// connections 记录所有在线用户的连接集合。
	// 外层 map 的 key 为 userID，内层 map 的 key 为 connectionID。
	connections map[uint]map[string]*chatConnection
	// offline 用于存储离线消息。
	// map 的 key 为接收方的 userID，value 为待接收的消息切片。
	offline map[uint][]*model.ChatMessage
}

// 初始化并返回一个 ChatService 实例。
func NewChatService(friendRepo repo.FriendRepository) ChatService {
	return &chatService{
		friendRepo:  friendRepo,
		connections: make(map[uint]map[string]*chatConnection),
		offline:     make(map[uint][]*model.ChatMessage),
	}
}

// RegisterConnection 将用户的连接注册到服务中，并在注册成功后尝试推送该用户的历史离线消息。
func (s *chatService) RegisterConnection(userID uint, deliver DeliveryFunc) string {
	// 原子递增生成唯一的连接 ID
	connectionID := fmt.Sprintf("%d", atomic.AddUint64(&s.sequence, 1))

	s.mu.Lock()
	// 如果该用户是第一次连接，初始化其连接集合
	if s.connections[userID] == nil {
		s.connections[userID] = make(map[string]*chatConnection)
	}
	// 将新的连接存入集合中
	s.connections[userID][connectionID] = &chatConnection{
		id:      connectionID,
		deliver: deliver,
	}
	// 获取该用户的离线消息列表，以备稍后投递
	pending := append([]*model.ChatMessage(nil), s.offline[userID]...)
	s.mu.Unlock()

	// 如果没有离线消息，直接返回连接 ID
	if len(pending) == 0 {
		return connectionID
	}

	// 记录成功投递的离线消息的 ID
	delivered := make(map[string]struct{}, len(pending))
	for _, message := range pending {
		// 调用回调函数投递消息，并将 offline 标志设为 true
		if err := deliver(message, true); err == nil {
			delivered[message.ID] = struct{}{}
		}
	}

	// 如果没有任何消息投递成功，说明发送失败或断开，直接返回
	if len(delivered) == 0 {
		return connectionID
	}

	s.mu.Lock()
	queue := s.offline[userID]
	remaining := make([]*model.ChatMessage, 0, len(queue))
	// 遍历当前的离线队列，移除已成功投递的消息
	for _, message := range queue {
		if _, ok := delivered[message.ID]; !ok {
			remaining = append(remaining, message)
		}
	}
	// 更新离线消息队列，若全部投递完毕则删除该用户的离线记录
	if len(remaining) == 0 {
		delete(s.offline, userID)
	} else {
		s.offline[userID] = remaining
	}
	s.mu.Unlock()

	return connectionID
}

// UnregisterConnection 将指定连接从服务中移除。
func (s *chatService) UnregisterConnection(userID uint, connectionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userConnections, ok := s.connections[userID]
	if !ok {
		return
	}

	// 从用户的连接集合中删除该连接
	delete(userConnections, connectionID)
	// 如果用户不再有任何连接，则清除该用户的 map 表项
	if len(userConnections) == 0 {
		delete(s.connections, userID)
	}
}

// SendMessage 发送消息给目标用户。
// 包含了参数检查、好友关系验证、消息实体生成及在线/离线投递逻辑。
func (s *chatService) SendMessage(fromUserID, toUserID uint, messageType string, content string) (*model.ChatMessage, error) {
	content = strings.TrimSpace(content)
	// 校验内容是否为空
	if content == "" {
		return nil, ErrMessageEmpty
	}
	// 默认消息类型为 text
	if messageType == "" {
		messageType = "text"
	}

	// 验证双方是否是好友关系，不是则拒绝发送
	if !s.friendRepo.CheckFriendship(fromUserID, toUserID) {
		return nil, ErrMessagePermission
	}

	// 构造消息实体
	message := &model.ChatMessage{
		// 生成基于时间戳和序列号的唯一消息 ID
		ID:          fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddUint64(&s.sequence, 1)),
		FromUserID:  fromUserID,
		ToUserID:    toUserID,
		MessageType: messageType,
		Content:     content,
		CreatedAt:   time.Now(),
	}

	s.mu.RLock()
	userConnections := s.connections[toUserID]
	// 收集目标用户当前的所有在线连接
	connections := make([]*chatConnection, 0, len(userConnections))
	for _, connection := range userConnections {
		connections = append(connections, connection)
	}
	s.mu.RUnlock()

	// 如果目标用户没有任何在线连接，将消息存入离线队列
	if len(connections) == 0 {
		s.enqueueOfflineMessage(toUserID, message)
		return message, nil
	}

	successCount := 0
	failedConnectionIDs := make([]string, 0)
	// 遍历所有连接尝试推送消息
	for _, connection := range connections {
		// deliver 调用，如果是普通新消息则 offline 参数为 false
		if err := connection.deliver(message, false); err != nil {
			// 如果推送失败（如连接已断开），记录下失败的连接 ID
			failedConnectionIDs = append(failedConnectionIDs, connection.id)
			continue
		}
		successCount++
	}

	// 如果有投递失败的连接，则清理这些无效连接
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

	// 如果消息未能成功投递到任何一个在线连接，视作用户不在线，将其转入离线队列
	if successCount == 0 {
		s.enqueueOfflineMessage(toUserID, message)
	}

	return message, nil
}

// enqueueOfflineMessage 将消息追加到目标用户的离线消息队列中。
func (s *chatService) enqueueOfflineMessage(userID uint, message *model.ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.offline[userID] = append(s.offline[userID], message)
}
