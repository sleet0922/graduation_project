package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
	"strings"

	"gorm.io/gorm"
)

var (
	ErrUnsupportedE2EEKeyType = errors.New("key_type 仅支持 x25519")
	ErrInvalidE2EEPublicKey   = errors.New("public_key 必须是 base64 编码且解码后长度为 32 字节")
	ErrE2EEPublicKeyNotFound  = errors.New("e2ee public key not found")
	ErrE2EEGroupPermission    = errors.New("forbidden: not group member")
	ErrE2EEGroupKeyNotFound   = errors.New("e2ee group key not found")
	ErrE2EEGroupVersionAbsent = errors.New("e2ee group key version not found")
	ErrE2EEGroupKeyBoxMissing = errors.New("e2ee group key box not found for current user")
	ErrE2EEGroupBoxesInvalid  = errors.New("invalid e2ee group key boxes payload")
	ErrE2EEGroupVersionLock   = errors.New("cannot publish boxes for historical version")
)

const (
	groupWrapAAD = "zat_e2ee_group_wrap_v1"
)

type GroupKeyBoxUpload struct {
	UserID          uint
	WrappedGroupKey string
	WrapNonce       string
}

type E2EEService interface {
	PublishUserPublicKey(ctx context.Context, userID uint, keyType, publicKey string) (*model.E2EEUserPublicKey, error)
	GetUserPublicKey(ctx context.Context, userID uint) (*model.E2EEUserPublicKey, error)
	GetGroupCurrentKeyBox(ctx context.Context, currentUserID, groupID uint) (*model.E2EEGroupKeyBox, error)
	GetGroupKeyBoxByVersion(ctx context.Context, currentUserID, groupID uint, keyVersion int) (*model.E2EEGroupKeyBox, error)
	GetGroupCurrentVersion(ctx context.Context, groupID uint) (int, error)
	RotateGroupKey(ctx context.Context, groupID, currentUserID uint) error
	PublishGroupKeyBoxes(ctx context.Context, currentUserID, groupID uint, keyVersion int, boxes []GroupKeyBoxUpload, keyWrapAlg string) error
}

type e2eeService struct {
	keyRepo      repo.E2EEKeyRepository
	groupRepo    repo.GroupRepository
	groupKeyRepo repo.E2EEGroupKeyRepository
}

func NewE2EEService(keyRepo repo.E2EEKeyRepository, groupRepo repo.GroupRepository, groupKeyRepo repo.E2EEGroupKeyRepository) E2EEService {
	return &e2eeService{
		keyRepo:      keyRepo,
		groupRepo:    groupRepo,
		groupKeyRepo: groupKeyRepo,
	}
}

func (s *e2eeService) PublishUserPublicKey(ctx context.Context, userID uint, keyType, publicKey string) (*model.E2EEUserPublicKey, error) {
	normalizedKeyType := strings.ToLower(strings.TrimSpace(keyType))
	if normalizedKeyType != "x25519" {
		return nil, ErrUnsupportedE2EEKeyType
	}

	normalizedPublicKey := strings.TrimSpace(publicKey)
	decoded, err := base64.StdEncoding.DecodeString(normalizedPublicKey)
	if err != nil || len(decoded) != 32 {
		return nil, ErrInvalidE2EEPublicKey
	}

	record := &model.E2EEUserPublicKey{
		UserID:    userID,
		KeyType:   normalizedKeyType,
		PublicKey: normalizedPublicKey,
	}
	if err := s.keyRepo.Upsert(ctx, record); err != nil {
		return nil, err
	}
	return s.keyRepo.GetByUserID(ctx, userID)
}

func (s *e2eeService) GetUserPublicKey(ctx context.Context, userID uint) (*model.E2EEUserPublicKey, error) {
	key, err := s.keyRepo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrE2EEPublicKeyNotFound
		}
		return nil, err
	}
	return key, nil
}

func (s *e2eeService) GetGroupCurrentKeyBox(ctx context.Context, currentUserID, groupID uint) (*model.E2EEGroupKeyBox, error) {
	if !s.groupRepo.IsMember(groupID, currentUserID) {
		return nil, ErrE2EEGroupPermission
	}
	currentVersion, err := s.groupKeyRepo.GetCurrentVersion(ctx, groupID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrE2EEGroupKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	box, err := s.groupKeyRepo.GetUserKeyBoxByVersion(ctx, groupID, currentVersion, currentUserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrE2EEGroupKeyBoxMissing
	}
	if err != nil {
		return nil, err
	}
	if !isSupportedKeyBox(box) {
		// 兼容旧数据：若当前版本仍为历史格式（如24字节nonce），自动轮换新版本后返回。
		if err := s.RotateGroupKey(ctx, groupID, currentUserID); err != nil {
			return nil, err
		}
		currentVersion, err = s.groupKeyRepo.GetCurrentVersion(ctx, groupID)
		if err != nil {
			return nil, err
		}
		box, err = s.groupKeyRepo.GetUserKeyBoxByVersion(ctx, groupID, currentVersion, currentUserID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrE2EEGroupKeyBoxMissing
		}
		if err != nil {
			return nil, err
		}
	}
	return box, nil
}

func (s *e2eeService) GetGroupKeyBoxByVersion(ctx context.Context, currentUserID, groupID uint, keyVersion int) (*model.E2EEGroupKeyBox, error) {
	if !s.groupRepo.IsMember(groupID, currentUserID) {
		return nil, ErrE2EEGroupPermission
	}
	exists, err := s.groupKeyRepo.ExistsVersion(ctx, groupID, keyVersion)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrE2EEGroupVersionAbsent
	}
	box, err := s.groupKeyRepo.GetUserKeyBoxByVersion(ctx, groupID, keyVersion, currentUserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrE2EEGroupKeyBoxMissing
	}
	if err != nil {
		return nil, err
	}
	return box, nil
}

func (s *e2eeService) GetGroupCurrentVersion(ctx context.Context, groupID uint) (int, error) {
	return s.groupKeyRepo.GetCurrentVersion(ctx, groupID)
}

func (s *e2eeService) RotateGroupKey(ctx context.Context, groupID, currentUserID uint) error {
	members, err := s.groupRepo.GetMembersByGroupID(groupID)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return ErrGroupMemberNotFound
	}
	_, err = s.groupKeyRepo.CreateNextVersion(ctx, groupID, currentUserID)
	return err
}

func (s *e2eeService) PublishGroupKeyBoxes(ctx context.Context, currentUserID, groupID uint, keyVersion int, boxes []GroupKeyBoxUpload, keyWrapAlg string) error {
	if !s.groupRepo.IsMember(groupID, currentUserID) {
		return ErrE2EEGroupPermission
	}
	if keyVersion <= 0 || len(boxes) == 0 {
		return ErrE2EEGroupBoxesInvalid
	}
	if keyWrapAlg == "" {
		keyWrapAlg = "chacha20poly1305-v1"
	}
	if keyWrapAlg != "chacha20poly1305-v1" {
		return ErrE2EEGroupBoxesInvalid
	}
	currentVersion, err := s.groupKeyRepo.GetCurrentVersion(ctx, groupID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrE2EEGroupKeyNotFound
	}
	if err != nil {
		return err
	}
	if keyVersion != currentVersion {
		return ErrE2EEGroupVersionLock
	}
	members, err := s.groupRepo.GetMembersByGroupID(groupID)
	if err != nil {
		return err
	}
	memberSet := make(map[uint]struct{}, len(members))
	for _, member := range members {
		memberSet[member.UserID] = struct{}{}
	}
	seen := make(map[uint]struct{}, len(boxes))
	modelBoxes := make([]*model.E2EEGroupKeyBox, 0, len(boxes))
	for _, box := range boxes {
		if box.UserID == 0 || strings.TrimSpace(box.WrappedGroupKey) == "" || strings.TrimSpace(box.WrapNonce) == "" {
			return ErrE2EEGroupBoxesInvalid
		}
		if _, ok := memberSet[box.UserID]; !ok {
			return ErrE2EEGroupBoxesInvalid
		}
		if _, ok := seen[box.UserID]; ok {
			return ErrE2EEGroupBoxesInvalid
		}
		seen[box.UserID] = struct{}{}
		wrappedRaw, err := decodeBase64URLOrStd(box.WrappedGroupKey)
		if err != nil || len(wrappedRaw) <= 16 {
			return ErrE2EEGroupBoxesInvalid
		}
		nonceRaw, err := decodeBase64URLOrStd(box.WrapNonce)
		if err != nil || len(nonceRaw) != 12 {
			return ErrE2EEGroupBoxesInvalid
		}
		modelBoxes = append(modelBoxes, &model.E2EEGroupKeyBox{
			GroupID:         groupID,
			KeyVersion:      keyVersion,
			UserID:          box.UserID,
			WrappedGroupKey: strings.TrimSpace(box.WrappedGroupKey),
			WrapNonce:       strings.TrimSpace(box.WrapNonce),
			KeyWrapAlg:      keyWrapAlg,
			WrappedByUserID: currentUserID, // 记录加密者（当前用户）的ID
		})
	}
	if len(seen) != len(memberSet) {
		return ErrE2EEGroupBoxesInvalid
	}
	return s.groupKeyRepo.ReplaceVersionBoxes(ctx, groupID, keyVersion, modelBoxes)
}

func isSupportedKeyBox(box *model.E2EEGroupKeyBox) bool {
	if box == nil {
		return false
	}
	if box.KeyWrapAlg != "chacha20poly1305-v1" {
		return false
	}
	groupKey, err := decodeBase64URLOrStd(box.WrappedGroupKey)
	if err != nil {
		return false
	}
	// ciphertext + tag 至少大于纯明文长度
	if len(groupKey) <= 16 {
		return false
	}
	nonce, err := decodeBase64URLOrStd(box.WrapNonce)
	return err == nil && len(nonce) == 12
}

func decodeBase64URLOrStd(raw string) ([]byte, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty base64 input")
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	return nil, fmt.Errorf("invalid base64")
}
