package repo

import (
	"context"
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FriendRepository interface {
	Create(ctx context.Context, friend *model.Friend) error
	Delete(ctx context.Context, friend *model.Friend) error
	GetByUserID(ctx context.Context, userID uint) ([]*model.Friend, error)
	GetFriendDetailsByUserID(ctx context.Context, userID uint) ([]*model.FriendDetail, error)
	CheckFriendship(ctx context.Context, userID uint, friendID uint) bool
	SendFriendRequest(ctx context.Context, friendRequest *model.FriendRequest) error
	CheckRequestExists(ctx context.Context, senderID, receiverID uint) (bool, error)
	GetRequestByID(ctx context.Context, requestID uint) (*model.FriendRequest, error)
	UpdateRequestStatus(ctx context.Context, request *model.FriendRequest) error
	GetRequestsByReceiverID(ctx context.Context, receiverID uint) ([]*model.FriendRequest, error)
	AcceptFriendRequest(ctx context.Context, request *model.FriendRequest) error
	RemoveBothFriends(ctx context.Context, userID, friendID uint) error
	UpdateRemark(ctx context.Context, userID, friendID uint, remark string) error
}

// ----------好友 repository 实现----------
type friendRepository struct {
	db *gorm.DB
}

// ----------好友 repository 构造函数----------
func NewFriendRepository(db *gorm.DB) FriendRepository {
	return &friendRepository{db: db}
}

// 数据库 创建好友关系
func (r *friendRepository) Create(ctx context.Context, friend *model.Friend) error {
	return r.db.WithContext(ctx).Create(friend).Error
}

// 数据库 删除好友关系
func (r *friendRepository) Delete(ctx context.Context, friend *model.Friend) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND friend_id = ?", friend.UserID, friend.FriendID).Delete(&model.Friend{}).Error
}

// 数据库 根据用户ID查询好友列表
func (r *friendRepository) GetByUserID(ctx context.Context, userID uint) ([]*model.Friend, error) {
	var friends []*model.Friend
	err := r.db.WithContext(ctx).Where("user_id = ? AND deleted_at IS NULL", userID).Find(&friends).Error
	return friends, err
}

// 数据库 根据用户ID查询好友详情列表
func (r *friendRepository) GetFriendDetailsByUserID(ctx context.Context, userID uint) ([]*model.FriendDetail, error) {
	var friendDetails []*model.FriendDetail
	err := r.db.WithContext(ctx).Table("friend").
		Select("friend.id, friend.user_id, friend.friend_id, friend.remark, \"user\".account, \"user\".name, \"user\".email, \"user\".avatar, \"user\".gender, \"user\".birthday, \"user\".location").
		Joins("LEFT JOIN \"user\" ON friend.friend_id = \"user\".id").
		Where("friend.user_id = ? AND friend.deleted_at IS NULL", userID).
		Find(&friendDetails).Error
	return friendDetails, err
}

// 数据库 检查好友关系
func (r *friendRepository) CheckFriendship(ctx context.Context, userID uint, friendID uint) bool {
	var friend model.Friend
	err := r.db.WithContext(ctx).Where(
		"((user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)) AND deleted_at IS NULL",
		userID, friendID, friendID, userID,
	).First(&friend).Error
	return err == nil
}

// 数据库 发送好友请求
func (r *friendRepository) SendFriendRequest(ctx context.Context, friendRequest *model.FriendRequest) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "sender_id"}, {Name: "receiver_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"status":     0,
			"deleted_at": nil,
		}),
	}).Create(friendRequest).Error
}

// 数据库 检查好友请求
func (r *friendRepository) CheckRequestExists(ctx context.Context, senderID, receiverID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Where(
		"status = 0 AND ((sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)) AND deleted_at IS NULL",
		senderID, receiverID, receiverID, senderID,
	).Model(&model.FriendRequest{}).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// 数据库 根据请求ID查询好友请求
func (r *friendRepository) GetRequestByID(ctx context.Context, requestID uint) (*model.FriendRequest, error) {
	var request model.FriendRequest
	err := r.db.WithContext(ctx).Where("deleted_at IS NULL").First(&request, requestID).Error
	return &request, err
}

// 数据库 更新好友请求状态
func (r *friendRepository) UpdateRequestStatus(ctx context.Context, request *model.FriendRequest) error {
	return r.db.WithContext(ctx).Save(request).Error
}

// 数据库 根据接收者ID查询好友请求列表
func (r *friendRepository) GetRequestsByReceiverID(ctx context.Context, receiverID uint) ([]*model.FriendRequest, error) {
	var requests []*model.FriendRequest
	err := r.db.WithContext(ctx).Where("receiver_id = ? AND deleted_at IS NULL", receiverID).Find(&requests).Error
	return requests, err
}

// 数据库 接受好友请求
func (r *friendRepository) AcceptFriendRequest(ctx context.Context, request *model.FriendRequest) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		request.Status = 1
		err := tx.Save(request).Error
		if err != nil {
			return err
		}
		err = tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "friend_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"deleted_at": nil,
				"remark":     "",
			}),
		}).Create(&model.Friend{UserID: request.SenderID, FriendID: request.ReceiverID}).Error
		if err != nil {
			return err
		}
		err = tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "friend_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"deleted_at": nil,
				"remark":     "",
			}),
		}).Create(&model.Friend{UserID: request.ReceiverID, FriendID: request.SenderID}).Error
		if err != nil {
			return err
		}
		return nil
	})
}

// 数据库 删除好友关系
func (r *friendRepository) RemoveBothFriends(ctx context.Context, userID, friendID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("user_id = ? AND friend_id = ?", userID, friendID).Delete(&model.Friend{}).Error
		if err != nil {
			return err
		}
		err = tx.Where("user_id = ? AND friend_id = ?", friendID, userID).Delete(&model.Friend{}).Error
		if err != nil {
			return err
		}
		return nil
	})
}

// 数据库 更新好友备注
func (r *friendRepository) UpdateRemark(ctx context.Context, userID, friendID uint, remark string) error {
	return r.db.WithContext(ctx).Model(&model.Friend{}).
		Where("user_id = ? AND friend_id = ?", userID, friendID).
		Update("remark", remark).Error
}
