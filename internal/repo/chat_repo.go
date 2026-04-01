package repo

import (
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
)

type ChatRepository interface {
	Save(message *model.ChatMessage) error
	GetHistory(userID, friendID uint) ([]*model.ChatMessage, error)
	GetAllHistory(userID uint) ([]*model.ChatMessage, error)
	DeleteHistory(userID, friendID uint) error
	DeleteAllHistory(userID uint) error
}

type chatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) ChatRepository {
	return &chatRepository{db: db}
}

func (r *chatRepository) Save(message *model.ChatMessage) error {
	return r.db.Create(message).Error
}

func (r *chatRepository) GetHistory(userID, friendID uint) ([]*model.ChatMessage, error) {
	var messages []*model.ChatMessage
	err := r.db.Where("((from_user_id = ? AND to_user_id = ? AND sender_deleted = ?) OR (from_user_id = ? AND to_user_id = ? AND receiver_deleted = ?))",
		userID, friendID, false, friendID, userID, false).
		Order("created_at asc").
		Find(&messages).Error
	return messages, err
}

func (r *chatRepository) GetAllHistory(userID uint) ([]*model.ChatMessage, error) {
	var messages []*model.ChatMessage
	err := r.db.Where("(from_user_id = ? AND sender_deleted = ?) OR (to_user_id = ? AND receiver_deleted = ?)",
		userID, false, userID, false).
		Order("created_at asc").
		Find(&messages).Error
	return messages, err
}

func (r *chatRepository) DeleteHistory(userID, friendID uint) error {
	// 自己视角的聊天记录
	err := r.db.Model(&model.ChatMessage{}).
		Where("from_user_id = ? AND to_user_id = ?", userID, friendID).
		Update("sender_deleted", true).Error
	if err != nil {
		return err
	}
	return r.db.Model(&model.ChatMessage{}).
		Where("from_user_id = ? AND to_user_id = ?", friendID, userID).
		Update("receiver_deleted", true).Error
}

func (r *chatRepository) DeleteAllHistory(userID uint) error {
	err := r.db.Model(&model.ChatMessage{}).
		Where("from_user_id = ?", userID).
		Update("sender_deleted", true).Error
	if err != nil {
		return err
	}
	return r.db.Model(&model.ChatMessage{}).
		Where("to_user_id = ?", userID).
		Update("receiver_deleted", true).Error
}
