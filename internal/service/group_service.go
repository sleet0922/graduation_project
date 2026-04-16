package service

import (
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
	CreateGroup(ownerID uint, name, avatar string, memberIDs []uint) (*model.ChatGroupDetail, error)
	AddMembers(operatorID, groupID uint, memberIDs []uint) ([]*model.ChatGroupMemberDetail, error)
	RemoveMember(operatorID, groupID, memberID uint) error
	LeaveGroup(userID, groupID uint) error
	DeleteGroup(operatorID, groupID uint) error
	GetGroups(userID uint) ([]*model.ChatGroupDetail, error)
	GetMembers(userID, groupID uint) ([]*model.ChatGroupMemberDetail, error)
}

type groupService struct {
	groupRepo  repo.GroupRepository
	friendRepo repo.FriendRepository
	userRepo   repo.UserRepository
}

func NewGroupService(groupRepo repo.GroupRepository, friendRepo repo.FriendRepository, userRepo repo.UserRepository) GroupService {
	return &groupService{
		groupRepo:  groupRepo,
		friendRepo: friendRepo,
		userRepo:   userRepo,
	}
}

func (s *groupService) CreateGroup(ownerID uint, name, avatar string, memberIDs []uint) (*model.ChatGroupDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrGroupNameEmpty
	}

	memberIDs = normalizeMemberIDs(ownerID, memberIDs)
	if len(memberIDs) == 0 {
		return nil, ErrGroupMembersEmpty
	}

	err := s.validateInvitees(ownerID, memberIDs)
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

	err = s.groupRepo.Create(group, members)
	if err != nil {
		return nil, err
	}

	return s.buildGroupDetail(group)
}

func (s *groupService) AddMembers(operatorID, groupID uint, memberIDs []uint) ([]*model.ChatGroupMemberDetail, error) {
	if _, err := s.groupRepo.GetByID(groupID); err != nil {
		return nil, ErrGroupNotFound
	}
	if !s.groupRepo.IsMember(groupID, operatorID) {
		return nil, ErrGroupPermission
	}

	memberIDs = normalizeMemberIDs(operatorID, memberIDs)
	if len(memberIDs) == 0 {
		return nil, ErrGroupMembersEmpty
	}

	err := s.validateInvitees(operatorID, memberIDs)
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
	err = s.groupRepo.AddMembers(groupID, members)
	if err != nil {
		return nil, err
	}
	return s.GetMembers(operatorID, groupID)
}

func (s *groupService) RemoveMember(operatorID, groupID, memberID uint) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return ErrGroupNotFound
	}
	if group.OwnerID != operatorID {
		return ErrGroupKickDenied
	}
	if memberID == group.OwnerID {
		return ErrGroupOwnerProtected
	}
	if !s.groupRepo.IsMember(groupID, memberID) {
		return ErrGroupMemberNotFound
	}
	return s.groupRepo.RemoveMember(groupID, memberID)
}

func (s *groupService) LeaveGroup(userID, groupID uint) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return ErrGroupNotFound
	}
	if !s.groupRepo.IsMember(groupID, userID) {
		return ErrGroupPermission
	}
	if group.OwnerID == userID {
		return ErrGroupLeaveDenied
	}
	return s.groupRepo.RemoveMember(groupID, userID)
}

func (s *groupService) DeleteGroup(operatorID, groupID uint) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return ErrGroupNotFound
	}
	if group.OwnerID != operatorID {
		return ErrGroupDeleteDenied
	}
	return s.groupRepo.DeleteGroup(groupID)
}

func (s *groupService) GetGroups(userID uint) ([]*model.ChatGroupDetail, error) {
	groups, err := s.groupRepo.GetGroupsByUserID(userID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.ChatGroupDetail, 0, len(groups))
	for _, group := range groups {
		detail, err := s.buildGroupDetail(group)
		if err != nil {
			return nil, err
		}
		result = append(result, detail)
	}
	return result, nil
}

func (s *groupService) GetMembers(userID, groupID uint) ([]*model.ChatGroupMemberDetail, error) {
	if !s.groupRepo.IsMember(groupID, userID) {
		return nil, ErrGroupPermission
	}

	members, err := s.groupRepo.GetMembersByGroupID(groupID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.ChatGroupMemberDetail, 0, len(members))
	for _, member := range members {
		user, err := s.userRepo.GetByID(member.UserID)
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

func (s *groupService) validateInvitees(operatorID uint, memberIDs []uint) error {
	for _, memberID := range memberIDs {
		user, err := s.userRepo.GetByID(memberID)
		if err != nil || user == nil {
			return ErrGroupMemberNotFound
		}
		if operatorID != 0 && !s.friendRepo.CheckFriendship(operatorID, memberID) {
			return ErrGroupFriendOnly
		}
	}
	return nil
}

func (s *groupService) buildGroupDetail(group *model.ChatGroup) (*model.ChatGroupDetail, error) {
	count, err := s.groupRepo.CountMembers(group.ID)
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
