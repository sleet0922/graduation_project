package repo

import (
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GroupRepository interface {
	Create(group *model.ChatGroup, members []*model.ChatGroupMember) error
	AddMembers(groupID uint, members []*model.ChatGroupMember) error
	RemoveMember(groupID, userID uint) error
	DeleteGroup(groupID uint) error
	GetByID(groupID uint) (*model.ChatGroup, error)
	GetGroupsByUserID(userID uint) ([]*model.ChatGroup, error)
	GetMembersByGroupID(groupID uint) ([]*model.ChatGroupMember, error)
	CountMembers(groupID uint) (int64, error)
	IsMember(groupID, userID uint) bool
}

type groupRepository struct {
	db *gorm.DB
}

func NewGroupRepository(db *gorm.DB) GroupRepository {
	return &groupRepository{db: db}
}

func (r *groupRepository) Create(group *model.ChatGroup, members []*model.ChatGroupMember) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
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

func (r *groupRepository) AddMembers(groupID uint, members []*model.ChatGroupMember) error {
	if len(members) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "group_id"}, {Name: "user_id"}},
		DoNothing: true,
	}).Create(&members).Error
}

func (r *groupRepository) RemoveMember(groupID, userID uint) error {
	return r.db.Where("group_id = ? AND user_id = ?", groupID, userID).
		Delete(&model.ChatGroupMember{}).Error
}

func (r *groupRepository) DeleteGroup(groupID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupID).Delete(&model.ChatGroupMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.ChatGroup{}, groupID).Error
	})
}

func (r *groupRepository) GetByID(groupID uint) (*model.ChatGroup, error) {
	var group model.ChatGroup
	if err := r.db.First(&group, groupID).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *groupRepository) GetGroupsByUserID(userID uint) ([]*model.ChatGroup, error) {
	var groups []*model.ChatGroup
	err := r.db.Model(&model.ChatGroup{}).
		Joins("JOIN chat_group_member ON chat_group_member.group_id = chat_group.id").
		Where("chat_group_member.user_id = ? AND chat_group_member.deleted_at IS NULL", userID).
		Order("chat_group.updated_at desc").
		Find(&groups).Error
	return groups, err
}

func (r *groupRepository) GetMembersByGroupID(groupID uint) ([]*model.ChatGroupMember, error) {
	var members []*model.ChatGroupMember
	err := r.db.Where("group_id = ?", groupID).
		Order("created_at asc").
		Find(&members).Error
	return members, err
}

func (r *groupRepository) CountMembers(groupID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.ChatGroupMember{}).
		Where("group_id = ?", groupID).
		Count(&count).Error
	return count, err
}

func (r *groupRepository) IsMember(groupID, userID uint) bool {
	var count int64
	err := r.db.Model(&model.ChatGroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Count(&count).Error
	return err == nil && count > 0
}
