package model

import "gorm.io/gorm"

type Friend struct {
	gorm.Model
	UserID   uint   `gorm:"uniqueIndex:idx_user_friend" json:"user_id"`
	FriendID uint   `gorm:"uniqueIndex:idx_user_friend" json:"friend_id"`
	Remark   string `json:"remark"`
}

func (Friend) TableName() string {
	return "friend"
}
