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
