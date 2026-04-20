package repo

import (
	"context"
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type E2EEKeyRepository interface {
	Upsert(ctx context.Context, key *model.E2EEUserPublicKey) error
	GetByUserID(ctx context.Context, userID uint) (*model.E2EEUserPublicKey, error)
}

type e2eeKeyRepository struct {
	db *gorm.DB
}

func NewE2EEKeyRepository(db *gorm.DB) E2EEKeyRepository {
	return &e2eeKeyRepository{db: db}
}

func (r *e2eeKeyRepository) Upsert(ctx context.Context, key *model.E2EEUserPublicKey) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"key_type", "public_key", "updated_at"}),
		}).
		Create(key).Error
}

func (r *e2eeKeyRepository) GetByUserID(ctx context.Context, userID uint) (*model.E2EEUserPublicKey, error) {
	var key model.E2EEUserPublicKey
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&key).Error
	if err != nil {
		return nil, err
	}
	return &key, nil
}
