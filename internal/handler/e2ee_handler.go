package handler

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/logger"
	"sleet0922/graduation_project/pkg/response"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type E2EEHandler struct {
	e2eeService service.E2EEService
}

type publishGroupKeyBoxesRequest struct {
	GroupID    uint                        `json:"group_id"`
	KeyVersion int                         `json:"key_version"`
	KeyWrapAlg string                      `json:"key_wrap_alg"`
	Boxes      []publishGroupKeyBoxPayload `json:"boxes"`
}

type publishGroupKeyBoxPayload struct {
	UserID          uint   `json:"user_id"`
	WrappedGroupKey string `json:"wrapped_group_key"`
	WrapNonce       string `json:"wrap_nonce"`
}

type rotateGroupKeyRequest struct {
	GroupID            uint `json:"group_id"`
	ExpectedKeyVersion int  `json:"expected_key_version"`
}

func NewE2EEHandler(e2eeService service.E2EEService) *E2EEHandler {
	return &E2EEHandler{e2eeService: e2eeService}
}

// ----------E2EE handler 工具函数----------
func parseUintQuery(raw string) (uint, error) {
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		return 0, fmt.Errorf("invalid uint query")
	}
	return uint(v), nil
}

func parseIntQuery(raw string) (int, error) {
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (h *E2EEHandler) handleGroupKeyError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrE2EEGroupPermission):
		return response.Error(c, http.StatusForbidden, "你不在该群聊中")
	case errors.Is(err, service.ErrE2EEGroupKeyNotFound):
		return response.Error(c, http.StatusNotFound, "group key not initialized")
	case errors.Is(err, service.ErrE2EEGroupVersionAbsent):
		return response.Error(c, http.StatusNotFound, "e2ee group key version not found")
	case errors.Is(err, service.ErrE2EEGroupKeyBoxMissing):
		return response.Error(c, http.StatusNotFound, "e2ee group key box not found")
	case errors.Is(err, service.ErrE2EEGroupVersionLock):
		return response.Error(c, http.StatusConflict, "e2ee group key version conflict")
	case errors.Is(err, service.ErrE2EEGroupBoxesPublished):
		return response.Error(c, http.StatusConflict, "e2ee group key boxes already published")
	case errors.Is(err, service.ErrE2EEGroupBoxesInvalid):
		return response.Error(c, http.StatusBadRequest, "invalid e2ee group key boxes payload")
	default:
		return response.Error(c, http.StatusInternalServerError, "服务端异常")
	}
}

func decodedLenBase64URLOrStd(raw string) (int, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, "empty"
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(raw); err == nil {
		return len(decoded), ""
	}
	if decoded, err := base64.URLEncoding.DecodeString(raw); err == nil {
		return len(decoded), ""
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(raw); err == nil {
		return len(decoded), ""
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return len(decoded), ""
	}
	return 0, "invalid_base64"
}

func maskToken(raw string) string {
	if raw == "" {
		return ""
	}
	if len(raw) <= 12 {
		return raw
	}
	return raw[:6] + "..." + raw[len(raw)-6:]
}

// ----------E2EE handler 方法----------

// 发布用户公钥
func (h *E2EEHandler) PublishPublicKey(c *fiber.Ctx) error {
	type publishKeyRequest struct {
		KeyType   string `json:"key_type"`
		PublicKey string `json:"public_key"`
	}

	var req publishKeyRequest
	if err := c.BodyParser(&req); err != nil || req.KeyType == "" || req.PublicKey == "" {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	key, err := h.e2eeService.PublishUserPublicKey(c.Context(), userID, req.KeyType, req.PublicKey)
	if err != nil {
		if errors.Is(err, service.ErrUnsupportedE2EEKeyType) || errors.Is(err, service.ErrInvalidE2EEPublicKey) {
			return response.Error(c, http.StatusBadRequest, err.Error())
		}
		return response.Error(c, http.StatusInternalServerError, "服务端异常")
	}

	return response.Success(c, fiber.Map{
		"user_id":    key.UserID,
		"key_type":   key.KeyType,
		"updated_at": key.UpdatedAt.UTC().Format(time.RFC3339),
	}, "ok")
}

// 获取用户公钥
func (h *E2EEHandler) GetPublicKey(c *fiber.Ctx) error {
	userIDText := c.Query("user_id")
	parsedID, err := strconv.ParseUint(userIDText, 10, 64)
	if err != nil || parsedID == 0 {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	key, err := h.e2eeService.GetUserPublicKey(c.Context(), uint(parsedID))
	if err != nil {
		if errors.Is(err, service.ErrE2EEPublicKeyNotFound) {
			return response.Error(c, http.StatusNotFound, err.Error())
		}
		return response.Error(c, http.StatusInternalServerError, "服务端异常")
	}

	return response.Success(c, fiber.Map{
		"user_id":    key.UserID,
		"key_type":   key.KeyType,
		"public_key": key.PublicKey,
		"updated_at": key.UpdatedAt.UTC().Format(time.RFC3339),
	}, "ok")
}

func (h *E2EEHandler) GetGroupCurrentKey(c *fiber.Ctx) error {
	currentUserID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}
	groupID, err := parseUintQuery(c.Query("group_id"))
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}
	box, err := h.e2eeService.GetGroupCurrentKeyBox(c.Context(), currentUserID, groupID)
	if err != nil {
		if errors.Is(err, service.ErrE2EEGroupKeyBoxMissing) {
			version, verr := h.e2eeService.GetGroupCurrentVersion(c.Context(), groupID)
			if verr != nil {
				return h.handleGroupKeyError(c, err)
			}
			return c.Status(428).JSON(fiber.Map{
				"code":    428,
				"message": "e2ee group key box not found, please upload key boxes",
				"data": fiber.Map{
					"group_id":     groupID,
					"key_version":  version,
					"need_publish": true,
				},
			})
		}
		return h.handleGroupKeyError(c, err)
	}

	wrappedKeyLen, wrappedKeyDecodeErr := decodedLenBase64URLOrStd(box.WrappedGroupKey)
	wrapNonceLen, wrapNonceDecodeErr := decodedLenBase64URLOrStd(box.WrapNonce)
	logger.Info("e2ee group current key payload",
		"current_user_id", currentUserID,
		"box_user_id", box.UserID,
		"group_id", box.GroupID,
		"key_version", box.KeyVersion,
		"wrapped_group_key_masked", maskToken(box.WrappedGroupKey),
		"wrapped_group_key_raw_len", len(box.WrappedGroupKey),
		"wrapped_group_key_decoded_len", wrappedKeyLen,
		"wrapped_group_key_decode_error", wrappedKeyDecodeErr,
		"wrap_nonce_masked", maskToken(box.WrapNonce),
		"wrap_nonce_raw_len", len(box.WrapNonce),
		"wrap_nonce_decoded_len", wrapNonceLen,
		"wrap_nonce_decode_error", wrapNonceDecodeErr,
		"wrapped_by_user_id", box.WrappedByUserID,
		"is_wrapped_by_current_user", box.WrappedByUserID == currentUserID,
		"key_wrap_alg", box.KeyWrapAlg,
	)

	payload := fiber.Map{
		"group_id":           box.GroupID,
		"key_version":        box.KeyVersion,
		"target_user_id":     box.UserID,
		"wrapped_group_key":  box.WrappedGroupKey,
		"wrap_nonce":         box.WrapNonce,
		"wrapped_by_user_id": box.WrappedByUserID,
	}
	if box.KeyWrapAlg != "" {
		payload["key_wrap_alg"] = box.KeyWrapAlg
	}
	return response.Success(c, payload, "ok")
}

func (h *E2EEHandler) PublishGroupKeyBoxes(c *fiber.Ctx) error {
	currentUserID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}
	var req publishGroupKeyBoxesRequest
	if err := c.BodyParser(&req); err != nil || req.GroupID == 0 || req.KeyVersion == 0 || len(req.Boxes) == 0 {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}
	inputBoxes := make([]service.GroupKeyBoxUpload, 0, len(req.Boxes))
	for _, box := range req.Boxes {
		inputBoxes = append(inputBoxes, service.GroupKeyBoxUpload{
			UserID:          box.UserID,
			WrappedGroupKey: box.WrappedGroupKey,
			WrapNonce:       box.WrapNonce,
		})
	}
	if err := h.e2eeService.PublishGroupKeyBoxes(c.Context(), currentUserID, req.GroupID, req.KeyVersion, inputBoxes, req.KeyWrapAlg); err != nil {
		switch {
		case errors.Is(err, service.ErrE2EEGroupPermission):
			return response.Error(c, http.StatusForbidden, "你不在该群聊中")
		case errors.Is(err, service.ErrE2EEGroupKeyNotFound), errors.Is(err, service.ErrE2EEGroupVersionAbsent):
			return response.Error(c, http.StatusNotFound, "e2ee group key version not found")
		case errors.Is(err, service.ErrE2EEGroupVersionLock):
			return response.Error(c, http.StatusConflict, "e2ee group key version conflict")
		case errors.Is(err, service.ErrE2EEGroupBoxesPublished):
			return response.Error(c, http.StatusConflict, "e2ee group key boxes already published")
		case errors.Is(err, service.ErrE2EEGroupBoxesInvalid):
			return response.Error(c, http.StatusBadRequest, "invalid e2ee group key boxes payload")
		default:
			return response.Error(c, http.StatusInternalServerError, "服务端异常")
		}
	}
	return response.Success(c, fiber.Map{
		"group_id":    req.GroupID,
		"key_version": req.KeyVersion,
		"box_count":   len(req.Boxes),
	}, "ok")
}

func (h *E2EEHandler) RotateGroupKey(c *fiber.Ctx) error {
	currentUserID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}
	var req rotateGroupKeyRequest
	if err := c.BodyParser(&req); err != nil || req.GroupID == 0 || req.ExpectedKeyVersion <= 0 {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}
	keyVersion, err := h.e2eeService.RotateGroupKeyIfCurrent(c.Context(), currentUserID, req.GroupID, req.ExpectedKeyVersion)
	if err != nil {
		return h.handleGroupKeyError(c, err)
	}
	return response.Success(c, fiber.Map{
		"group_id":    req.GroupID,
		"key_version": keyVersion,
	}, "ok")
}

// 获取指定版本的群聊密钥
func (h *E2EEHandler) GetGroupKeyByVersion(c *fiber.Ctx) error {
	currentUserID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}
	groupID, err := parseUintQuery(c.Query("group_id"))
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}
	keyVersion, err := parseIntQuery(c.Query("key_version"))
	if err != nil || keyVersion <= 0 {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}
	box, err := h.e2eeService.GetGroupKeyBoxByVersion(c.Context(), currentUserID, groupID, keyVersion)
	if err != nil {
		return h.handleGroupKeyError(c, err)
	}
	return response.Success(c, fiber.Map{
		"group_id":           box.GroupID,
		"key_version":        box.KeyVersion,
		"target_user_id":     box.UserID,
		"wrapped_group_key":  box.WrappedGroupKey,
		"wrap_nonce":         box.WrapNonce,
		"wrapped_by_user_id": box.WrappedByUserID,
	}, "ok")
}
