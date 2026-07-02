package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"

	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
)

type fakeFeedService struct {
	createPostFn   func(context.Context, uint, string, []service.CreateMediaInput) (*model.FeedPost, error)
	deletePostFn   func(context.Context, uint, uint) error
	getDetailFn    func(context.Context, uint, uint) (*service.FeedPostWithLiked, error)
	toggleLikeFn   func(context.Context, uint, uint) (bool, error)
	createCommentFn func(context.Context, uint, uint, string, *uint) (*model.FeedComment, error)
	listCommentsFn func(context.Context, uint, uint, int, int) ([]model.FeedComment, int64, error)
}

func (s *fakeFeedService) CreatePost(ctx context.Context, userID uint, content string, media []service.CreateMediaInput) (*model.FeedPost, error) {
	if s.createPostFn != nil {
		return s.createPostFn(ctx, userID, content, media)
	}
	return &model.FeedPost{BaseModel: model.BaseModel{ID: 1}, UserID: userID, Content: content}, nil
}

func (s *fakeFeedService) DeletePost(ctx context.Context, userID, postID uint) error {
	if s.deletePostFn != nil {
		return s.deletePostFn(ctx, userID, postID)
	}
	return nil
}

func (s *fakeFeedService) GetPostDetail(ctx context.Context, userID, postID uint) (*service.FeedPostWithLiked, error) {
	if s.getDetailFn != nil {
		return s.getDetailFn(ctx, userID, postID)
	}
	return &service.FeedPostWithLiked{Post: &model.FeedPost{BaseModel: model.BaseModel{ID: postID}}, IsLiked: true}, nil
}

func (s *fakeFeedService) ListFeed(ctx context.Context, userID uint, page, pageSize int) ([]service.FeedPostWithLiked, int64, error) {
	return []service.FeedPostWithLiked{{Post: &model.FeedPost{BaseModel: model.BaseModel{ID: 1}}, IsLiked: false}}, 1, nil
}

func (s *fakeFeedService) ListMyPosts(ctx context.Context, userID uint, page, pageSize int) ([]service.FeedPostWithLiked, int64, error) {
	return s.ListFeed(ctx, userID, page, pageSize)
}

func (s *fakeFeedService) ToggleLike(ctx context.Context, userID, postID uint) (bool, error) {
	if s.toggleLikeFn != nil {
		return s.toggleLikeFn(ctx, userID, postID)
	}
	return true, nil
}

func (s *fakeFeedService) IsLiked(ctx context.Context, userID, postID uint) (bool, error) {
	return postID == 1, nil
}

func (s *fakeFeedService) CreateComment(ctx context.Context, userID, postID uint, content string, replyToID *uint) (*model.FeedComment, error) {
	if s.createCommentFn != nil {
		return s.createCommentFn(ctx, userID, postID, content, replyToID)
	}
	return &model.FeedComment{BaseModel: model.BaseModel{ID: 1}, UserID: userID, PostID: postID, Content: content}, nil
}

func (s *fakeFeedService) DeleteComment(ctx context.Context, userID, commentID uint) error {
	return nil
}

func (s *fakeFeedService) ListComments(ctx context.Context, userID, postID uint, page, pageSize int) ([]model.FeedComment, int64, error) {
	if s.listCommentsFn != nil {
		return s.listCommentsFn(ctx, userID, postID, page, pageSize)
	}
	return []model.FeedComment{{BaseModel: model.BaseModel{ID: 1}, PostID: postID, UserID: userID, Content: "hi"}}, 1, nil
}

func TestFeedHandlerCreatePostAndDetail(t *testing.T) {
	handler := NewFeedHandler(&fakeFeedService{})
	app := fiber.New()
	app.Post("/feed", withUser(7, handler.CreatePost))
	app.Get("/detail", withUser(7, handler.GetDetail))

	status, payload := testResponse(t, app, testJSONRequest("POST", "/feed", map[string]any{"content": "hello"}))
	if status != http.StatusOK || int(payload["code"].(float64)) != errcode.Success {
		t.Fatalf("create post response = status %d payload %#v", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/feed", map[string]any{}))
	if status != http.StatusBadRequest || int(payload["code"].(float64)) != errcode.InvalidParams {
		t.Fatalf("empty post response = status %d payload %#v", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("GET", "/detail?post_id=1", nil))
	if status != http.StatusOK || int(payload["code"].(float64)) != errcode.Success {
		t.Fatalf("detail response = status %d payload %#v", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("GET", "/detail?post_id=bad", nil))
	if status != http.StatusBadRequest {
		t.Fatalf("bad detail response = status %d payload %#v, want 400", status, payload)
	}
}

func TestFeedHandlerErrorMappings(t *testing.T) {
	handler := NewFeedHandler(&fakeFeedService{
		deletePostFn: func(ctx context.Context, userID, postID uint) error {
			if postID == 1 {
				return service.ErrPostNotFound
			}
			return service.ErrNotPostOwner
		},
		toggleLikeFn: func(ctx context.Context, userID, postID uint) (bool, error) {
			return false, service.ErrPostNotFound
		},
		listCommentsFn: func(ctx context.Context, userID, postID uint, page, pageSize int) ([]model.FeedComment, int64, error) {
			return nil, 0, service.ErrPostNotFound
		},
	})
	app := fiber.New()
	app.Delete("/feed", withUser(7, handler.DeletePost))
	app.Post("/like", withUser(7, handler.ToggleLike))
	app.Get("/comments", withUser(7, handler.ListComments))

	status, _ := testResponse(t, app, testJSONRequest("DELETE", "/feed", map[string]any{"post_id": 1}))
	if status != http.StatusNotFound {
		t.Fatalf("delete missing post status = %d, want 404", status)
	}
	status, _ = testResponse(t, app, testJSONRequest("DELETE", "/feed", map[string]any{"post_id": 2}))
	if status != http.StatusForbidden {
		t.Fatalf("delete non owner status = %d, want 403", status)
	}
	status, _ = testResponse(t, app, testJSONRequest("POST", "/like", map[string]any{"post_id": 1}))
	if status != http.StatusNotFound {
		t.Fatalf("toggle missing post status = %d, want 404", status)
	}
	status, _ = testResponse(t, app, testJSONRequest("GET", "/comments?post_id=1", nil))
	if status != http.StatusNotFound {
		t.Fatalf("comments missing post status = %d, want 404", status)
	}

	handler = NewFeedHandler(&fakeFeedService{
		createCommentFn: func(ctx context.Context, userID, postID uint, content string, replyToID *uint) (*model.FeedComment, error) {
			return nil, errors.New("db")
		},
	})
	app = fiber.New()
	app.Post("/comment", withUser(7, handler.CreateComment))
	status, payload := testResponse(t, app, testJSONRequest("POST", "/comment", map[string]any{"post_id": 1, "content": "hi"}))
	if status != http.StatusInternalServerError || int(payload["code"].(float64)) != errcode.InternalServerError {
		t.Fatalf("comment db error response = status %d payload %#v", status, payload)
	}
}
