package repo

import (
	"fmt"
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
)

type ChatRepository interface {
	Save(message *model.ChatMessage) error
	GetHistory(userID, friendID uint) ([]*model.ChatMessage, error)
	GetGroupHistory(userID, groupID uint) ([]*model.ChatMessage, error)
	GetAllHistory(userID uint) ([]*model.ChatMessage, error)
	DeleteHistory(userID, friendID uint) error
	DeleteGroupHistory(userID, groupID uint) error
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
	err := r.db.Where("(conversation_type = ? OR conversation_type = '') AND ((from_user_id = ? AND to_user_id = ? AND sender_deleted = ?) OR (from_user_id = ? AND to_user_id = ? AND receiver_deleted = ?))",
		"single", userID, friendID, false, friendID, userID, false).
		Order("created_at asc").
		Find(&messages).Error
	return messages, err
}

func (r *chatRepository) GetGroupHistory(userID, groupID uint) ([]*model.ChatMessage, error) {
	var messages []*model.ChatMessage
	err := r.db.Where("conversation_type = ? AND group_id = ? AND (deleted_by IS NULL OR deleted_by NOT LIKE ?)",
		"group", groupID, deletedMarker(userID)).
		Order("created_at asc").
		Find(&messages).Error
	return messages, err
}

func (r *chatRepository) GetAllHistory(userID uint) ([]*model.ChatMessage, error) {
	var messages []*model.ChatMessage
	err := r.db.Where("((conversation_type = ? OR conversation_type = '') AND ((from_user_id = ? AND sender_deleted = ?) OR (to_user_id = ? AND receiver_deleted = ?))) OR (conversation_type = ? AND (deleted_by IS NULL OR deleted_by NOT LIKE ?))",
		"single", userID, false, userID, false, "group", deletedMarker(userID)).
		Order("created_at asc").
		Find(&messages).Error
	return messages, err
}

func (r *chatRepository) DeleteHistory(userID, friendID uint) error {
	err := r.db.Model(&model.ChatMessage{}).
		Where("(conversation_type = ? OR conversation_type = '') AND from_user_id = ? AND to_user_id = ?", "single", userID, friendID).
		Update("sender_deleted", true).Error
	if err != nil {
		return err
	}
	return r.db.Model(&model.ChatMessage{}).
		Where("(conversation_type = ? OR conversation_type = '') AND from_user_id = ? AND to_user_id = ?", "single", friendID, userID).
		Update("receiver_deleted", true).Error
}

func (r *chatRepository) DeleteGroupHistory(userID, groupID uint) error {
	return r.db.Model(&model.ChatMessage{}).
		Where("conversation_type = ? AND group_id = ? AND (deleted_by IS NULL OR deleted_by NOT LIKE ?)", "group", groupID, deletedMarker(userID)).
		Update("deleted_by", gorm.Expr("CONCAT(COALESCE(deleted_by, ''), ?)", deletedMarker(userID))).Error
}

func (r *chatRepository) DeleteAllHistory(userID uint) error {
	err := r.db.Model(&model.ChatMessage{}).
		Where("(conversation_type = ? OR conversation_type = '') AND from_user_id = ?", "single", userID).
		Update("sender_deleted", true).Error
	if err != nil {
		return err
	}
	err = r.db.Model(&model.ChatMessage{}).
		Where("(conversation_type = ? OR conversation_type = '') AND to_user_id = ?", "single", userID).
		Update("receiver_deleted", true).Error
	if err != nil {
		return err
	}
	return r.db.Model(&model.ChatMessage{}).
		Where("conversation_type = ? AND (deleted_by IS NULL OR deleted_by NOT LIKE ?)", "group", deletedMarker(userID)).
		Update("deleted_by", gorm.Expr("CONCAT(COALESCE(deleted_by, ''), ?)", deletedMarker(userID))).Error
}

func deletedMarker(userID uint) string {
	return fmt.Sprintf(",%d,", userID)
}
