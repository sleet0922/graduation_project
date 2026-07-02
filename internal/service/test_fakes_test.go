package service

import (
	"context"
	"errors"
	"sort"
	"sync"

	"gorm.io/gorm"

	"sleet0922/graduation_project/internal/model"
)

func testUser(id uint, account, email string) *model.User {
	return &model.User{
		Model:   gorm.Model{ID: id},
		Account: account,
		Email:   email,
		Name:    account,
	}
}

type fakeUserRepo struct {
	mu        sync.Mutex
	byID      map[uint]*model.User
	byEmail   map[string]*model.User
	byAccount map[string]*model.User
	nextID    uint
	err       error
}

func newFakeUserRepo(users ...*model.User) *fakeUserRepo {
	r := &fakeUserRepo{
		byID:      make(map[uint]*model.User),
		byEmail:   make(map[string]*model.User),
		byAccount: make(map[string]*model.User),
		nextID:    1,
	}
	for _, user := range users {
		r.addSeed(user)
	}
	return r
}

func cloneUser(user *model.User) *model.User {
	if user == nil {
		return nil
	}
	copied := *user
	return &copied
}

func (r *fakeUserRepo) addSeed(user *model.User) {
	if user.ID == 0 {
		user.ID = r.nextID
	}
	if user.ID >= r.nextID {
		r.nextID = user.ID + 1
	}
	copied := cloneUser(user)
	r.byID[copied.ID] = copied
	r.byEmail[copied.Email] = copied
	r.byAccount[copied.Account] = copied
}

func (r *fakeUserRepo) Add(ctx context.Context, user *model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	if _, ok := r.byEmail[user.Email]; ok {
		return gorm.ErrDuplicatedKey
	}
	if user.ID == 0 {
		user.ID = r.nextID
		r.nextID++
	}
	r.addSeed(user)
	return nil
}

func (r *fakeUserRepo) Delete(ctx context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	user, ok := r.byID[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	delete(r.byID, id)
	delete(r.byEmail, user.Email)
	delete(r.byAccount, user.Account)
	return nil
}

func (r *fakeUserRepo) Update(ctx context.Context, user *model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.addSeed(user)
	return nil
}

func (r *fakeUserRepo) GetByID(ctx context.Context, id uint) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	user, ok := r.byID[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return cloneUser(user), nil
}

func (r *fakeUserRepo) GetByAccount(ctx context.Context, account string) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	user, ok := r.byAccount[account]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return cloneUser(user), nil
}

func (r *fakeUserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	user, ok := r.byEmail[email]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return cloneUser(user), nil
}

func (r *fakeUserRepo) UpdateAvatar(ctx context.Context, userID uint, avatar string) (*model.User, error) {
	return r.updateUser(userID, func(user *model.User) { user.Avatar = avatar })
}

func (r *fakeUserRepo) UpdateName(ctx context.Context, userID uint, name string) (*model.User, error) {
	return r.updateUser(userID, func(user *model.User) { user.Name = name })
}

func (r *fakeUserRepo) UpdatePassword(ctx context.Context, userID uint, password string) (*model.User, error) {
	return r.updateUser(userID, func(user *model.User) { user.Password = password })
}

func (r *fakeUserRepo) UpdateProfile(ctx context.Context, userID uint, gender int, birthday string, location string) (*model.User, error) {
	return r.updateUser(userID, func(user *model.User) {
		user.Gender = gender
		user.Birthday = birthday
		user.Location = location
	})
}

func (r *fakeUserRepo) GetSelf(ctx context.Context, userID uint) (*model.User, error) {
	return r.GetByID(ctx, userID)
}

func (r *fakeUserRepo) UpsertLocation(ctx context.Context, location *model.UserLocation) error {
	if r.err != nil {
		return r.err
	}
	if location == nil || location.UserID == 0 {
		return errors.New("invalid location")
	}
	return nil
}

func (r *fakeUserRepo) updateUser(userID uint, mutate func(*model.User)) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	user, ok := r.byID[userID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	mutate(user)
	return cloneUser(user), nil
}

type fakeFriendRepo struct {
	mu                 sync.Mutex
	friendships        map[[2]uint]bool
	requestExists      map[[2]uint]bool
	requests           map[uint]*model.FriendRequest
	nextRequestID      uint
	lastRequest        *model.FriendRequest
	acceptedRequestIDs []uint
	err                error
}

func newFakeFriendRepo() *fakeFriendRepo {
	return &fakeFriendRepo{
		friendships:   make(map[[2]uint]bool),
		requestExists: make(map[[2]uint]bool),
		requests:      make(map[uint]*model.FriendRequest),
		nextRequestID: 1,
	}
}

func (r *fakeFriendRepo) Create(ctx context.Context, friend *model.Friend) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.friendships[[2]uint{friend.UserID, friend.FriendID}] = true
	return r.err
}

func (r *fakeFriendRepo) Delete(ctx context.Context, friend *model.Friend) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.friendships, [2]uint{friend.UserID, friend.FriendID})
	return r.err
}

func (r *fakeFriendRepo) GetByUserID(ctx context.Context, userID uint) ([]*model.Friend, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	var friends []*model.Friend
	for pair := range r.friendships {
		if pair[0] == userID {
			friends = append(friends, &model.Friend{UserID: pair[0], FriendID: pair[1]})
		}
	}
	sort.Slice(friends, func(i, j int) bool { return friends[i].FriendID < friends[j].FriendID })
	return friends, nil
}

func (r *fakeFriendRepo) GetFriendDetailsByUserID(ctx context.Context, userID uint) ([]*model.FriendDetail, error) {
	if r.err != nil {
		return nil, r.err
	}
	return nil, nil
}

func (r *fakeFriendRepo) CheckFriendship(ctx context.Context, userID uint, friendID uint) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.friendships[[2]uint{userID, friendID}] || r.friendships[[2]uint{friendID, userID}]
}

func (r *fakeFriendRepo) SendFriendRequest(ctx context.Context, friendRequest *model.FriendRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	if friendRequest.ID == 0 {
		friendRequest.ID = r.nextRequestID
		r.nextRequestID++
	}
	copied := *friendRequest
	r.lastRequest = &copied
	r.requests[copied.ID] = &copied
	r.requestExists[[2]uint{copied.SenderID, copied.ReceiverID}] = true
	return nil
}

func (r *fakeFriendRepo) CheckRequestExists(ctx context.Context, senderID, receiverID uint) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return false, r.err
	}
	return r.requestExists[[2]uint{senderID, receiverID}] || r.requestExists[[2]uint{receiverID, senderID}], nil
}

func (r *fakeFriendRepo) GetRequestByID(ctx context.Context, requestID uint) (*model.FriendRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	request, ok := r.requests[requestID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copied := *request
	return &copied, nil
}

func (r *fakeFriendRepo) UpdateRequestStatus(ctx context.Context, request *model.FriendRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	copied := *request
	r.requests[copied.ID] = &copied
	return nil
}

func (r *fakeFriendRepo) GetRequestsByReceiverID(ctx context.Context, receiverID uint) ([]*model.FriendRequest, error) {
	if r.err != nil {
		return nil, r.err
	}
	return nil, nil
}

func (r *fakeFriendRepo) AcceptFriendRequest(ctx context.Context, request *model.FriendRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	request.Status = 1
	copied := *request
	r.requests[copied.ID] = &copied
	r.friendships[[2]uint{request.SenderID, request.ReceiverID}] = true
	r.friendships[[2]uint{request.ReceiverID, request.SenderID}] = true
	r.acceptedRequestIDs = append(r.acceptedRequestIDs, request.ID)
	return nil
}

func (r *fakeFriendRepo) RemoveBothFriends(ctx context.Context, userID, friendID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.friendships, [2]uint{userID, friendID})
	delete(r.friendships, [2]uint{friendID, userID})
	return r.err
}

func (r *fakeFriendRepo) UpdateRemark(ctx context.Context, userID, friendID uint, remark string) error {
	return r.err
}

type fakeGroupRepo struct {
	mu            sync.Mutex
	groups        map[uint]*model.ChatGroup
	members       map[uint]map[uint]*model.ChatGroupMember
	nextGroupID   uint
	deletedGroups []uint
	err           error
}

func newFakeGroupRepo() *fakeGroupRepo {
	return &fakeGroupRepo{
		groups:      make(map[uint]*model.ChatGroup),
		members:     make(map[uint]map[uint]*model.ChatGroupMember),
		nextGroupID: 1,
	}
}

func (r *fakeGroupRepo) Create(ctx context.Context, group *model.ChatGroup, members []*model.ChatGroupMember) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	if group.ID == 0 {
		group.ID = r.nextGroupID
		r.nextGroupID++
	}
	copiedGroup := *group
	r.groups[group.ID] = &copiedGroup
	if r.members[group.ID] == nil {
		r.members[group.ID] = make(map[uint]*model.ChatGroupMember)
	}
	for _, member := range members {
		member.GroupID = group.ID
		copiedMember := *member
		r.members[group.ID][member.UserID] = &copiedMember
	}
	return nil
}

func (r *fakeGroupRepo) AddMembers(ctx context.Context, groupID uint, members []*model.ChatGroupMember) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	if r.members[groupID] == nil {
		r.members[groupID] = make(map[uint]*model.ChatGroupMember)
	}
	for _, member := range members {
		member.GroupID = groupID
		copied := *member
		r.members[groupID][member.UserID] = &copied
	}
	return nil
}

func (r *fakeGroupRepo) RemoveMember(ctx context.Context, groupID, userID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	delete(r.members[groupID], userID)
	return nil
}

func (r *fakeGroupRepo) DeleteGroup(ctx context.Context, groupID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	delete(r.groups, groupID)
	delete(r.members, groupID)
	r.deletedGroups = append(r.deletedGroups, groupID)
	return nil
}

func (r *fakeGroupRepo) GetByID(ctx context.Context, groupID uint) (*model.ChatGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	group, ok := r.groups[groupID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copied := *group
	return &copied, nil
}

func (r *fakeGroupRepo) GetGroupsByUserID(ctx context.Context, userID uint) ([]*model.ChatGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	var groups []*model.ChatGroup
	for groupID, memberMap := range r.members {
		if _, ok := memberMap[userID]; ok {
			copied := *r.groups[groupID]
			groups = append(groups, &copied)
		}
	}
	return groups, nil
}

func (r *fakeGroupRepo) GetMembersByGroupID(ctx context.Context, groupID uint) ([]*model.ChatGroupMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	members := make([]*model.ChatGroupMember, 0, len(r.members[groupID]))
	for _, member := range r.members[groupID] {
		copied := *member
		members = append(members, &copied)
	}
	sort.Slice(members, func(i, j int) bool { return members[i].UserID < members[j].UserID })
	return members, nil
}

func (r *fakeGroupRepo) CountMembers(ctx context.Context, groupID uint) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return 0, r.err
	}
	return int64(len(r.members[groupID])), nil
}

func (r *fakeGroupRepo) IsMember(ctx context.Context, groupID, userID uint) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.members[groupID][userID]
	return ok
}
