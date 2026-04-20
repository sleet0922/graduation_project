package handler

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/logger"
	"sleet0922/graduation_project/pkg/response"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type E2EEHandler struct {
	e2eeService service.E2EEService
}

type publishGroupKeyBoxesRequest struct {
	GroupID    uint                        `json:"group_id" binding:"required"`
	KeyVersion int                         `json:"key_version" binding:"required"`
	KeyWrapAlg string                      `json:"key_wrap_alg"`
	Boxes      []publishGroupKeyBoxPayload `json:"boxes" binding:"required"`
}

type publishGroupKeyBoxPayload struct {
	UserID          uint   `json:"user_id" binding:"required"`
	WrappedGroupKey string `json:"wrapped_group_key" binding:"required"`
	WrapNonce       string `json:"wrap_nonce" binding:"required"`
}

func NewE2EEHandler(e2eeService service.E2EEService) *E2EEHandler {
	return &E2EEHandler{e2eeService: e2eeService}
}

func (h *E2EEHandler) PublishPublicKey(c *gin.Context) {
	type publishKeyRequest struct {
		KeyType   string `json:"key_type" binding:"required"`
		PublicKey string `json:"public_key" binding:"required"`
	}

	var req publishKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID, err := h.getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "token 无效")
		return
	}

	key, err := h.e2eeService.PublishUserPublicKey(c.Request.Context(), userID, req.KeyType, req.PublicKey)
	if err != nil {
		if errors.Is(err, service.ErrUnsupportedE2EEKeyType) || errors.Is(err, service.ErrInvalidE2EEPublicKey) {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "服务端异常")
		return
	}

	response.Success(c, gin.H{
		"user_id":    key.UserID,
		"key_type":   key.KeyType,
		"updated_at": key.UpdatedAt.UTC().Format(time.RFC3339),
	}, "ok")
}

func (h *E2EEHandler) GetPublicKey(c *gin.Context) {
	userIDText := c.Query("user_id")
	parsedID, err := strconv.ParseUint(userIDText, 10, 64)
	if err != nil || parsedID == 0 {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	key, err := h.e2eeService.GetUserPublicKey(c.Request.Context(), uint(parsedID))
	if err != nil {
		if errors.Is(err, service.ErrE2EEPublicKeyNotFound) {
			response.Error(c, http.StatusNotFound, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "服务端异常")
		return
	}

	response.Success(c, gin.H{
		"user_id":    key.UserID,
		"key_type":   key.KeyType,
		"public_key": key.PublicKey,
		"updated_at": key.UpdatedAt.UTC().Format(time.RFC3339),
	}, "ok")
}

func (h *E2EEHandler) GetGroupCurrentKey(c *gin.Context) {
	currentUserID, err := h.getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "token 无效")
		return
	}
	groupID, err := parseUintQuery(c.Query("group_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	box, err := h.e2eeService.GetGroupCurrentKeyBox(c.Request.Context(), currentUserID, groupID)
	if err != nil {
		// 如果是密钥盒子缺失，返回特殊状态码 428，提示客户端需要上传密钥
		if errors.Is(err, service.ErrE2EEGroupKeyBoxMissing) {
			version, verr := h.e2eeService.GetGroupCurrentVersion(c.Request.Context(), groupID)
			if verr != nil {
				h.handleGroupKeyError(c, err)
				return
			}
			c.JSON(428, gin.H{
				"code":         428,
				"message":      "e2ee group key box not found, please upload key boxes",
				"data": gin.H{
					"group_id":     groupID,
					"key_version":  version,
					"need_publish": true,
				},
			})
			return
		}
		h.handleGroupKeyError(c, err)
		return
	}

	wrappedKeyLen, wrappedKeyDecodeErr := decodedLenBase64URLOrStd(box.WrappedGroupKey)
	wrapNonceLen, wrapNonceDecodeErr := decodedLenBase64URLOrStd(box.WrapNonce)
	logger.Info("e2ee group current key payload",
		"current_user_id", currentUserID,
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

	payload := gin.H{
		"group_id":           box.GroupID,
		"key_version":        box.KeyVersion,
		"wrapped_group_key":  box.WrappedGroupKey,
		"wrap_nonce":         box.WrapNonce,
		"wrapped_by_user_id": box.WrappedByUserID,
	}
	if box.KeyWrapAlg != "" {
		payload["key_wrap_alg"] = box.KeyWrapAlg
	}
	response.Success(c, payload, "ok")
}

func (h *E2EEHandler) PublishGroupKeyBoxes(c *gin.Context) {
	currentUserID, err := h.getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "token 无效")
		return
	}
	var req publishGroupKeyBoxesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	inputBoxes := make([]service.GroupKeyBoxUpload, 0, len(req.Boxes))
	for _, box := range req.Boxes {
		inputBoxes = append(inputBoxes, service.GroupKeyBoxUpload{
			UserID:          box.UserID,
			WrappedGroupKey: box.WrappedGroupKey,
			WrapNonce:       box.WrapNonce,
		})
	}
	if err := h.e2eeService.PublishGroupKeyBoxes(c.Request.Context(), currentUserID, req.GroupID, req.KeyVersion, inputBoxes, req.KeyWrapAlg); err != nil {
		switch {
		case errors.Is(err, service.ErrE2EEGroupPermission):
			response.Error(c, http.StatusForbidden, "你不在该群聊中")
		case errors.Is(err, service.ErrE2EEGroupKeyNotFound), errors.Is(err, service.ErrE2EEGroupVersionAbsent):
			response.Error(c, http.StatusNotFound, "e2ee group key version not found")
		case errors.Is(err, service.ErrE2EEGroupVersionLock):
			response.Error(c, http.StatusConflict, "e2ee group key version conflict")
		case errors.Is(err, service.ErrE2EEGroupBoxesInvalid):
			response.Error(c, http.StatusBadRequest, "invalid e2ee group key boxes payload")
		default:
			response.Error(c, http.StatusInternalServerError, "服务端异常")
		}
		return
	}
	response.Success(c, gin.H{
		"group_id":    req.GroupID,
		"key_version": req.KeyVersion,
		"box_count":   len(req.Boxes),
	}, "ok")
}

func (h *E2EEHandler) GetGroupKeyByVersion(c *gin.Context) {
	currentUserID, err := h.getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "token 无效")
		return
	}
	groupID, err := parseUintQuery(c.Query("group_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	keyVersion, err := parseIntQuery(c.Query("key_version"))
	if err != nil || keyVersion <= 0 {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	box, err := h.e2eeService.GetGroupKeyBoxByVersion(c.Request.Context(), currentUserID, groupID, keyVersion)
	if err != nil {
		h.handleGroupKeyError(c, err)
		return
	}
	response.Success(c, gin.H{
		"group_id":           box.GroupID,
		"key_version":        box.KeyVersion,
		"wrapped_group_key":  box.WrappedGroupKey,
		"wrap_nonce":         box.WrapNonce,
		"wrapped_by_user_id": box.WrappedByUserID,
	}, "ok")
}

func (h *E2EEHandler) handleGroupKeyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrE2EEGroupPermission):
		response.Error(c, http.StatusForbidden, "你不在该群聊中")
	case errors.Is(err, service.ErrE2EEGroupKeyNotFound):
		response.Error(c, http.StatusNotFound, "group key not initialized")
	case errors.Is(err, service.ErrE2EEGroupVersionAbsent):
		response.Error(c, http.StatusNotFound, "e2ee group key version not found")
	case errors.Is(err, service.ErrE2EEGroupKeyBoxMissing):
		response.Error(c, http.StatusNotFound, "e2ee group key box not found")
	case errors.Is(err, service.ErrE2EEGroupVersionLock):
		response.Error(c, http.StatusConflict, "e2ee group key version conflict")
	case errors.Is(err, service.ErrE2EEGroupBoxesInvalid):
		response.Error(c, http.StatusBadRequest, "invalid e2ee group key boxes payload")
	default:
		response.Error(c, http.StatusInternalServerError, "服务端异常")
	}
}

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

func (h *E2EEHandler) getUserID(c *gin.Context) (uint, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, fmt.Errorf("user_id not found in context")
	}
	return userID.(uint), nil
}

func decodedLenBase64URLOrStd(raw string) (int, string) {
	if raw == "" {
		return 0, "empty"
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(raw); err == nil {
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
