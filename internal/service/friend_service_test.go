package service

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"sleet0922/graduation_project/internal/model"
)

func TestFriendServiceSendFriendRequest(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func(*fakeFriendRepo)
		from    uint
		to      uint
		wantErr error
	}{
		{name: "cannot add self", from: 1, to: 1, wantErr: ErrCannotAddSelf},
		{
			name: "already friends",
			setup: func(repo *fakeFriendRepo) {
				repo.friendships[[2]uint{1, 2}] = true
			},
			from:    1,
			to:      2,
			wantErr: ErrAlreadyFriend,
		},
		{
			name: "pending request exists in either direction",
			setup: func(repo *fakeFriendRepo) {
				repo.requestExists[[2]uint{2, 1}] = true
			},
			from:    1,
			to:      2,
			wantErr: ErrRequestExists,
		},
		{name: "creates pending request", from: 1, to: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			friendRepo := newFakeFriendRepo()
			if tt.setup != nil {
				tt.setup(friendRepo)
			}
			svc := NewFriendService(friendRepo, newFakeUserRepo(), nil)

			err := svc.SendFriendRequest(ctx, tt.from, tt.to)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("SendFriendRequest error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && friendRepo.lastRequest == nil {
				t.Fatal("SendFriendRequest did not persist request")
			}
		})
	}
}

func TestFriendServiceSendFriendRequestByAccount(t *testing.T) {
	ctx := context.Background()
	userRepo := newFakeUserRepo(
		testUser(2, "1000000002", "friend@example.com"),
	)
	friendRepo := newFakeFriendRepo()
	svc := NewFriendService(friendRepo, userRepo, nil)

	receiverID, err := svc.SendFriendRequestByAccount(ctx, 1, "friend@example.com")
	if err != nil {
		t.Fatalf("SendFriendRequestByAccount email failed: %v", err)
	}
	if receiverID != 2 {
		t.Fatalf("receiverID = %d, want 2", receiverID)
	}

	_, err = svc.SendFriendRequestByAccount(ctx, 1, "missing")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("missing account error = %v, want ErrUserNotFound", err)
	}
}

func TestFriendServiceHandleFriendRequest(t *testing.T) {
	ctx := context.Background()

	t.Run("validates status before querying repository", func(t *testing.T) {
		svc := NewFriendService(newFakeFriendRepo(), newFakeUserRepo(), nil)
		err := svc.HandleFriendRequest(ctx, 2, 1, 9)
		if !errors.Is(err, ErrInvalidFriendRequestStatus) {
			t.Fatalf("HandleFriendRequest error = %v, want ErrInvalidFriendRequestStatus", err)
		}
	})

	t.Run("rejects non receiver", func(t *testing.T) {
		repo := newFakeFriendRepo()
		repo.requests[1] = &model.FriendRequest{Model: gorm.Model{ID: 1}, SenderID: 1, ReceiverID: 2, Status: 0}
		svc := NewFriendService(repo, newFakeUserRepo(), nil)

		err := svc.HandleFriendRequest(ctx, 3, 1, 1)
		if !errors.Is(err, ErrFriendRequestPermission) {
			t.Fatalf("HandleFriendRequest error = %v, want ErrFriendRequestPermission", err)
		}
	})

	t.Run("accepts pending request", func(t *testing.T) {
		repo := newFakeFriendRepo()
		repo.requests[1] = &model.FriendRequest{Model: gorm.Model{ID: 1}, SenderID: 1, ReceiverID: 2, Status: 0}
		svc := NewFriendService(repo, newFakeUserRepo(), nil)

		if err := svc.HandleFriendRequest(ctx, 2, 1, 1); err != nil {
			t.Fatalf("HandleFriendRequest accept failed: %v", err)
		}
		if !repo.CheckFriendship(ctx, 1, 2) {
			t.Fatal("accepting request did not create friendship")
		}
	})

	t.Run("rejects pending request", func(t *testing.T) {
		repo := newFakeFriendRepo()
		repo.requests[1] = &model.FriendRequest{Model: gorm.Model{ID: 1}, SenderID: 1, ReceiverID: 2, Status: 0}
		svc := NewFriendService(repo, newFakeUserRepo(), nil)

		if err := svc.HandleFriendRequest(ctx, 2, 1, 2); err != nil {
			t.Fatalf("HandleFriendRequest reject failed: %v", err)
		}
		if repo.requests[1].Status != 2 {
			t.Fatalf("request status = %d, want 2", repo.requests[1].Status)
		}
	})

	t.Run("missing request bubbles not found", func(t *testing.T) {
		svc := NewFriendService(newFakeFriendRepo(), newFakeUserRepo(), nil)
		err := svc.HandleFriendRequest(ctx, 2, 404, 1)
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("HandleFriendRequest error = %v, want gorm.ErrRecordNotFound", err)
		}
	})
}
