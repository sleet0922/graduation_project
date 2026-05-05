package model

import (
	"time"

	"gorm.io/gorm"
)

type ChatGroup struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	Name      string         `json:"name" gorm:"type:varchar(128);not null"`
	Avatar    string         `json:"avatar" gorm:"type:text"`
	OwnerID   uint           `json:"owner_id" gorm:"index;not null"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (ChatGroup) TableName() string {
	return "chat_group"
}

type ChatGroupMember struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	GroupID   uint           `json:"group_id" gorm:"uniqueIndex:idx_group_user;index;not null"`
	UserID    uint           `json:"user_id" gorm:"uniqueIndex:idx_group_user;index;not null"`
	InviterID uint           `json:"inviter_id" gorm:"index"`
	Role      string         `json:"role" gorm:"type:varchar(32);not null"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (ChatGroupMember) TableName() string {
	return "chat_group_member"
}

type ChatGroupDetail struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Avatar      string    `json:"avatar"`
	OwnerID     uint      `json:"owner_id"`
	MemberCount int64     `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ChatGroupMemberDetail struct {
	UserID  uint   `json:"user_id"`
	Account string `json:"account"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Avatar  string `json:"avatar"`
	Role    string `json:"role"`
}
