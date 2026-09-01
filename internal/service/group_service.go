package service

import (
	"context"
	"errors"
	"fmt"
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
	ErrGroupMemberLimit    = errors.New("群成员数量已达上限")
)

const maxGroupMembers = 500 // 群组最大成员数

type GroupService interface {
	CreateGroup(ctx context.Context, ownerID uint, name, avatar string, memberIDs []uint) (*model.ChatGroupDetail, error)
	AddMembers(ctx context.Context, operatorID, groupID uint, memberIDs []uint) ([]*model.ChatGroupMemberDetail, error)
	RemoveMember(ctx context.Context, operatorID, groupID, memberID uint) error
	LeaveGroup(ctx context.Context, userID, groupID uint) error
	DeleteGroup(ctx context.Context, operatorID, groupID uint) error
	GetGroups(ctx context.Context, userID uint) ([]*model.ChatGroupDetail, error)
	GetMembers(ctx context.Context, userID, groupID uint) ([]*model.ChatGroupMemberDetail, error)
}

func (s *groupService) rotateGroupKey(ctx context.Context, groupID, actorID uint) (int, error) {
	if s.e2ee == nil {
		return 0, nil
	}
	if err := s.e2ee.RotateGroupKey(ctx, groupID, actorID); err != nil {
		return 0, err
	}
	return s.e2ee.GetGroupCurrentVersion(ctx, groupID)
}

func (s *groupService) notifyGroupKeyChanged(ctx context.Context, groupID uint, keyVersion int) {
	if s.chatService == nil || keyVersion <= 0 {
		return
	}
	members, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
	if err != nil || len(members) == 0 {
		return
	}
	userIDs := make([]uint, 0, len(members))
	for _, member := range members {
		userIDs = append(userIDs, member.UserID)
	}
	s.chatService.PushSystemEvent(ctx, userIDs, map[string]any{
		"type":        "e2ee_group_key_changed",
		"group_id":    groupID,
		"key_version": keyVersion,
	})
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
	// 检查成员数量上限（包括群主）
	if len(memberIDs)+1 > maxGroupMembers {
		return nil, ErrGroupMemberLimit
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
	unlockGroupState := lockE2EEGroupState(group.ID)
	defer unlockGroupState()
	keyVersion, err := s.rotateGroupKey(ctx, group.ID, ownerID)
	if err != nil {
		if rollbackErr := s.groupRepo.DeleteGroup(ctx, group.ID); rollbackErr != nil {
			return nil, fmt.Errorf("rotate initial group key: %w; rollback group: %v", err, rollbackErr)
		}
		return nil, err
	}
	if s.chatService != nil {
		s.chatService.PushSystemEvent(ctx, memberIDs, map[string]any{
			"type":        "group_member_added",
			"group_id":    group.ID,
			"operator_id": ownerID,
		})
	}
	s.notifyGroupKeyChanged(ctx, group.ID, keyVersion)
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
	unlockGroupState := lockE2EEGroupState(groupID)
	defer unlockGroupState()
	if _, err := s.groupRepo.GetByID(ctx, groupID); err != nil {
		return nil, ErrGroupNotFound
	}
	if !s.groupRepo.IsMember(ctx, groupID, operatorID) {
		return nil, ErrGroupPermission
	}
	existingMembers, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	existingIDs := make(map[uint]struct{}, len(existingMembers))
	for _, member := range existingMembers {
		existingIDs[member.UserID] = struct{}{}
	}
	newMemberIDs := make([]uint, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		if _, exists := existingIDs[memberID]; !exists {
			newMemberIDs = append(newMemberIDs, memberID)
		}
	}
	if len(newMemberIDs) == 0 {
		return s.GetMembers(ctx, operatorID, groupID)
	}
	// 检查添加后是否超过上限
	if len(existingMembers)+len(newMemberIDs) > maxGroupMembers {
		return nil, ErrGroupMemberLimit
	}
	err = s.validateInvitees(ctx, operatorID, newMemberIDs)
	if err != nil {
		return nil, err
	}
	members := make([]*model.ChatGroupMember, 0, len(newMemberIDs))
	for _, memberID := range newMemberIDs {
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
	keyVersion, err := s.rotateGroupKey(ctx, groupID, operatorID)
	if err != nil {
		var rollbackErr error
		for _, memberID := range newMemberIDs {
			if removeErr := s.groupRepo.RemoveMember(ctx, groupID, memberID); removeErr != nil {
				rollbackErr = errors.Join(rollbackErr, removeErr)
			}
		}
		if rollbackErr != nil {
			return nil, fmt.Errorf("rotate group key: %w; rollback members: %v", err, rollbackErr)
		}
		return nil, err
	}
	// 实时通知被拉入群的成员
	if s.chatService != nil {
		s.chatService.PushSystemEvent(ctx, newMemberIDs, map[string]any{
			"type":        "group_member_added",
			"group_id":    groupID,
			"operator_id": operatorID,
		})
	}
	s.notifyGroupKeyChanged(ctx, groupID, keyVersion)
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
	unlockGroupState := lockE2EEGroupState(groupID)
	defer unlockGroupState()
	currentGroup, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return ErrGroupNotFound
	}
	if currentGroup.OwnerID != operatorID {
		return ErrGroupKickDenied
	}
	if !s.groupRepo.IsMember(ctx, groupID, memberID) {
		return ErrGroupMemberNotFound
	}
	members, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
	if err != nil {
		return err
	}
	var removedMember *model.ChatGroupMember
	for _, member := range members {
		if member.UserID == memberID {
			copied := *member
			removedMember = &copied
			break
		}
	}
	if removedMember == nil {
		return ErrGroupMemberNotFound
	}
	if err := s.groupRepo.RemoveMember(ctx, groupID, memberID); err != nil {
		return err
	}
	keyVersion, err := s.rotateGroupKey(ctx, groupID, operatorID)
	if err != nil {
		if rollbackErr := s.groupRepo.AddMembers(ctx, groupID, []*model.ChatGroupMember{removedMember}); rollbackErr != nil {
			return fmt.Errorf("rotate group key: %w; restore member: %v", err, rollbackErr)
		}
		return err
	}
	// 通知被踢出的成员
	if s.chatService != nil {
		s.chatService.PushSystemEvent(ctx, []uint{memberID}, map[string]any{
			"type":        "group_member_removed",
			"group_id":    groupID,
			"operator_id": operatorID,
		})
	}
	s.notifyGroupKeyChanged(ctx, groupID, keyVersion)
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
	unlockGroupState := lockE2EEGroupState(groupID)
	defer unlockGroupState()
	if !s.groupRepo.IsMember(ctx, groupID, userID) {
		return ErrGroupPermission
	}
	members, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
	if err != nil {
		return err
	}
	var leavingMember *model.ChatGroupMember
	for _, member := range members {
		if member.UserID == userID {
			copied := *member
			leavingMember = &copied
			break
		}
	}
	if leavingMember == nil {
		return ErrGroupMemberNotFound
	}
	if err := s.groupRepo.RemoveMember(ctx, groupID, userID); err != nil {
		return err
	}
	keyVersion, err := s.rotateGroupKey(ctx, groupID, userID)
	if err != nil {
		if rollbackErr := s.groupRepo.AddMembers(ctx, groupID, []*model.ChatGroupMember{leavingMember}); rollbackErr != nil {
			return fmt.Errorf("rotate group key: %w; restore leaving member: %v", err, rollbackErr)
		}
		return err
	}
	// 通知群主：有人离开了群聊
	if s.chatService != nil && group.OwnerID != userID {
		s.chatService.PushSystemEvent(ctx, []uint{group.OwnerID}, map[string]any{
			"type":     "group_member_left",
			"group_id": groupID,
			"user_id":  userID,
		})
	}
	s.notifyGroupKeyChanged(ctx, groupID, keyVersion)
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
	unlockGroupState := lockE2EEGroupState(groupID)
	defer unlockGroupState()
	currentGroup, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return ErrGroupNotFound
	}
	if currentGroup.OwnerID != operatorID {
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
	groups, counts, err := s.groupRepo.GetGroupsByUserIDWithMemberCounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]*model.ChatGroupDetail, 0, len(groups))
	for _, group := range groups {
		result = append(result, &model.ChatGroupDetail{
			ID:          group.ID,
			Name:        group.Name,
			Avatar:      group.Avatar,
			OwnerID:     group.OwnerID,
			MemberCount: counts[group.ID],
			CreatedAt:   group.CreatedAt,
			UpdatedAt:   group.UpdatedAt,
		})
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
