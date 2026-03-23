package service

import (
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
)

type FriendService interface {
	AddFriend(userID, friendID uint) error
	RemoveFriend(userID, friendID uint) error
	GetByUserID(userID uint) ([]*model.Friend, error)
	CheckFriendship(userID uint, friendID uint) bool
}

type friendService struct {
	friendRepo repo.FriendRepository
}

func NewFriendService(repo repo.FriendRepository) FriendService {
	return &friendService{friendRepo: repo}
}

func (s *friendService) AddFriend(userID, friendID uint) error {
	friend := &model.Friend{
		UserID:   userID,
		FriendID: friendID,
	}
	return s.friendRepo.Create(friend)
}

func (s *friendService) RemoveFriend(userID, friendID uint) error {
	friend := &model.Friend{
		UserID:   userID,
		FriendID: friendID,
	}
	return s.friendRepo.Delete(friend)
}

func (s *friendService) GetByUserID(userID uint) ([]*model.Friend, error) {
	return s.friendRepo.GetByUserID(userID)
}
func (s *friendService) CheckFriendship(userID uint, friendID uint) bool {
	return s.friendRepo.CheckFriendship(userID, friendID)
}
