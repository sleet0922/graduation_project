package model

import "time"

type E2EEGroupKey struct {
	GroupID    uint      `json:"group_id" gorm:"primaryKey;autoIncrement:false"`
	KeyVersion int       `json:"key_version" gorm:"primaryKey;autoIncrement:false"`
	Algo       string    `json:"algo" gorm:"type:varchar(64);not null"`
	CreatedBy  uint      `json:"created_by" gorm:"not null"`
	CreatedAt  time.Time `json:"created_at"`
}

func (E2EEGroupKey) TableName() string {
	return "e2ee_group_keys"
}

type E2EEGroupKeyBox struct {
	GroupID         uint      `json:"group_id" gorm:"primaryKey;autoIncrement:false;index:idx_e2ee_gkb_group_version,priority:1"`
	KeyVersion      int       `json:"key_version" gorm:"primaryKey;autoIncrement:false;index:idx_e2ee_gkb_group_version,priority:2"`
	UserID          uint      `json:"user_id" gorm:"primaryKey;autoIncrement:false;index:idx_e2ee_gkb_user"`
	WrappedGroupKey string    `json:"wrapped_group_key" gorm:"type:text;not null"`
	WrapNonce       string    `json:"wrap_nonce" gorm:"type:text;not null"`
	KeyWrapAlg      string    `json:"key_wrap_alg" gorm:"type:varchar(64);not null"`
	WrappedByUserID uint      `json:"wrapped_by_user_id" gorm:"not null"`
	CreatedAt       time.Time `json:"created_at"`
}

func (E2EEGroupKeyBox) TableName() string {
	return "e2ee_group_key_boxes"
}
