package model

import "gorm.io/gorm"

type Friend struct {
	gorm.Model
	UserID   uint `gorm:"index"`
	FriendID uint `gorm:"index"`
}

func (Friend) TableName() string {
	return "friend"
}
