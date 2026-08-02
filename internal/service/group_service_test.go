package service

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"sleet0922/graduation_project/internal/model"
)

type fakeGroupE2EEService struct {
	version     int
	rotateErr   error
	rotateCalls int
}

func (s *fakeGroupE2EEService) PublishUserPublicKey(context.Context, uint, string, string) (*model.E2EEUserPublicKey, error) {
	return nil, nil
}

func (s *fakeGroupE2EEService) GetUserPublicKey(context.Context, uint) (*model.E2EEUserPublicKey, error) {
	return nil, nil
}

func (s *fakeGroupE2EEService) GetGroupCurrentKeyBox(context.Context, uint, uint) (*model.E2EEGroupKeyBox, error) {
	return nil, nil
}

func (s *fakeGroupE2EEService) GetGroupKeyBoxByVersion(context.Context, uint, uint, int) (*model.E2EEGroupKeyBox, error) {
	return nil, nil
}

func (s *fakeGroupE2EEService) GetGroupCurrentVersion(context.Context, uint) (int, error) {
	return s.version, nil
}

func (s *fakeGroupE2EEService) RotateGroupKey(context.Context, uint, uint) error {
	s.rotateCalls++
	if s.rotateErr != nil {
		return s.rotateErr
	}
	s.version++
	return nil
}

func (s *fakeGroupE2EEService) RotateGroupKeyIfCurrent(context.Context, uint, uint, int) (int, error) {
	return s.version, nil
}

func (s *fakeGroupE2EEService) PublishGroupKeyBoxes(context.Context, uint, uint, int, []GroupKeyBoxUpload, string) error {
	return nil
}

func TestNormalizeMemberIDs(t *testing.T) {
	got := normalizeMemberIDs(1, []uint{0, 1, 2, 2, 3, 0, 3})
	want := []uint{2, 3}
	if len(got) != len(want) {
		t.Fatalf("normalizeMemberIDs = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeMemberIDs = %#v, want %#v", got, want)
		}
	}
}

func TestGroupServiceCreateGroup(t *testing.T) {
	ctx := context.Background()
	users := newFakeUserRepo(
		testUser(1, "1", "owner@example.com"),
		testUser(2, "2", "friend@example.com"),
		testUser(3, "3", "not-friend@example.com"),
	)
	friends := newFakeFriendRepo()
	friends.friendships[[2]uint{1, 2}] = true
	groups := newFakeGroupRepo()
	svc := NewGroupService(groups, friends, users, nil, nil)

	if _, err := svc.CreateGroup(ctx, 1, "  ", "", []uint{2}); !errors.Is(err, ErrGroupNameEmpty) {
		t.Fatalf("empty name error = %v, want ErrGroupNameEmpty", err)
	}
	if _, err := svc.CreateGroup(ctx, 1, "g", "", []uint{0, 1}); !errors.Is(err, ErrGroupMembersEmpty) {
		t.Fatalf("empty members error = %v, want ErrGroupMembersEmpty", err)
	}
	if _, err := svc.CreateGroup(ctx, 1, "g", "", []uint{3}); !errors.Is(err, ErrGroupFriendOnly) {
		t.Fatalf("non friend error = %v, want ErrGroupFriendOnly", err)
	}
	if _, err := svc.CreateGroup(ctx, 1, "g", "", []uint{404}); !errors.Is(err, ErrGroupMemberNotFound) {
		t.Fatalf("missing user error = %v, want ErrGroupMemberNotFound", err)
	}

	detail, err := svc.CreateGroup(ctx, 1, " Study Group ", " avatar ", []uint{2, 2, 1, 0})
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if detail.Name != "Study Group" {
		t.Fatalf("group name = %q, want trimmed Study Group", detail.Name)
	}
	if detail.MemberCount != 2 {
		t.Fatalf("member count = %d, want owner plus one friend", detail.MemberCount)
	}
	if !groups.IsMember(ctx, detail.ID, 1) || !groups.IsMember(ctx, detail.ID, 2) {
		t.Fatal("created group missing owner or invitee")
	}
}

func TestGroupServiceMembershipActions(t *testing.T) {
	ctx := context.Background()
	users := newFakeUserRepo(
		testUser(1, "1", "owner@example.com"),
		testUser(2, "2", "member@example.com"),
		testUser(3, "3", "new@example.com"),
	)
	friends := newFakeFriendRepo()
	friends.friendships[[2]uint{1, 3}] = true
	friends.friendships[[2]uint{2, 3}] = true
	groups := newFakeGroupRepo()
	groups.groups[10] = &model.ChatGroup{ID: 10, OwnerID: 1, Name: "g"}
	groups.members[10] = map[uint]*model.ChatGroupMember{
		1: {GroupID: 10, UserID: 1, Role: "owner"},
		2: {GroupID: 10, UserID: 2, Role: "member"},
	}
	svc := NewGroupService(groups, friends, users, nil, nil)

	if _, err := svc.AddMembers(ctx, 99, 10, []uint{3}); !errors.Is(err, ErrGroupPermission) {
		t.Fatalf("non member add error = %v, want ErrGroupPermission", err)
	}
	members, err := svc.AddMembers(ctx, 2, 10, []uint{3})
	if err != nil {
		t.Fatalf("AddMembers failed: %v", err)
	}
	if len(members) != 3 {
		t.Fatalf("members after add = %d, want 3", len(members))
	}

	if err := svc.RemoveMember(ctx, 2, 10, 3); !errors.Is(err, ErrGroupKickDenied) {
		t.Fatalf("non owner remove error = %v, want ErrGroupKickDenied", err)
	}
	if err := svc.RemoveMember(ctx, 1, 10, 1); !errors.Is(err, ErrGroupOwnerProtected) {
		t.Fatalf("remove owner error = %v, want ErrGroupOwnerProtected", err)
	}
	if err := svc.RemoveMember(ctx, 1, 10, 3); err != nil {
		t.Fatalf("RemoveMember failed: %v", err)
	}
	if groups.IsMember(ctx, 10, 3) {
		t.Fatal("removed member is still in group")
	}

	if err := svc.LeaveGroup(ctx, 1, 10); !errors.Is(err, ErrGroupLeaveDenied) {
		t.Fatalf("owner leave error = %v, want ErrGroupLeaveDenied", err)
	}
	if err := svc.LeaveGroup(ctx, 2, 10); err != nil {
		t.Fatalf("member LeaveGroup failed: %v", err)
	}
	if groups.IsMember(ctx, 10, 2) {
		t.Fatal("leaving member is still in group")
	}
}

func TestGroupServiceMembershipRotationFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	rotationErr := errors.New("rotation failed")
	newService := func() (*fakeGroupRepo, GroupService) {
		users := newFakeUserRepo(
			testUser(1, "1", "owner@example.com"),
			testUser(2, "2", "member@example.com"),
			testUser(3, "3", "new@example.com"),
		)
		friends := newFakeFriendRepo()
		friends.friendships[[2]uint{1, 3}] = true
		groups := newFakeGroupRepo()
		groups.groups[10] = &model.ChatGroup{ID: 10, OwnerID: 1, Name: "g"}
		groups.members[10] = map[uint]*model.ChatGroupMember{
			1: {GroupID: 10, UserID: 1, Role: "owner"},
			2: {GroupID: 10, UserID: 2, Role: "member"},
		}
		e2ee := &fakeGroupE2EEService{version: 1, rotateErr: rotationErr}
		return groups, NewGroupService(groups, friends, users, e2ee, nil)
	}

	groups, svc := newService()
	if _, err := svc.AddMembers(ctx, 1, 10, []uint{3}); !errors.Is(err, rotationErr) {
		t.Fatalf("add rotation error = %v", err)
	}
	if groups.IsMember(ctx, 10, 3) {
		t.Fatal("new member remained after key rotation failed")
	}

	groups, svc = newService()
	if err := svc.RemoveMember(ctx, 1, 10, 2); !errors.Is(err, rotationErr) {
		t.Fatalf("remove rotation error = %v", err)
	}
	if !groups.IsMember(ctx, 10, 2) {
		t.Fatal("removed member was not restored after key rotation failed")
	}

	groups, svc = newService()
	if err := svc.LeaveGroup(ctx, 2, 10); !errors.Is(err, rotationErr) {
		t.Fatalf("leave rotation error = %v", err)
	}
	if !groups.IsMember(ctx, 10, 2) {
		t.Fatal("leaving member was not restored after key rotation failed")
	}

	users := newFakeUserRepo(
		testUser(1, "1", "owner@example.com"),
		testUser(3, "3", "new@example.com"),
	)
	friends := newFakeFriendRepo()
	friends.friendships[[2]uint{1, 3}] = true
	groups = newFakeGroupRepo()
	svc = NewGroupService(
		groups,
		friends,
		users,
		&fakeGroupE2EEService{rotateErr: rotationErr},
		nil,
	)
	if _, err := svc.CreateGroup(ctx, 1, "g", "", []uint{3}); !errors.Is(err, rotationErr) {
		t.Fatalf("create rotation error = %v", err)
	}
	if len(groups.groups) != 0 || len(groups.members) != 0 {
		t.Fatalf("group was not rolled back after initial rotation failed: groups=%#v members=%#v", groups.groups, groups.members)
	}
}

func TestGroupServiceMembershipRotationBroadcastsCurrentVersion(t *testing.T) {
	ctx := context.Background()
	users := newFakeUserRepo(
		testUser(1, "1", "owner@example.com"),
		testUser(2, "2", "member@example.com"),
		testUser(3, "3", "new@example.com"),
	)
	friends := newFakeFriendRepo()
	friends.friendships[[2]uint{1, 3}] = true
	groups := newFakeGroupRepo()
	groups.groups[10] = &model.ChatGroup{ID: 10, OwnerID: 1, Name: "g"}
	groups.members[10] = map[uint]*model.ChatGroupMember{
		1: {GroupID: 10, UserID: 1, Role: "owner"},
		2: {GroupID: 10, UserID: 2, Role: "member"},
	}
	chat := NewChatService(friends, groups)
	events := map[uint][]map[string]any{}
	for _, userID := range []uint{1, 2, 3} {
		id := userID
		chat.RegisterConnection(ctx, id, nil, func(payload any) error {
			if event, ok := payload.(map[string]any); ok {
				events[id] = append(events[id], event)
			}
			return nil
		}, nil)
	}
	e2ee := &fakeGroupE2EEService{version: 1}
	svc := NewGroupService(groups, friends, users, e2ee, chat)

	if _, err := svc.AddMembers(ctx, 1, 10, []uint{3}); err != nil {
		t.Fatalf("AddMembers failed: %v", err)
	}
	for _, userID := range []uint{1, 2, 3} {
		found := false
		for _, event := range events[userID] {
			if event["type"] == "e2ee_group_key_changed" && event["key_version"] == 2 {
				found = true
			}
		}
		if !found {
			t.Fatalf("user %d did not receive group key version 2 event: %#v", userID, events[userID])
		}
	}
}

func TestGroupServiceDeleteAndList(t *testing.T) {
	ctx := context.Background()
	users := newFakeUserRepo(testUser(1, "1", ""), testUser(2, "2", ""))
	groups := newFakeGroupRepo()
	groups.groups[10] = &model.ChatGroup{ID: 10, OwnerID: 1, Name: "g"}
	groups.members[10] = map[uint]*model.ChatGroupMember{
		1: {GroupID: 10, UserID: 1, Role: "owner"},
		2: {GroupID: 10, UserID: 2, Role: "member"},
	}
	svc := NewGroupService(groups, newFakeFriendRepo(), users, nil, nil)

	list, err := svc.GetGroups(ctx, 2)
	if err != nil {
		t.Fatalf("GetGroups failed: %v", err)
	}
	if len(list) != 1 || list[0].MemberCount != 2 {
		t.Fatalf("GetGroups = %#v, want one group with two members", list)
	}

	if err := svc.DeleteGroup(ctx, 2, 10); !errors.Is(err, ErrGroupDeleteDenied) {
		t.Fatalf("non owner delete error = %v, want ErrGroupDeleteDenied", err)
	}
	if err := svc.DeleteGroup(ctx, 1, 404); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("missing group delete error = %v, want ErrGroupNotFound", err)
	}
	if err := svc.DeleteGroup(ctx, 1, 10); err != nil {
		t.Fatalf("DeleteGroup failed: %v", err)
	}
	if _, err := groups.GetByID(ctx, 10); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("group after delete error = %v, want not found", err)
	}
}
