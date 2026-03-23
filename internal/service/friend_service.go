package service

import (
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
)

// ----------好友 service 接口----------
type FriendService interface {
	SendFriendRequest(senderID, receiverID uint) error
	HandleFriendRequest(requestID uint, status uint) error
	GetFriendRequestsByUserID(userID uint) ([]*model.FriendRequest, error)
	RemoveFriend(userID, friendID uint) error
	GetByUserID(userID uint) ([]*model.Friend, error)
	CheckFriendship(userID uint, friendID uint) bool
}

// ----------好友 service 实现----------
type friendService struct {
	friendRepo repo.FriendRepository
}

// ----------好友 service 构造函数----------
func NewFriendService(repo repo.FriendRepository) FriendService {
	return &friendService{friendRepo: repo}
}

// ----------好友 service 方法----------
func (s *friendService) SendFriendRequest(senderID, receiverID uint) error {
	exists, err := s.friendRepo.CheckRequestExists(senderID, receiverID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	friendRequest := &model.FriendRequest{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Status:     0,
	}
	return s.friendRepo.SendFriendRequest(friendRequest)
}

func (s *friendService) HandleFriendRequest(requestID uint, status uint) error {
	request, err := s.friendRepo.GetRequestByID(requestID)
	if err != nil {
		return err
	}
	if request.Status != 0 {
		return nil
	}
	if status == 1 {
		return s.friendRepo.AcceptFriendRequest(request)
	} else {
		request.Status = status
		return s.friendRepo.UpdateRequestStatus(request)
	}
}

func (s *friendService) GetFriendRequestsByUserID(userID uint) ([]*model.FriendRequest, error) {
	return s.friendRepo.GetRequestsByReceiverID(userID)
}

func (s *friendService) RemoveFriend(userID, friendID uint) error {
	return s.friendRepo.RemoveBothFriends(userID, friendID)
}

func (s *friendService) GetByUserID(userID uint) ([]*model.Friend, error) {
	return s.friendRepo.GetByUserID(userID)
}

func (s *friendService) CheckFriendship(userID uint, friendID uint) bool {
	return s.friendRepo.CheckFriendship(userID, friendID)
}
