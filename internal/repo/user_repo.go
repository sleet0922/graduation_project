package repo

import (
	"context"
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
)

type UserRepository interface {
	Add(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id uint) error
	Update(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id uint) (*model.User, error)
	GetByAccount(ctx context.Context, account string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	UpdateAvatar(ctx context.Context, userID uint, avatar string) (*model.User, error)
	UpdateName(ctx context.Context, userID uint, name string) (*model.User, error)
	UpdatePassword(ctx context.Context, userID uint, password string) (*model.User, error)
	UpdateProfile(ctx context.Context, userID uint, gender int, birthday string, location string) (*model.User, error)
	GetSelf(ctx context.Context, userID uint) (*model.User, error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Add(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, id).Error
}

func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepository) GetByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByAccount(ctx context.Context, account string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("account = ? AND deleted_at IS NULL", account).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) UpdateAvatar(ctx context.Context, userID uint, avatar string) (*model.User, error) {
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("avatar", avatar).Error
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, userID)
}

func (r *userRepository) UpdateName(ctx context.Context, userID uint, name string) (*model.User, error) {
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("name", name).Error
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, userID)
}

func (r *userRepository) UpdatePassword(ctx context.Context, userID uint, password string) (*model.User, error) {
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("password", password).Error
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, userID)
}

func (r *userRepository) UpdateProfile(ctx context.Context, userID uint, gender int, birthday string, location string) (*model.User, error) {
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"gender":   gender,
		"birthday": birthday,
		"location": location,
	}).Error
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, userID)
}

func (r *userRepository) GetSelf(ctx context.Context, userID uint) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", userID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
