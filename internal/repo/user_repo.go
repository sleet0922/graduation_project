package repo

import (
	"context"
	"fmt"
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	UpsertLocation(ctx context.Context, location *model.UserLocation) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// 数据库 创建用户
func (r *userRepository) Add(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// 数据库 删除用户（含级联清理：好友/申请/群成员/E2EE密钥/位置/动态数据）
func (r *userRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 找出该用户发布的所有帖子 IDs
		var postIDs []uint
		if err := tx.Model(&model.FeedPost{}).Unscoped().
			Where("user_id = ?", id).
			Pluck("id", &postIDs).Error; err != nil {
			return fmt.Errorf("list user posts before deletion: %w", err)
		}

		if len(postIDs) > 0 {
			// 删除这些帖子的媒体附件、点赞、评论
			if err := tx.Unscoped().Where("post_id IN ?", postIDs).Delete(&model.FeedMedia{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Where("post_id IN ?", postIDs).Delete(&model.FeedLike{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Where("post_id IN ?", postIDs).Delete(&model.FeedComment{}).Error; err != nil {
				return err
			}
		}
		// 删除用户发布的帖子
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.FeedPost{}).Error; err != nil {
			return err
		}
		// 删除该用户在他人帖子上的点赞和评论
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.FeedLike{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.FeedComment{}).Error; err != nil {
			return err
		}
		// 删除好友关系（双向）
		if err := tx.Unscoped().Where("user_id = ? OR friend_id = ?", id, id).Delete(&model.Friend{}).Error; err != nil {
			return err
		}
		// 删除好友请求（作为发起方或接收方）
		if err := tx.Unscoped().Where("sender_id = ? OR receiver_id = ?", id, id).Delete(&model.FriendRequest{}).Error; err != nil {
			return err
		}
		// 删除群成员记录
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.ChatGroupMember{}).Error; err != nil {
			return err
		}
		// 删除 E2EE 公钥
		if err := tx.Where("user_id = ?", id).Delete(&model.E2EEUserPublicKey{}).Error; err != nil {
			return err
		}
		// 删除 E2EE 群密钥盒
		if err := tx.Where("user_id = ?", id).Delete(&model.E2EEGroupKeyBox{}).Error; err != nil {
			return err
		}
		// 删除位置记录
		if err := tx.Where("user_id = ?", id).Delete(&model.UserLocation{}).Error; err != nil {
			return err
		}
		// 最后软删除用户本体
		return tx.Delete(&model.User{}, id).Error
	})
}

// 数据库 更新用户信息
func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// 数据库 ID查询用户
func (r *userRepository) GetByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// 数据库 账号查询用户
func (r *userRepository) GetByAccount(ctx context.Context, account string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("account = ? AND deleted_at IS NULL", account).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// 数据库 邮箱查询用户
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// 数据库 更新头像
func (r *userRepository) UpdateAvatar(ctx context.Context, userID uint, avatar string) (*model.User, error) {
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("avatar", avatar).Error
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, userID)
}

// 数据库 更新昵称
func (r *userRepository) UpdateName(ctx context.Context, userID uint, name string) (*model.User, error) {
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("name", name).Error
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, userID)
}

// 数据库 更新密码
func (r *userRepository) UpdatePassword(ctx context.Context, userID uint, password string) (*model.User, error) {
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("password", password).Error
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, userID)
}

// 数据库 更新个人资料
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

// 数据库 获取用户自己的信息
func (r *userRepository) GetSelf(ctx context.Context, userID uint) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", userID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// 数据库 更新或插入用户位置
func (r *userRepository) UpsertLocation(ctx context.Context, location *model.UserLocation) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"latitude", "longitude", "province", "city", "district", "address", "timestamp"}),
	}).Create(location).Error
}
