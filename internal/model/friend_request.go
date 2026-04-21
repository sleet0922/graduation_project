package model

import "gorm.io/gorm"

type FriendRequest struct {
	gorm.Model
	SenderID   uint `gorm:"uniqueIndex:idx_sender_receiver" json:"sender_id"`
	ReceiverID uint `gorm:"uniqueIndex:idx_sender_receiver" json:"receiver_id"`
	Status     uint `gorm:"default:0" json:"status"` //接受变1,拒绝变2
}

func (FriendRequest) TableName() string {
	return "friend_request"
}
