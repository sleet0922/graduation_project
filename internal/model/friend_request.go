package model

import "gorm.io/gorm"

type FriendRequest struct {
	gorm.Model
	SenderID   uint `gorm:"index"`
	ReceiverID uint `gorm:"index"`
	Status     uint `gorm :"default:0;"`    //接受变1,拒绝变2
}

func (FriendRequest) TableName() string {
	return "friend_request"
}
