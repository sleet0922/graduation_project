package service

import (
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
)

type FriendService interface {
	SendFriendRequest(senderID, receiverID uint) error
	HandleFriendRequest(requestID uint, status uint) error
	GetFriendRequestsByUserID(userID uint) ([]*model.FriendRequest, error)
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

func (s *friendService) SendFriendRequest(senderID, receiverID uint) error {
	exists, err := s.friendRepo.CheckRequestExists(senderID, receiverID)
	if err != nil && err.Error() != "record not found" {
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
	request.Status = status
	err = s.friendRepo.UpdateRequestStatus(request)
	if err != nil {
		return err
	}
	if status == 1 {
		err = s.friendRepo.Create(&model.Friend{
			UserID:   request.SenderID,
			FriendID: request.ReceiverID,
		})
		if err != nil {
			return err
		}
		err = s.friendRepo.Create(&model.Friend{
			UserID:   request.ReceiverID,
			FriendID: request.SenderID,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *friendService) GetFriendRequestsByUserID(userID uint) ([]*model.FriendRequest, error) {
	return s.friendRepo.GetRequestsByReceiverID(userID)
}

func (s *friendService) RemoveFriend(userID, friendID uint) error {
	err := s.friendRepo.Delete(&model.Friend{
		UserID:   userID,
		FriendID: friendID,
	})
	if err != nil {
		return err
	}
	return s.friendRepo.Delete(&model.Friend{
		UserID:   friendID,
		FriendID: userID,
	})
}

func (s *friendService) GetByUserID(userID uint) ([]*model.Friend, error) {
	return s.friendRepo.GetByUserID(userID)
}

func (s *friendService) CheckFriendship(userID uint, friendID uint) bool {
	return s.friendRepo.CheckFriendship(userID, friendID)
}

