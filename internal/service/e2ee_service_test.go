package service

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"testing"

	"gorm.io/gorm"

	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
)

type fakeE2EEKeyRepo struct {
	mu   sync.Mutex
	keys map[uint]*model.E2EEUserPublicKey
	err  error
}

func newFakeE2EEKeyRepo() *fakeE2EEKeyRepo {
	return &fakeE2EEKeyRepo{keys: make(map[uint]*model.E2EEUserPublicKey)}
}

func (r *fakeE2EEKeyRepo) Upsert(ctx context.Context, key *model.E2EEUserPublicKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	copied := *key
	r.keys[key.UserID] = &copied
	return nil
}

func (r *fakeE2EEKeyRepo) GetByUserID(ctx context.Context, userID uint) (*model.E2EEUserPublicKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	key, ok := r.keys[userID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copied := *key
	return &copied, nil
}

type fakeE2EEGroupKeyRepo struct {
	mu       sync.Mutex
	versions map[uint]int
	boxes    map[[3]uint]*model.E2EEGroupKeyBox
	err      error
}

func newFakeE2EEGroupKeyRepo() *fakeE2EEGroupKeyRepo {
	return &fakeE2EEGroupKeyRepo{
		versions: make(map[uint]int),
		boxes:    make(map[[3]uint]*model.E2EEGroupKeyBox),
	}
}

func boxKey(groupID uint, version int, userID uint) [3]uint {
	return [3]uint{groupID, uint(version), userID}
}

func (r *fakeE2EEGroupKeyRepo) GetCurrentVersion(ctx context.Context, groupID uint) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return 0, r.err
	}
	version := r.versions[groupID]
	if version == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return version, nil
}

func (r *fakeE2EEGroupKeyRepo) ExistsVersion(ctx context.Context, groupID uint, keyVersion int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return false, r.err
	}
	return r.versions[groupID] >= keyVersion && keyVersion > 0, nil
}

func (r *fakeE2EEGroupKeyRepo) GetCurrentUserKeyBox(ctx context.Context, groupID, userID uint) (*model.E2EEGroupKeyBox, error) {
	version, err := r.GetCurrentVersion(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return r.GetUserKeyBoxByVersion(ctx, groupID, version, userID)
}

func (r *fakeE2EEGroupKeyRepo) GetUserKeyBoxByVersion(ctx context.Context, groupID uint, keyVersion int, userID uint) (*model.E2EEGroupKeyBox, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	box, ok := r.boxes[boxKey(groupID, keyVersion, userID)]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copied := *box
	return &copied, nil
}

func (r *fakeE2EEGroupKeyRepo) CreateNextVersion(ctx context.Context, groupID, createdBy uint) (*model.E2EEGroupKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	r.versions[groupID]++
	return &model.E2EEGroupKey{GroupID: groupID, KeyVersion: r.versions[groupID], CreatedBy: createdBy}, nil
}

func (r *fakeE2EEGroupKeyRepo) CreateNextVersionIfCurrent(ctx context.Context, groupID uint, expectedVersion int, createdBy uint) (*model.E2EEGroupKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	if r.versions[groupID] == expectedVersion {
		r.versions[groupID]++
	}
	return &model.E2EEGroupKey{GroupID: groupID, KeyVersion: r.versions[groupID], CreatedBy: createdBy}, nil
}

func (r *fakeE2EEGroupKeyRepo) GetVersionBoxes(ctx context.Context, groupID uint, keyVersion int) ([]*model.E2EEGroupKeyBox, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	var boxes []*model.E2EEGroupKeyBox
	for key, box := range r.boxes {
		if key[0] == groupID && key[1] == uint(keyVersion) {
			copied := *box
			boxes = append(boxes, &copied)
		}
	}
	return boxes, nil
}

func (r *fakeE2EEGroupKeyRepo) ReplaceVersionBoxes(ctx context.Context, groupID uint, keyVersion int, boxes []*model.E2EEGroupKeyBox) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	for key := range r.boxes {
		if key[0] == groupID && key[1] == uint(keyVersion) {
			return repo.ErrE2EEGroupKeyBoxesExist
		}
	}
	for _, box := range boxes {
		copied := *box
		r.boxes[boxKey(groupID, keyVersion, box.UserID)] = &copied
	}
	return nil
}

func TestE2EEServicePublishUserPublicKey(t *testing.T) {
	ctx := context.Background()
	keyRepo := newFakeE2EEKeyRepo()
	svc := NewE2EEService(keyRepo, newFakeGroupRepo(), newFakeE2EEGroupKeyRepo(), newFakeFriendRepo(), nil)
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))

	key, err := svc.PublishUserPublicKey(ctx, 1, " X25519 ", " "+validKey+" ")
	if err != nil {
		t.Fatalf("PublishUserPublicKey failed: %v", err)
	}
	if key.KeyType != "x25519" || key.PublicKey != validKey {
		t.Fatalf("published key = %#v, want normalized x25519 and trimmed public key", key)
	}

	if _, err := svc.PublishUserPublicKey(ctx, 1, "ed25519", validKey); !errors.Is(err, ErrUnsupportedE2EEKeyType) {
		t.Fatalf("bad key type error = %v, want ErrUnsupportedE2EEKeyType", err)
	}
	if _, err := svc.PublishUserPublicKey(ctx, 1, "x25519", base64.StdEncoding.EncodeToString(make([]byte, 31))); !errors.Is(err, ErrInvalidE2EEPublicKey) {
		t.Fatalf("bad public key error = %v, want ErrInvalidE2EEPublicKey", err)
	}
}

func TestE2EEServiceIdentityKeyChangeRotatesUserGroups(t *testing.T) {
	ctx := context.Background()
	keyRepo := newFakeE2EEKeyRepo()
	groupRepo := newFakeGroupRepo()
	groupRepo.groups[10] = &model.ChatGroup{ID: 10, OwnerID: 1}
	groupRepo.members[10] = map[uint]*model.ChatGroupMember{1: {GroupID: 10, UserID: 1}}
	groupKeyRepo := newFakeE2EEGroupKeyRepo()
	groupKeyRepo.versions[10] = 1
	svc := NewE2EEService(keyRepo, groupRepo, groupKeyRepo, newFakeFriendRepo(), nil)
	firstKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	secondKeyBytes := make([]byte, 32)
	secondKeyBytes[0] = 1
	secondKey := base64.StdEncoding.EncodeToString(secondKeyBytes)

	if _, err := svc.PublishUserPublicKey(ctx, 1, "x25519", firstKey); err != nil {
		t.Fatalf("first key publish failed: %v", err)
	}
	if _, err := svc.PublishUserPublicKey(ctx, 1, "x25519", secondKey); err != nil {
		t.Fatalf("replacement key publish failed: %v", err)
	}
	if version, _ := groupKeyRepo.GetCurrentVersion(ctx, 10); version != 2 {
		t.Fatalf("current group key version = %d, want 2 after identity key change", version)
	}
}

func TestE2EEServiceGroupKeyBoxes(t *testing.T) {
	ctx := context.Background()
	groupRepo := newFakeGroupRepo()
	groupRepo.groups[10] = &model.ChatGroup{ID: 10, OwnerID: 1}
	groupRepo.members[10] = map[uint]*model.ChatGroupMember{
		1: {GroupID: 10, UserID: 1},
		2: {GroupID: 10, UserID: 2},
	}
	groupKeyRepo := newFakeE2EEGroupKeyRepo()
	groupKeyRepo.versions[10] = 1
	svc := NewE2EEService(newFakeE2EEKeyRepo(), groupRepo, groupKeyRepo, newFakeFriendRepo(), nil)

	validWrapped := base64.StdEncoding.EncodeToString(make([]byte, 32))
	validNonce := base64.StdEncoding.EncodeToString(make([]byte, 12))

	if err := svc.PublishGroupKeyBoxes(ctx, 99, 10, 1, []GroupKeyBoxUpload{{UserID: 2, WrappedGroupKey: validWrapped, WrapNonce: validNonce}}, ""); !errors.Is(err, ErrE2EEGroupPermission) {
		t.Fatalf("non member publish error = %v, want ErrE2EEGroupPermission", err)
	}
	if err := svc.PublishGroupKeyBoxes(ctx, 1, 10, 2, []GroupKeyBoxUpload{{UserID: 2, WrappedGroupKey: validWrapped, WrapNonce: validNonce}}, ""); !errors.Is(err, ErrE2EEGroupVersionLock) {
		t.Fatalf("historical version error = %v, want ErrE2EEGroupVersionLock", err)
	}
	if err := svc.PublishGroupKeyBoxes(ctx, 1, 10, 1, []GroupKeyBoxUpload{{UserID: 2, WrappedGroupKey: "bad", WrapNonce: validNonce}}, ""); !errors.Is(err, ErrE2EEGroupBoxesInvalid) {
		t.Fatalf("bad wrapped key error = %v, want ErrE2EEGroupBoxesInvalid", err)
	}
	validBoxes := []GroupKeyBoxUpload{
		{UserID: 1, WrappedGroupKey: validWrapped, WrapNonce: validNonce},
		{UserID: 2, WrappedGroupKey: validWrapped, WrapNonce: validNonce},
	}
	if err := svc.PublishGroupKeyBoxes(ctx, 1, 10, 1, validBoxes, ""); err != nil {
		t.Fatalf("PublishGroupKeyBoxes failed: %v", err)
	}
	if err := svc.PublishGroupKeyBoxes(ctx, 2, 10, 1, validBoxes, ""); !errors.Is(err, ErrE2EEGroupBoxesPublished) {
		t.Fatalf("second publisher error = %v, want ErrE2EEGroupBoxesPublished", err)
	}

	box, err := svc.GetGroupKeyBoxByVersion(ctx, 2, 10, 1)
	if err != nil {
		t.Fatalf("GetGroupKeyBoxByVersion failed: %v", err)
	}
	if box.UserID != 2 || box.KeyWrapAlg != "chacha20poly1305-v1" {
		t.Fatalf("box = %#v, want user 2 with default algorithm", box)
	}
}

func TestE2EEServiceRotateGroupKeyIfCurrentIsIdempotent(t *testing.T) {
	ctx := context.Background()
	groupRepo := newFakeGroupRepo()
	groupRepo.groups[10] = &model.ChatGroup{ID: 10, OwnerID: 1}
	groupRepo.members[10] = map[uint]*model.ChatGroupMember{1: {GroupID: 10, UserID: 1}}
	groupKeyRepo := newFakeE2EEGroupKeyRepo()
	groupKeyRepo.versions[10] = 1
	svc := NewE2EEService(newFakeE2EEKeyRepo(), groupRepo, groupKeyRepo, newFakeFriendRepo(), nil)

	version, err := svc.RotateGroupKeyIfCurrent(ctx, 1, 10, 1)
	if err != nil || version != 2 {
		t.Fatalf("first recovery rotation = version %d error %v, want version 2", version, err)
	}
	version, err = svc.RotateGroupKeyIfCurrent(ctx, 1, 10, 1)
	if err != nil || version != 2 {
		t.Fatalf("repeated recovery rotation = version %d error %v, want version 2", version, err)
	}
}

func TestE2EEServiceRotateGroupKeyRequiresMembers(t *testing.T) {
	ctx := context.Background()
	groupRepo := newFakeGroupRepo()
	groupRepo.groups[10] = &model.ChatGroup{ID: 10, OwnerID: 1}
	groupKeyRepo := newFakeE2EEGroupKeyRepo()
	svc := NewE2EEService(newFakeE2EEKeyRepo(), groupRepo, groupKeyRepo, newFakeFriendRepo(), nil)

	if err := svc.RotateGroupKey(ctx, 10, 1); !errors.Is(err, ErrGroupMemberNotFound) {
		t.Fatalf("RotateGroupKey error = %v, want ErrGroupMemberNotFound", err)
	}

	groupRepo.members[10] = map[uint]*model.ChatGroupMember{1: {GroupID: 10, UserID: 1}}
	if err := svc.RotateGroupKey(ctx, 10, 1); err != nil {
		t.Fatalf("RotateGroupKey failed: %v", err)
	}
	if version, _ := groupKeyRepo.GetCurrentVersion(ctx, 10); version != 1 {
		t.Fatalf("current version = %d, want 1", version)
	}
}
