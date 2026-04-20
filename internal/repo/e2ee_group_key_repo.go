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
	ReplaceVersionBoxes(ctx context.Context, groupID uint, keyVersion int, boxes []*model.E2EEGroupKeyBox) error
}

type e2eeGroupKeyRepository struct {
	db *gorm.DB
}

func NewE2EEGroupKeyRepository(db *gorm.DB) E2EEGroupKeyRepository {
	return &e2eeGroupKeyRepository{db: db}
}

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

func (r *e2eeGroupKeyRepository) ExistsVersion(ctx context.Context, groupID uint, keyVersion int) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.E2EEGroupKey{}).
		Where("group_id = ? AND key_version = ?", groupID, keyVersion).
		Count(&count).Error
	return count > 0, err
}

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
		if err := tx.Where("group_id = ? AND key_version = ?", groupID, keyVersion).
			Delete(&model.E2EEGroupKeyBox{}).Error; err != nil {
			return err
		}
		return tx.Create(&boxes).Error
	})
}
