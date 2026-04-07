package model

import (
	"time"

	"gorm.io/gorm"
)

type ChatMessage struct {
	ID               string         `json:"id" gorm:"primarykey;type:varchar(64)"`
	ConversationType string         `json:"conversation_type" gorm:"type:varchar(16);index"`
	FromUserID       uint           `json:"from_user_id" gorm:"index"`
	ToUserID         uint           `json:"to_user_id" gorm:"index"`
	GroupID          uint           `json:"group_id" gorm:"index"`
	MessageType      string         `json:"message_type" gorm:"type:varchar(32)"`
	Content          string         `json:"content" gorm:"type:text"`
	SenderDeleted    bool           `json:"-" gorm:"default:false"`
	ReceiverDeleted  bool           `json:"-" gorm:"default:false"`
	DeletedBy        string         `json:"-" gorm:"type:text"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}
