package model

import "gorm.io/gorm"

type Friend struct {
	gorm.Model
	UserID   uint `gorm:"uniqueIndex:idx_user_friend"`
	FriendID uint `gorm:"uniqueIndex:idx_user_friend"`
	Remark   string
}

func (Friend) TableName() string {
	return "friend"
}
