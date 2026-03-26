package service

import (
	"errors"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
)

var (
	ErrCannotAddSelf = errors.New("不能添加自己为好友")
	ErrAlreadyFriend = errors.New("你们已经是好友了")
	ErrRequestExists = errors.New("好友申请已存在")
)

// ----------好友 service 接口----------
type FriendService interface {
	SendFriendRequest(senderID, receiverID uint) error
	HandleFriendRequest(requestID uint, status uint) error
	GetFriendRequestsByUserID(userID uint) ([]*model.FriendRequest, error)
	RemoveFriend(userID, friendID uint) error
	GetByUserID(userID uint) ([]*model.Friend, error)
	GetFriendDetailsByUserID(userID uint) ([]*model.FriendDetail, error)
	CheckFriendship(userID uint, friendID uint) bool
	UpdateRemark(userID, friendID uint, remark string) error
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
	if senderID == receiverID {
		return ErrCannotAddSelf
	}

	if s.friendRepo.CheckFriendship(senderID, receiverID) {
		return ErrAlreadyFriend
	}

	exists, err := s.friendRepo.CheckRequestExists(senderID, receiverID)
	if err != nil {
		return err
	}
	if exists {
		return ErrRequestExists
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

func (s *friendService) GetFriendDetailsByUserID(userID uint) ([]*model.FriendDetail, error) {
	return s.friendRepo.GetFriendDetailsByUserID(userID)
}

func (s *friendService) CheckFriendship(userID uint, friendID uint) bool {
	return s.friendRepo.CheckFriendship(userID, friendID)
}

func (s *friendService) UpdateRemark(userID, friendID uint, remark string) error {
	return s.friendRepo.UpdateRemark(userID, friendID, remark)
}
