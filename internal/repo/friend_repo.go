package repo

import (
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
)

type FriendRepository interface {
	Create(friend *model.Friend) error
	Delete(friend *model.Friend) error
	GetByUserID(userID uint) ([]*model.Friend, error)
	GetFriendDetailsByUserID(userID uint) ([]*model.FriendDetail, error)
	CheckFriendship(userID uint, friendID uint) bool
	SendFriendRequest(friendRequest *model.FriendRequest) error
	CheckRequestExists(senderID, receiverID uint) (bool, error)
	GetRequestByID(requestID uint) (*model.FriendRequest, error)
	UpdateRequestStatus(request *model.FriendRequest) error
	GetRequestsByReceiverID(receiverID uint) ([]*model.FriendRequest, error)
	AcceptFriendRequest(request *model.FriendRequest) error
	RemoveBothFriends(userID, friendID uint) error
}

// ----------好友 repository 实现----------
type friendRepository struct {
	db *gorm.DB
}

// ----------好友 repository 构造函数----------
func NewFriendRepository(db *gorm.DB) FriendRepository {
	return &friendRepository{db: db}
}

// ----------好友 repository 方法----------
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

func (r *friendRepository) GetFriendDetailsByUserID(userID uint) ([]*model.FriendDetail, error) {
	var friendDetails []*model.FriendDetail
	err := r.db.Table("friend").
		Select("friend.id, friend.user_id, friend.friend_id, friend.remark, user.account, user.name, user.email, user.avatar, user.gender, user.birthday, user.location").
		Joins("LEFT JOIN user ON friend.friend_id = user.id").
		Where("friend.user_id = ?", userID).
		Find(&friendDetails).Error
	return friendDetails, err
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
	err := r.db.Where(
		"status = 0 AND ((sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?))",
		senderID, receiverID, receiverID, senderID,
	).Model(&model.FriendRequest{}).Count(&count).Error
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

func (r *friendRepository) AcceptFriendRequest(request *model.FriendRequest) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		request.Status = 1
		if err := tx.Save(request).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Friend{UserID: request.SenderID, FriendID: request.ReceiverID}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Friend{UserID: request.ReceiverID, FriendID: request.SenderID}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *friendRepository) RemoveBothFriends(userID, friendID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND friend_id = ?", userID, friendID).Delete(&model.Friend{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND friend_id = ?", friendID, userID).Delete(&model.Friend{}).Error; err != nil {
			return err
		}
		return nil
	})
}
