package service

import (
	"context"
	"errors"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
	"strings"
)

var (
	ErrGroupNameEmpty      = errors.New("群名称不能为空")
	ErrGroupMembersEmpty   = errors.New("至少选择一位好友加入群聊")
	ErrGroupNotFound       = errors.New("群聊不存在")
	ErrGroupPermission     = errors.New("你不在该群聊中")
	ErrGroupDeleteDenied   = errors.New("只有群主可以删除群聊")
	ErrGroupKickDenied     = errors.New("只有群主可以踢人")
	ErrGroupLeaveDenied    = errors.New("群主不能直接退出群聊，请先解散群聊")
	ErrGroupMemberNotFound = errors.New("群成员不存在")
	ErrGroupFriendOnly     = errors.New("只能拉好友进群")
	ErrGroupOwnerProtected = errors.New("不能移除群主")
)

type GroupService interface {
	CreateGroup(ctx context.Context, ownerID uint, name, avatar string, memberIDs []uint) (*model.ChatGroupDetail, error)
	AddMembers(ctx context.Context, operatorID, groupID uint, memberIDs []uint) ([]*model.ChatGroupMemberDetail, error)
	RemoveMember(ctx context.Context, operatorID, groupID, memberID uint) error
	LeaveGroup(ctx context.Context, userID, groupID uint) error
	DeleteGroup(ctx context.Context, operatorID, groupID uint) error
	GetGroups(ctx context.Context, userID uint) ([]*model.ChatGroupDetail, error)
	GetMembers(ctx context.Context, userID, groupID uint) ([]*model.ChatGroupMemberDetail, error)
}

type groupService struct {
	groupRepo   repo.GroupRepository
	friendRepo  repo.FriendRepository
	userRepo    repo.UserRepository
	e2ee        E2EEService
	chatService ChatService
}

func NewGroupService(groupRepo repo.GroupRepository, friendRepo repo.FriendRepository, userRepo repo.UserRepository, e2ee E2EEService, chatService ChatService) GroupService {
	return &groupService{
		groupRepo:   groupRepo,
		friendRepo:  friendRepo,
		userRepo:    userRepo,
		e2ee:        e2ee,
		chatService: chatService,
	}
}

// ----------私有方法----------
// 去除无效成员ID、操作用户ID和重复ID
func normalizeMemberIDs(excludeID uint, memberIDs []uint) []uint {
	seen := make(map[uint]struct{}, len(memberIDs))
	result := make([]uint, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		if memberID == 0 || memberID == excludeID {
			continue
		}
		if _, ok := seen[memberID]; ok {
			continue
		}
		seen[memberID] = struct{}{}
		result = append(result, memberID)
	}
	return result
}

// 验证关系
func (s *groupService) validateInvitees(ctx context.Context, operatorID uint, memberIDs []uint) error {
	for _, memberID := range memberIDs {
		user, err := s.userRepo.GetByID(ctx, memberID)
		if err != nil || user == nil {
			return ErrGroupMemberNotFound
		}
		if operatorID != 0 && !s.friendRepo.CheckFriendship(ctx, operatorID, memberID) {
			return ErrGroupFriendOnly
		}
	}
	return nil
}

// 构建群详情
func (s *groupService) buildGroupDetail(ctx context.Context, group *model.ChatGroup) (*model.ChatGroupDetail, error) {
	count, err := s.groupRepo.CountMembers(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	return &model.ChatGroupDetail{
		ID:          group.ID,
		Name:        group.Name,
		Avatar:      group.Avatar,
		OwnerID:     group.OwnerID,
		MemberCount: count,
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
	}, nil
}

// ----------群组 service 方法----------
// 创建群聊
func (s *groupService) CreateGroup(ctx context.Context, ownerID uint, name, avatar string, memberIDs []uint) (*model.ChatGroupDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrGroupNameEmpty
	}
	memberIDs = normalizeMemberIDs(ownerID, memberIDs)
	if len(memberIDs) == 0 {
		return nil, ErrGroupMembersEmpty
	}

	err := s.validateInvitees(ctx, ownerID, memberIDs)
	if err != nil {
		return nil, err
	}
	group := &model.ChatGroup{
		Name:    name,
		Avatar:  strings.TrimSpace(avatar),
		OwnerID: ownerID,
	}
	members := make([]*model.ChatGroupMember, 0, len(memberIDs)+1)
	members = append(members, &model.ChatGroupMember{
		UserID: ownerID,
		Role:   "owner",
	})
	for _, memberID := range memberIDs {
		members = append(members, &model.ChatGroupMember{
			UserID:    memberID,
			InviterID: ownerID,
			Role:      "member",
		})
	}
	err = s.groupRepo.Create(ctx, group, members)
	if err != nil {
		return nil, err
	}
	if s.e2ee != nil {
		if err := s.e2ee.RotateGroupKey(ctx, group.ID, ownerID); err != nil {
			return nil, err
		}
	}
	return s.buildGroupDetail(ctx, group)
}

// 添加群成员
func (s *groupService) AddMembers(ctx context.Context, operatorID, groupID uint, memberIDs []uint) ([]*model.ChatGroupMemberDetail, error) {
	if _, err := s.groupRepo.GetByID(ctx, groupID); err != nil {
		return nil, ErrGroupNotFound
	}
	if !s.groupRepo.IsMember(ctx, groupID, operatorID) {
		return nil, ErrGroupPermission
	}
	memberIDs = normalizeMemberIDs(operatorID, memberIDs)
	if len(memberIDs) == 0 {
		return nil, ErrGroupMembersEmpty
	}
	err := s.validateInvitees(ctx, operatorID, memberIDs)
	if err != nil {
		return nil, err
	}
	members := make([]*model.ChatGroupMember, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		members = append(members, &model.ChatGroupMember{
			GroupID:   groupID,
			UserID:    memberID,
			InviterID: operatorID,
			Role:      "member",
		})
	}
	err = s.groupRepo.AddMembers(ctx, groupID, members)
	if err != nil {
		return nil, err
	}
	if s.e2ee != nil {
		if err := s.e2ee.RotateGroupKey(ctx, groupID, operatorID); err != nil {
			return nil, err
		}
	}
	return s.GetMembers(ctx, operatorID, groupID)
}

// 移除群成员
func (s *groupService) RemoveMember(ctx context.Context, operatorID, groupID, memberID uint) error {
	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return ErrGroupNotFound
	}
	if group.OwnerID != operatorID {
		return ErrGroupKickDenied
	}
	if memberID == group.OwnerID {
		return ErrGroupOwnerProtected
	}
	if !s.groupRepo.IsMember(ctx, groupID, memberID) {
		return ErrGroupMemberNotFound
	}
	if err := s.groupRepo.RemoveMember(ctx, groupID, memberID); err != nil {
		return err
	}
	if s.e2ee != nil {
		return s.e2ee.RotateGroupKey(ctx, groupID, operatorID)
	}
	return nil
}

// 退出群聊
func (s *groupService) LeaveGroup(ctx context.Context, userID, groupID uint) error {
	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return ErrGroupNotFound
	}
	if !s.groupRepo.IsMember(ctx, groupID, userID) {
		return ErrGroupPermission
	}
	if group.OwnerID == userID {
		return ErrGroupLeaveDenied
	}
	if err := s.groupRepo.RemoveMember(ctx, groupID, userID); err != nil {
		return err
	}
	if s.e2ee != nil {
		return s.e2ee.RotateGroupKey(ctx, groupID, userID)
	}
	return nil
}

// 删除群聊
func (s *groupService) DeleteGroup(ctx context.Context, operatorID, groupID uint) error {
	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return ErrGroupNotFound
	}
	if group.OwnerID != operatorID {
		return ErrGroupDeleteDenied
	}

	var memberIDs []uint
	if s.chatService != nil {
		members, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
		if err == nil && len(members) > 0 {
			memberIDs = make([]uint, 0, len(members))
			for _, m := range members {
				memberIDs = append(memberIDs, m.UserID)
			}
		}
	}

	err = s.groupRepo.DeleteGroup(ctx, groupID)
	if err != nil {
		return err
	}

	if s.chatService != nil && len(memberIDs) > 0 {
		s.chatService.BroadcastGroupDissolved(ctx, groupID, memberIDs)
	}
	return nil
}

// 获取用户的群聊列表
func (s *groupService) GetGroups(ctx context.Context, userID uint) ([]*model.ChatGroupDetail, error) {
	groups, err := s.groupRepo.GetGroupsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]*model.ChatGroupDetail, 0, len(groups))
	for _, group := range groups {
		detail, err := s.buildGroupDetail(ctx, group)
		if err != nil {
			return nil, err
		}
		result = append(result, detail)
	}
	return result, nil
}

// 获取群成员列表
func (s *groupService) GetMembers(ctx context.Context, userID, groupID uint) ([]*model.ChatGroupMemberDetail, error) {
	if !s.groupRepo.IsMember(ctx, groupID, userID) {
		return nil, ErrGroupPermission
	}
	members, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	result := make([]*model.ChatGroupMemberDetail, 0, len(members))
	for _, member := range members {
		user, err := s.userRepo.GetByID(ctx, member.UserID)
		if err != nil || user == nil {
			return nil, ErrGroupMemberNotFound
		}
		result = append(result, &model.ChatGroupMemberDetail{
			UserID:  user.ID,
			Account: user.Account,
			Name:    user.Name,
			Email:   user.Email,
			Avatar:  user.Avatar,
			Role:    member.Role,
		})
	}
	return result, nil
}
