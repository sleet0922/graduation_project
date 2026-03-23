package repo

import (
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
)

type FriendRepository interface {
	Create(friend *model.Friend) error
	Delete(friend *model.Friend) error
	GetByUserID(userID uint) ([]*model.Friend, error)
	CheckFriendship(userID uint, friendID uint) bool
	SendFriendRequest(friendRequest *model.FriendRequest) error
	CheckRequestExists(senderID, receiverID uint) (bool, error)
	GetRequestByID(requestID uint) (*model.FriendRequest, error)
	UpdateRequestStatus(request *model.FriendRequest) error
	GetRequestsByReceiverID(receiverID uint) ([]*model.FriendRequest, error)
}

type friendRepository struct {
	db *gorm.DB
}

func NewFriendRepository(db *gorm.DB) FriendRepository {
	return &friendRepository{db: db}
}

func (r *friendRepository) Create(friend *model.Friend) error {
	return r.db.Create(friend).Error
}

func (r *friendRepository) Delete(friend *model.Friend) error {
	return r.db.Where("user_id = ? AND friend_id = ?", friend.UserID, friend.FriendID).Delete(&model.Friend{}).Error
}

func (r *friendRepository) GetByUserID(userID uint) ([]*model.Friend, error) {
	var friends []*model.Friend
	err := r.db.Where("user_id = ?", userID).Find(&friends).Error
	return friends, err
}

func (r *friendRepository) CheckFriendship(userID uint, friendID uint) bool {
	var friend model.Friend
	err := r.db.Where("user_id = ? AND friend_id = ?", userID, friendID).First(&friend).Error
	return err == nil
}

func (r *friendRepository) SendFriendRequest(friendRequest *model.FriendRequest) error {
	return r.db.Create(friendRequest).Error
}

func (r *friendRepository) CheckRequestExists(senderID, receiverID uint) (bool, error) {
	var count int64
	err := r.db.Where("sender_id = ? AND receiver_id = ? AND status = 0", senderID, receiverID).Model(&model.FriendRequest{}).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *friendRepository) GetRequestByID(requestID uint) (*model.FriendRequest, error) {
	var request model.FriendRequest
	err := r.db.First(&request, requestID).Error
	return &request, err
}

func (r *friendRepository) UpdateRequestStatus(request *model.FriendRequest) error {
	return r.db.Save(request).Error
}

func (r *friendRepository) GetRequestsByReceiverID(receiverID uint) ([]*model.FriendRequest, error) {
	var requests []*model.FriendRequest
	err := r.db.Where("receiver_id = ?", receiverID).Find(&requests).Error
	return requests, err
}
