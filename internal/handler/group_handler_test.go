package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"

	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
)

type fakeGroupService struct {
	err error
}

func (s *fakeGroupService) CreateGroup(context.Context, uint, string, string, []uint) (*model.ChatGroupDetail, error) {
	return nil, s.err
}

func (s *fakeGroupService) AddMembers(context.Context, uint, uint, []uint) ([]*model.ChatGroupMemberDetail, error) {
	return nil, s.err
}

func (s *fakeGroupService) RemoveMember(context.Context, uint, uint, uint) error {
	return s.err
}

func (s *fakeGroupService) LeaveGroup(context.Context, uint, uint) error {
	return s.err
}

func (s *fakeGroupService) DeleteGroup(context.Context, uint, uint) error {
	return s.err
}

func (s *fakeGroupService) GetGroups(context.Context, uint) ([]*model.ChatGroupDetail, error) {
	return nil, s.err
}

func (s *fakeGroupService) GetMembers(context.Context, uint, uint) ([]*model.ChatGroupMemberDetail, error) {
	return nil, s.err
}

func TestGroupHandlerErrorMappings(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		target     string
		method     string
		body       any
		err        error
		wantStatus int
	}{
		{name: "create permission", path: "/create", target: "/create", method: http.MethodPost, body: map[string]any{"name": "group", "member_ids": []uint{2}}, err: service.ErrGroupPermission, wantStatus: http.StatusForbidden},
		{name: "add missing group", path: "/add", target: "/add", method: http.MethodPost, body: map[string]any{"group_id": 1, "member_ids": []uint{2}}, err: service.ErrGroupNotFound, wantStatus: http.StatusNotFound},
		{name: "remove denied", path: "/remove", target: "/remove", method: http.MethodPost, body: map[string]any{"group_id": 1, "member_id": 2}, err: service.ErrGroupKickDenied, wantStatus: http.StatusForbidden},
		{name: "leave denied", path: "/leave", target: "/leave", method: http.MethodPost, body: map[string]any{"group_id": 1}, err: service.ErrGroupLeaveDenied, wantStatus: http.StatusForbidden},
		{name: "delete missing group", path: "/delete", target: "/delete", method: http.MethodPost, body: map[string]any{"group_id": 1}, err: service.ErrGroupNotFound, wantStatus: http.StatusNotFound},
		{name: "groups unexpected", path: "/groups", target: "/groups", method: http.MethodGet, err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError},
		{name: "members missing member", path: "/members", target: "/members?group_id=1", method: http.MethodGet, err: service.ErrGroupMemberNotFound, wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewGroupHandler(&fakeGroupService{err: tt.err})
			app := fiber.New()
			withUser := func(next fiber.Handler) fiber.Handler {
				return func(c *fiber.Ctx) error { c.Locals("user_id", uint(7)); return next(c) }
			}
			switch tt.method {
			case http.MethodGet:
				if tt.path == "/members" {
					app.Get(tt.path, withUser(handler.GetMembers))
				} else {
					app.Get(tt.path, withUser(handler.GetGroups))
				}
			default:
				register := map[string]fiber.Handler{
					"/create": handler.Create,
					"/add":    handler.AddMembers,
					"/remove": handler.RemoveMember,
					"/leave":  handler.Leave,
					"/delete": handler.Delete,
				}[tt.path]
				app.Add(tt.method, tt.path, withUser(register))
			}
			status, _ := testResponse(t, app, testJSONRequest(tt.method, tt.target, tt.body))
			if status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", status, tt.wantStatus)
			}
		})
	}
}
