package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type FeedHandler struct {
	feedService service.FeedService
}

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 50
)

// parsePagination validates query values at the HTTP boundary. The service
// layer still normalizes direct callers, but malformed client input must not
// silently turn into a different page or be echoed back as if it were valid.
func parsePagination(c *fiber.Ctx) (int, int, error) {
	page, err := strconv.Atoi(c.Query("page", strconv.Itoa(defaultPage)))
	if err != nil || page < 1 {
		return 0, 0, fmt.Errorf("page must be a positive integer")
	}
	pageSize, err := strconv.Atoi(c.Query("page_size", strconv.Itoa(defaultPageSize)))
	if err != nil || pageSize < 1 || pageSize > maxPageSize {
		return 0, 0, fmt.Errorf("page_size must be between 1 and %d", maxPageSize)
	}
	return page, pageSize, nil
}

func NewFeedHandler(feedService service.FeedService) *FeedHandler {
	return &FeedHandler{feedService: feedService}
}

// ===================== 创建动态 ====================
func (h *FeedHandler) CreatePost(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	type MediaInput struct {
		MediaType int    `json:"media_type"`
		MediaURL  string `json:"media_url"`
		SortOrder int    `json:"sort_order"`
	}

	var req struct {
		Content string       `json:"content"`
		Media   []MediaInput `json:"media"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}

	if req.Content == "" && len(req.Media) == 0 {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}

	mediaInputs := make([]service.CreateMediaInput, len(req.Media))
	for i, m := range req.Media {
		mediaInputs[i] = service.CreateMediaInput{
			MediaType: model.MediaType(m.MediaType),
			MediaURL:  m.MediaURL,
			SortOrder: m.SortOrder,
		}
	}

	post, err := h.feedService.CreatePost(c.Context(), userID, req.Content, mediaInputs)
	if err != nil {
		slog.Error("创建动态失败", "error", err, "user_id", userID)
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}

	return response.Success(c, post, "发布成功")
}

// ===================== 删除动态 ====================
func (h *FeedHandler) DeletePost(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	var req struct {
		PostID uint `json:"post_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.PostID == 0 {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}

	if err := h.feedService.DeletePost(c.Context(), userID, req.PostID); err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			return response.Result(c, http.StatusNotFound, errcode.NotFound, nil)
		}
		if errors.Is(err, service.ErrNotPostOwner) {
			return response.Result(c, http.StatusForbidden, errcode.Forbidden, nil)
		}
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}

	return response.Success(c, nil, "删除成功")
}

// ===================== 动态详情 ====================
func (h *FeedHandler) GetDetail(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	postIDStr := c.Query("post_id")
	if postIDStr == "" {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}
	postID, err := strconv.ParseUint(postIDStr, 10, 64)
	if err != nil {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}

	result, err := h.feedService.GetPostDetail(c.Context(), userID, uint(postID))
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			return response.Result(c, http.StatusNotFound, errcode.NotFound, nil)
		}
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}

	return response.Success(c, fiber.Map{
		"post":     result.Post,
		"is_liked": result.IsLiked,
	}, "获取动态详情成功")
}

// ===================== 动态列表（朋友圈） ====================
func (h *FeedHandler) ListFeed(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	page, pageSize, err := parsePagination(c)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, err.Error())
	}

	postsWithLiked, total, err := h.feedService.ListFeed(c.Context(), userID, page, pageSize)
	if err != nil {
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}

	// 构建带 is_liked 的列表
	type postItem struct {
		*model.FeedPost
		IsLiked bool `json:"is_liked"`
	}
	items := make([]postItem, len(postsWithLiked))
	for i, p := range postsWithLiked {
		items[i] = postItem{FeedPost: p.Post, IsLiked: p.IsLiked}
	}

	return response.Success(c, fiber.Map{
		"list":      items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}, "获取动态列表成功")
}

// ===================== 我的动态 ====================
func (h *FeedHandler) ListMyPosts(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	page, pageSize, err := parsePagination(c)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, err.Error())
	}

	postsWithLiked, total, err := h.feedService.ListMyPosts(c.Context(), userID, page, pageSize)
	if err != nil {
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}

	// 构建带 is_liked 的列表
	type postItem struct {
		*model.FeedPost
		IsLiked bool `json:"is_liked"`
	}
	items := make([]postItem, len(postsWithLiked))
	for i, p := range postsWithLiked {
		items[i] = postItem{FeedPost: p.Post, IsLiked: p.IsLiked}
	}

	return response.Success(c, fiber.Map{
		"list":      items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}, "获取我的动态成功")
}

// ===================== 点赞/取消点赞（Toggle） ====================
func (h *FeedHandler) ToggleLike(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	var req struct {
		PostID uint `json:"post_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.PostID == 0 {
		slog.Error("like bind error", "error", err)
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}

	liked, err := h.feedService.ToggleLike(c.Context(), userID, req.PostID)
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			return response.Result(c, http.StatusNotFound, errcode.NotFound, nil)
		}
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}

	msg := "取消点赞"
	if liked {
		msg = "点赞成功"
	}
	return response.Success(c, fiber.Map{
		"is_liked": liked,
	}, msg)
}

// ===================== 发表评论 ====================
func (h *FeedHandler) CreateComment(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	var req struct {
		PostID    uint   `json:"post_id"`
		Content   string `json:"content"`
		ReplyToID *uint  `json:"reply_to_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.PostID == 0 || req.Content == "" {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}

	comment, err := h.feedService.CreateComment(c.Context(), userID, req.PostID, req.Content, req.ReplyToID)
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			return response.Result(c, http.StatusNotFound, errcode.NotFound, nil)
		}
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}

	return response.Success(c, comment, "评论成功")
}

// ===================== 删除评论 ====================
func (h *FeedHandler) DeleteComment(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	var req struct {
		CommentID uint `json:"comment_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.CommentID == 0 {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}

	if err := h.feedService.DeleteComment(c.Context(), userID, req.CommentID); err != nil {
		if errors.Is(err, service.ErrCommentNotFound) || errors.Is(err, service.ErrPostNotFound) {
			return response.Result(c, http.StatusNotFound, errcode.NotFound, nil)
		}
		if errors.Is(err, service.ErrNotPostOwner) {
			return response.Result(c, http.StatusForbidden, errcode.Forbidden, nil)
		}
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}

	return response.Success(c, nil, "删除评论成功")
}

// ===================== 评论列表 ====================
func (h *FeedHandler) ListComments(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	postIDStr := c.Query("post_id")
	if postIDStr == "" {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}
	postID, err := strconv.ParseUint(postIDStr, 10, 64)
	if err != nil {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}

	page, pageSize, err := parsePagination(c)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, err.Error())
	}

	comments, total, err := h.feedService.ListComments(c.Context(), userID, uint(postID), page, pageSize)
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			return response.Result(c, http.StatusNotFound, errcode.NotFound, nil)
		}
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}

	return response.Success(c, fiber.Map{
		"list":      comments,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}, "获取评论列表成功")
}

// ===================== 查询点赞状态 ====================
func (h *FeedHandler) IsLiked(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	postIDStr := c.Query("post_id")
	if postIDStr == "" {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}
	postID, err := strconv.ParseUint(postIDStr, 10, 64)
	if err != nil {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}

	liked, err := h.feedService.IsLiked(c.Context(), userID, uint(postID))
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			return response.Result(c, http.StatusNotFound, errcode.NotFound, nil)
		}
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}

	return response.Success(c, fiber.Map{
		"is_liked": liked,
	}, "查询成功")
}
