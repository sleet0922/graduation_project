package service

import (
	"context"
	"errors"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
)

var (
	ErrCannotAddSelf              = errors.New("不能添加自己为好友")
	ErrAlreadyFriend              = errors.New("你们已经是好友了")
	ErrRequestExists              = errors.New("好友申请已存在")
	ErrFriendRequestPermission    = errors.New("无权处理该好友申请")
	ErrInvalidFriendRequestStatus = errors.New("无效的好友申请处理状态")
)

// ----------好友 service 接口----------
type FriendService interface {
	SendFriendRequest(ctx context.Context, senderID, receiverID uint) error
	HandleFriendRequest(ctx context.Context, userID, requestID uint, status uint) error
	GetFriendRequestsByUserID(ctx context.Context, userID uint) ([]*model.FriendRequest, error)
	RemoveFriend(ctx context.Context, userID, friendID uint) error
	GetByUserID(ctx context.Context, userID uint) ([]*model.Friend, error)
	GetFriendDetailsByUserID(ctx context.Context, userID uint) ([]*model.FriendDetail, error)
	CheckFriendship(ctx context.Context, userID uint, friendID uint) bool
	UpdateRemark(ctx context.Context, userID, friendID uint, remark string) error
}

// ----------好友 service 实现----------
type friendService struct {
	friendRepo repo.FriendRepository
}

// ----------好友 service 构造函数----------
func NewFriendService(repo repo.FriendRepository) FriendService {
	return &friendService{friendRepo: repo}
}

// 发送好友请求
func (s *friendService) SendFriendRequest(ctx context.Context, senderID, receiverID uint) error {
	if senderID == receiverID {
		return ErrCannotAddSelf
	}
	if s.friendRepo.CheckFriendship(ctx, senderID, receiverID) {
		return ErrAlreadyFriend
	}
	exists, err := s.friendRepo.CheckRequestExists(ctx, senderID, receiverID)
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
	return s.friendRepo.SendFriendRequest(ctx, friendRequest)
}

// 处理好友请求
func (s *friendService) HandleFriendRequest(ctx context.Context, userID, requestID uint, status uint) error {
	if status != 1 && status != 2 {
		return ErrInvalidFriendRequestStatus
	}

	request, err := s.friendRepo.GetRequestByID(ctx, requestID)
	if err != nil {
		return err
	}
	if request.ReceiverID != userID {
		return ErrFriendRequestPermission
	}
	if request.Status != 0 {
		return nil
	}
	if status == 1 {
		return s.friendRepo.AcceptFriendRequest(ctx, request)
	} else {
		request.Status = status
		return s.friendRepo.UpdateRequestStatus(ctx, request)
	}
}

// 获取用户的好友请求列表
func (s *friendService) GetFriendRequestsByUserID(ctx context.Context, userID uint) ([]*model.FriendRequest, error) {
	return s.friendRepo.GetRequestsByReceiverID(ctx, userID)
}

// 删除好友
func (s *friendService) RemoveFriend(ctx context.Context, userID, friendID uint) error {
	return s.friendRepo.RemoveBothFriends(ctx, userID, friendID)
}

// 根据用户ID获取好友列表
func (s *friendService) GetByUserID(ctx context.Context, userID uint) ([]*model.Friend, error) {
	return s.friendRepo.GetByUserID(ctx, userID)
}

// 根据用户ID获取好友详情列表
func (s *friendService) GetFriendDetailsByUserID(ctx context.Context, userID uint) ([]*model.FriendDetail, error) {
	return s.friendRepo.GetFriendDetailsByUserID(ctx, userID)
}

// 检查两人好友关系
func (s *friendService) CheckFriendship(ctx context.Context, userID uint, friendID uint) bool {
	return s.friendRepo.CheckFriendship(ctx, userID, friendID)
}

// 更新好友备注
func (s *friendService) UpdateRemark(ctx context.Context, userID, friendID uint, remark string) error {
	return s.friendRepo.UpdateRemark(ctx, userID, friendID, remark)
}
