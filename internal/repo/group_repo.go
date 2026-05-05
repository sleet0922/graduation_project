package repo

import (
	"context"
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GroupRepository interface {
	Create(ctx context.Context, group *model.ChatGroup, members []*model.ChatGroupMember) error
	AddMembers(ctx context.Context, groupID uint, members []*model.ChatGroupMember) error
	RemoveMember(ctx context.Context, groupID, userID uint) error
	DeleteGroup(ctx context.Context, groupID uint) error
	GetByID(ctx context.Context, groupID uint) (*model.ChatGroup, error)
	GetGroupsByUserID(ctx context.Context, userID uint) ([]*model.ChatGroup, error)
	GetMembersByGroupID(ctx context.Context, groupID uint) ([]*model.ChatGroupMember, error)
	CountMembers(ctx context.Context, groupID uint) (int64, error)
	IsMember(ctx context.Context, groupID, userID uint) bool
}

type groupRepository struct {
	db *gorm.DB
}

func NewGroupRepository(db *gorm.DB) GroupRepository {
	return &groupRepository{db: db}
}

// 数据库 创建群聊and添加成员
func (r *groupRepository) Create(ctx context.Context, group *model.ChatGroup, members []*model.ChatGroupMember) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Create(group).Error
		if err != nil {
			return err
		}
		if len(members) == 0 {
			return nil
		}
		for _, member := range members {
			member.GroupID = group.ID
		}
		return tx.Create(&members).Error
	})
}

// 数据库 添加群成员
func (r *groupRepository) AddMembers(ctx context.Context, groupID uint, members []*model.ChatGroupMember) error {
	if len(members) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "group_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"deleted_at": nil,
			"role":       "member",
			"inviter_id": gorm.Expr("EXCLUDED.inviter_id"),
		}),
	}).Create(&members).Error
}

// 数据库 删除群成员
func (r *groupRepository) RemoveMember(ctx context.Context, groupID, userID uint) error {
	return r.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).
		Delete(&model.ChatGroupMember{}).Error
}

// 数据库 删除群聊and群成员
func (r *groupRepository) DeleteGroup(ctx context.Context, groupID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("group_id = ?", groupID).Delete(&model.ChatGroupMember{}).Error
		if err != nil {
			return err
		}
		return tx.Delete(&model.ChatGroup{}, groupID).Error
	})
}

// 数据库 ID查询群聊
func (r *groupRepository) GetByID(ctx context.Context, groupID uint) (*model.ChatGroup, error) {
	var group model.ChatGroup
	err := r.db.WithContext(ctx).Where("deleted_at IS NULL").First(&group, groupID).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// 数据库 用户ID查询群聊列表
func (r *groupRepository) GetGroupsByUserID(ctx context.Context, userID uint) ([]*model.ChatGroup, error) {
	var groups []*model.ChatGroup
	err := r.db.WithContext(ctx).Model(&model.ChatGroup{}).
		Joins("JOIN chat_group_member ON chat_group_member.group_id = chat_group.id").
		Where("chat_group_member.user_id = ? AND chat_group_member.deleted_at IS NULL AND chat_group.deleted_at IS NULL", userID).
		Order("chat_group.updated_at desc").
		Find(&groups).Error
	return groups, err
}

// 数据库 群ID查询群成员列表
func (r *groupRepository) GetMembersByGroupID(ctx context.Context, groupID uint) ([]*model.ChatGroupMember, error) {
	var members []*model.ChatGroupMember
	err := r.db.WithContext(ctx).Where("group_id = ? AND deleted_at IS NULL", groupID).
		Order("created_at asc").
		Find(&members).Error
	return members, err
}

// 数据库 统计群成员数量
func (r *groupRepository) CountMembers(ctx context.Context, groupID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.ChatGroupMember{}).
		Where("group_id = ? AND deleted_at IS NULL", groupID).
		Count(&count).Error
	return count, err
}

// 数据库 检查用户是否是群成员
func (r *groupRepository) IsMember(ctx context.Context, groupID, userID uint) bool {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.ChatGroupMember{}).
		Where("group_id = ? AND user_id = ? AND deleted_at IS NULL", groupID, userID).
		Count(&count).Error
	return err == nil && count > 0
}
