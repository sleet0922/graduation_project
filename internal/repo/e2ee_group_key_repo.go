package repo

import (
	"context"
	"errors"
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type E2EEGroupKeyRepository interface {
	GetCurrentVersion(ctx context.Context, groupID uint) (int, error)
	ExistsVersion(ctx context.Context, groupID uint, keyVersion int) (bool, error)
	GetCurrentUserKeyBox(ctx context.Context, groupID, userID uint) (*model.E2EEGroupKeyBox, error)
	GetUserKeyBoxByVersion(ctx context.Context, groupID uint, keyVersion int, userID uint) (*model.E2EEGroupKeyBox, error)
	CreateNextVersion(ctx context.Context, groupID, createdBy uint) (*model.E2EEGroupKey, error)
	GetVersionBoxes(ctx context.Context, groupID uint, keyVersion int) ([]*model.E2EEGroupKeyBox, error)
	ReplaceVersionBoxes(ctx context.Context, groupID uint, keyVersion int, boxes []*model.E2EEGroupKeyBox) error
}

type e2eeGroupKeyRepository struct {
	db *gorm.DB
}

func NewE2EEGroupKeyRepository(db *gorm.DB) E2EEGroupKeyRepository {
	return &e2eeGroupKeyRepository{db: db}
}

// 数据库 获取群当前密钥版本
func (r *e2eeGroupKeyRepository) GetCurrentVersion(ctx context.Context, groupID uint) (int, error) {
	var key model.E2EEGroupKey
	err := r.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("key_version desc").
		First(&key).Error
	if err != nil {
		return 0, err
	}
	return key.KeyVersion, nil
}

// 数据库 检查指定版本是否存在
func (r *e2eeGroupKeyRepository) ExistsVersion(ctx context.Context, groupID uint, keyVersion int) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.E2EEGroupKey{}).
		Where("group_id = ? AND key_version = ?", groupID, keyVersion).
		Count(&count).Error
	return count > 0, err
}

// 数据库 获取用户当前密钥盒
func (r *e2eeGroupKeyRepository) GetCurrentUserKeyBox(ctx context.Context, groupID, userID uint) (*model.E2EEGroupKeyBox, error) {
	var box model.E2EEGroupKeyBox
	subQuery := r.db.WithContext(ctx).
		Model(&model.E2EEGroupKey{}).
		Select("MAX(key_version)").
		Where("group_id = ?", groupID)
	err := r.db.WithContext(ctx).
		Where("group_id = ? AND key_version = (?) AND user_id = ?", groupID, subQuery, userID).
		First(&box).Error
	if err != nil {
		return nil, err
	}
	return &box, nil
}

// 数据库 获取用户指定版本密钥盒
func (r *e2eeGroupKeyRepository) GetUserKeyBoxByVersion(ctx context.Context, groupID uint, keyVersion int, userID uint) (*model.E2EEGroupKeyBox, error) {
	var box model.E2EEGroupKeyBox
	err := r.db.WithContext(ctx).
		Where("group_id = ? AND key_version = ? AND user_id = ?", groupID, keyVersion, userID).
		First(&box).Error
	if err != nil {
		return nil, err
	}
	return &box, nil
}

// 数据库 创建下一版本密钥和密钥盒
func (r *e2eeGroupKeyRepository) CreateNextVersion(ctx context.Context, groupID, createdBy uint) (*model.E2EEGroupKey, error) {
	var createdKey *model.E2EEGroupKey
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.E2EEGroupKey
		maxVersion := 0
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("group_id = ?", groupID).
			Order("key_version desc").
			First(&current).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			maxVersion = current.KeyVersion
		}

		nextVersion := maxVersion + 1
		groupKey := &model.E2EEGroupKey{
			GroupID:    groupID,
			KeyVersion: nextVersion,
			Algo:       "chacha20poly1305-v1",
			CreatedBy:  createdBy,
		}
		if err := tx.Create(groupKey).Error; err != nil {
			return err
		}
		createdKey = groupKey
		return nil
	})
	if err != nil {
		return nil, err
	}
	return createdKey, nil
}

// 数据库 获取指定版本的所有密钥盒子
func (r *e2eeGroupKeyRepository) GetVersionBoxes(ctx context.Context, groupID uint, keyVersion int) ([]*model.E2EEGroupKeyBox, error) {
	var boxes []*model.E2EEGroupKeyBox
	err := r.db.WithContext(ctx).
		Where("group_id = ? AND key_version = ?", groupID, keyVersion).
		Find(&boxes).Error
	if err != nil {
		return nil, err
	}
	return boxes, nil
}

// 数据库 替换指定版本的所有密钥盒
func (r *e2eeGroupKeyRepository) ReplaceVersionBoxes(ctx context.Context, groupID uint, keyVersion int, boxes []*model.E2EEGroupKeyBox) error {
	if len(boxes) == 0 {
		return errors.New("empty group key boxes")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var key model.E2EEGroupKey
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("group_id = ? AND key_version = ?", groupID, keyVersion).
			First(&key).Error; err != nil {
			return err
		}
		// 只删除本次上传的用户的旧盒子，保留其他用户的盒子（如 self-wrap）
		userIDs := make([]uint, 0, len(boxes))
		for _, box := range boxes {
			userIDs = append(userIDs, box.UserID)
		}
		if err := tx.Where("group_id = ? AND key_version = ? AND user_id IN ?", groupID, keyVersion, userIDs).
			Delete(&model.E2EEGroupKeyBox{}).Error; err != nil {
			return err
		}
		return tx.Create(&boxes).Error
	})
}
