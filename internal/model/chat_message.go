package model

import (
	"time"
)

type ChatMessage struct {
	ID               string    `json:"id"`
	ConversationType string    `json:"conversation_type"`
	FromUserID       uint      `json:"from_user_id"`
	ToUserID         uint      `json:"to_user_id"`
	GroupID          uint      `json:"group_id"`
	MessageType      string    `json:"message_type"`
	Content          string    `json:"content"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
