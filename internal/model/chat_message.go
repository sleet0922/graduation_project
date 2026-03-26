package model

import "time"

type ChatMessage struct {
	ID          string    `json:"id"`
	FromUserID  uint      `json:"from_user_id"`
	ToUserID    uint      `json:"to_user_id"`
	MessageType string    `json:"message_type"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
}
