package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/pkg/logger"
	"strings"

	"gorm.io/gorm"
)

var (
	ErrUnsupportedE2EEKeyType  = errors.New("key_type 仅支持 x25519")
	ErrInvalidE2EEPublicKey    = errors.New("public_key 必须是 base64 编码且解码后长度为 32 字节")
	ErrE2EEPublicKeyNotFound   = errors.New("e2ee public key not found")
	ErrE2EEGroupPermission     = errors.New("forbidden: not group member")
	ErrE2EEGroupKeyNotFound    = errors.New("e2ee group key not found")
	ErrE2EEGroupVersionAbsent  = errors.New("e2ee group key version not found")
	ErrE2EEGroupKeyBoxMissing  = errors.New("e2ee group key box not found for current user")
	ErrE2EEGroupBoxesInvalid   = errors.New("invalid e2ee group key boxes payload")
	ErrE2EEGroupVersionLock    = errors.New("cannot publish boxes for historical version")
	ErrE2EEGroupBoxesPublished = errors.New("e2ee group key boxes already published")
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
	RotateGroupKeyIfCurrent(ctx context.Context, currentUserID, groupID uint, expectedVersion int) (int, error)
	PublishGroupKeyBoxes(ctx context.Context, currentUserID, groupID uint, keyVersion int, boxes []GroupKeyBoxUpload, keyWrapAlg string) error
}

type e2eeService struct {
	keyRepo      repo.E2EEKeyRepository
	groupRepo    repo.GroupRepository
	groupKeyRepo repo.E2EEGroupKeyRepository
	friendRepo   repo.FriendRepository
	chatService  ChatService
}

func NewE2EEService(keyRepo repo.E2EEKeyRepository, groupRepo repo.GroupRepository, groupKeyRepo repo.E2EEGroupKeyRepository, friendRepo repo.FriendRepository, chatService ChatService) E2EEService {
	return &e2eeService{
		keyRepo:      keyRepo,
		groupRepo:    groupRepo,
		groupKeyRepo: groupKeyRepo,
		friendRepo:   friendRepo,
		chatService:  chatService,
	}
}

// ----------E2EE工具函数----------
// 解码Base64
func decodeBase64URLOrStd(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty base64 input")
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	return nil, fmt.Errorf("invalid base64")
}

// 验证格式
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
	if len(groupKey) <= 16 {
		return false
	}
	nonce, err := decodeBase64URLOrStd(box.WrapNonce)
	return err == nil && len(nonce) == 12
}

// ----------E2EE公共方法----------
// 发布/更新自己的公钥
func (s *e2eeService) PublishUserPublicKey(ctx context.Context, userID uint, keyType, publicKey string) (*model.E2EEUserPublicKey, error) {
	normalizedKeyType := strings.ToLower(strings.TrimSpace(keyType))
	if normalizedKeyType != "x25519" {
		return nil, ErrUnsupportedE2EEKeyType
	}
	normalizedPublicKey := strings.TrimSpace(publicKey)
	decoded, err := decodeBase64URLOrStd(normalizedPublicKey)
	if err != nil || len(decoded) != 32 {
		return nil, ErrInvalidE2EEPublicKey
	}
	normalizedPublicKey = base64.StdEncoding.EncodeToString(decoded)
	unlockIdentityState := lockE2EEIdentityWrite()
	defer unlockIdentityState()

	var oldPublicKey string
	oldKey, err := s.keyRepo.GetByUserID(ctx, userID)
	if err == nil && oldKey != nil {
		oldPublicKey = oldKey.PublicKey
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	record := &model.E2EEUserPublicKey{
		UserID:    userID,
		KeyType:   normalizedKeyType,
		PublicKey: normalizedPublicKey,
	}
	if err := s.keyRepo.Upsert(ctx, record); err != nil {
		return nil, err
	}

	keyChanged := oldPublicKey != normalizedPublicKey
	if keyChanged {
		groups, err := s.groupRepo.GetGroupsByUserID(ctx, userID)
		if err != nil {
			return nil, s.rollbackIdentityChange(ctx, userID, oldKey, err)
		}
		rotatedVersions := make(map[uint]int, len(groups))
		for _, group := range groups {
			unlockGroupState := lockE2EEGroupState(group.ID)
			rotated, rotateErr := s.groupKeyRepo.CreateNextVersion(ctx, group.ID, userID)
			unlockGroupState()
			if rotateErr != nil {
				cause := fmt.Errorf("rotate group %d after identity change: %w", group.ID, rotateErr)
				return nil, s.rollbackIdentityChange(ctx, userID, oldKey, cause)
			}
			rotatedVersions[group.ID] = rotated.KeyVersion
		}
		for groupID, version := range rotatedVersions {
			s.notifyGroupKeyChanged(ctx, groupID, version)
		}
		go s.notifyFriendsKeyChanged(userID)
	}

	return s.keyRepo.GetByUserID(ctx, userID)
}

func (s *e2eeService) restoreUserPublicKey(ctx context.Context, userID uint, oldKey *model.E2EEUserPublicKey) error {
	if oldKey == nil {
		return s.keyRepo.DeleteByUserID(ctx, userID)
	}
	restored := *oldKey
	return s.keyRepo.Upsert(ctx, &restored)
}

func (s *e2eeService) rollbackIdentityChange(ctx context.Context, userID uint, oldKey *model.E2EEUserPublicKey, cause error) error {
	if restoreErr := s.restoreUserPublicKey(ctx, userID, oldKey); restoreErr != nil {
		return errors.Join(cause, fmt.Errorf("restore identity key after failed rotation: %w", restoreErr))
	}
	return cause
}

func (s *e2eeService) notifyGroupKeyChanged(ctx context.Context, groupID uint, keyVersion int) {
	if s.chatService == nil {
		return
	}
	members, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
	if err != nil {
		logger.Error("failed to list members for e2ee group key notification", "group_id", groupID, "key_version", keyVersion, "error", err)
		return
	}
	userIDs := make([]uint, 0, len(members))
	for _, member := range members {
		userIDs = append(userIDs, member.UserID)
	}
	if len(userIDs) == 0 {
		return
	}
	s.chatService.PushSystemEvent(ctx, userIDs, map[string]any{
		"type":        "e2ee_group_key_changed",
		"group_id":    groupID,
		"key_version": keyVersion,
	})
}

func (s *e2eeService) notifyFriendsKeyChanged(userID uint) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("notifyFriendsKeyChanged panic recovered", "user_id", userID, "panic", r)
		}
	}()
	if s.friendRepo == nil || s.chatService == nil {
		return
	}
	ctx := context.Background()
	friends, err := s.friendRepo.GetByUserID(ctx, userID)
	if err != nil {
		logger.Error("failed to get friend list for e2ee key change notification", "user_id", userID, "error", err)
		return
	}
	if len(friends) == 0 {
		return
	}
	friendIDs := make([]uint, 0, len(friends))
	for _, f := range friends {
		friendIDs = append(friendIDs, f.FriendID)
	}
	s.chatService.PushSystemEvent(ctx, friendIDs, map[string]any{
		"type":    "e2ee_key_changed",
		"user_id": userID,
	})
}

// 获取用户的公钥
func (s *e2eeService) GetUserPublicKey(ctx context.Context, userID uint) (*model.E2EEUserPublicKey, error) {
	unlockIdentityState := lockE2EEIdentityRead()
	defer unlockIdentityState()
	key, err := s.keyRepo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrE2EEPublicKeyNotFound
		}
		return nil, err
	}
	return key, nil
}

// 获取群聊当前版本的密钥盒
func (s *e2eeService) GetGroupCurrentKeyBox(ctx context.Context, currentUserID, groupID uint) (*model.E2EEGroupKeyBox, error) {
	unlockIdentityState := lockE2EEIdentityRead()
	defer unlockIdentityState()
	unlockGroupState := lockE2EEGroupState(groupID)
	defer unlockGroupState()
	if !s.groupRepo.IsMember(ctx, groupID, currentUserID) {
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
		if err := s.RotateGroupKey(ctx, groupID, currentUserID); err != nil {
			return nil, err
		}
		currentVersion, err = s.groupKeyRepo.GetCurrentVersion(ctx, groupID)
		if err != nil {
			return nil, err
		}
		s.notifyGroupKeyChanged(ctx, groupID, currentVersion)
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

// 获取群聊指定版本的密钥盒
func (s *e2eeService) GetGroupKeyBoxByVersion(ctx context.Context, currentUserID, groupID uint, keyVersion int) (*model.E2EEGroupKeyBox, error) {
	unlockIdentityState := lockE2EEIdentityRead()
	defer unlockIdentityState()
	if !s.groupRepo.IsMember(ctx, groupID, currentUserID) {
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

// 获取群聊当前版本
func (s *e2eeService) GetGroupCurrentVersion(ctx context.Context, groupID uint) (int, error) {
	return s.groupKeyRepo.GetCurrentVersion(ctx, groupID)
}

// 轮转群聊密钥（生成新版本）
func (s *e2eeService) RotateGroupKey(ctx context.Context, groupID, currentUserID uint) error {
	members, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return ErrGroupMemberNotFound
	}
	_, err = s.groupKeyRepo.CreateNextVersion(ctx, groupID, currentUserID)
	return err
}

func (s *e2eeService) RotateGroupKeyIfCurrent(ctx context.Context, currentUserID, groupID uint, expectedVersion int) (int, error) {
	if expectedVersion <= 0 {
		return 0, ErrE2EEGroupBoxesInvalid
	}
	unlockIdentityState := lockE2EEIdentityRead()
	defer unlockIdentityState()
	unlockGroupState := lockE2EEGroupState(groupID)
	defer unlockGroupState()
	if !s.groupRepo.IsMember(ctx, groupID, currentUserID) {
		return 0, ErrE2EEGroupPermission
	}
	key, err := s.groupKeyRepo.CreateNextVersionIfCurrent(ctx, groupID, expectedVersion, currentUserID)
	if err != nil {
		return 0, err
	}
	if key.KeyVersion > expectedVersion {
		s.notifyGroupKeyChanged(ctx, groupID, key.KeyVersion)
	}
	return key.KeyVersion, nil
}

// 发布群聊密钥盒
func (s *e2eeService) PublishGroupKeyBoxes(ctx context.Context, currentUserID, groupID uint, keyVersion int, boxes []GroupKeyBoxUpload, keyWrapAlg string) error {
	unlockIdentityState := lockE2EEIdentityRead()
	defer unlockIdentityState()
	unlockGroupState := lockE2EEGroupState(groupID)
	defer unlockGroupState()
	if !s.groupRepo.IsMember(ctx, groupID, currentUserID) {
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
	members, err := s.groupRepo.GetMembersByGroupID(ctx, groupID)
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
	if err := s.groupKeyRepo.ReplaceVersionBoxes(ctx, groupID, keyVersion, modelBoxes); err != nil {
		switch {
		case errors.Is(err, repo.ErrE2EEGroupKeyBoxesExist):
			return ErrE2EEGroupBoxesPublished
		case errors.Is(err, repo.ErrE2EEGroupKeyVersionStale):
			return ErrE2EEGroupVersionLock
		default:
			return err
		}
	}
	return nil
}
