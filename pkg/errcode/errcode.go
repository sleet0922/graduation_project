package errcode

// 统一定义系统错误码
const (
	Success = 200

	// 基本错误
	InvalidParams       = 400
	Unauthorized        = 401
	Forbidden           = 403
	NotFound            = 404
	Conflict            = 409
	InternalServerError = 500
	ServiceUnavailable  = 503

	// 用户相关 (100xx)
	ErrorUserExist     = 10001
	ErrorUserNotExist  = 10002
	ErrorPasswordCheck = 10003
	ErrorTokenGenerate = 10004
	ErrorTokenParse    = 10005
	ErrorTokenExpired  = 10006

	// 好友相关 (101xx)
	ErrorFriendExist      = 10101
	ErrorFriendNotExist   = 10102
	ErrorFriendRequestPending = 10103

	// 群组相关 (102xx)
	ErrorGroupNotExist    = 10201
	ErrorGroupPermission  = 10202
	ErrorGroupMemberLimit = 10203

	// 消息相关 (103xx)
	ErrorMessageEmpty      = 10301
	ErrorMessagePermission = 10302

	// E2EE相关 (104xx)
	ErrorE2EEKeyNotFound = 10401
	ErrorE2EEKeyInvalid  = 10402

	// RTC相关 (105xx)
	ErrorRTCCallNotFound = 10501
	ErrorRTCUserBusy     = 10502
	ErrorRTCPermission   = 10503
)

// MsgFlags 错误码对应的中文提示信息
var MsgFlags = map[uint16]string{
	Success:             "ok",
	InvalidParams:       "请求参数错误",
	Unauthorized:        "未授权",
	Forbidden:           "禁止访问",
	NotFound:            "资源不存在",
	Conflict:            "资源冲突",
	InternalServerError: "服务器内部错误",
	ServiceUnavailable:  "服务暂时不可用",

	ErrorUserExist:     "用户已存在",
	ErrorUserNotExist:  "用户不存在",
	ErrorPasswordCheck: "密码错误",
	ErrorTokenGenerate: "Token生成失败",
	ErrorTokenParse:    "Token解析失败",
	ErrorTokenExpired:  "Token已过期",

	ErrorFriendExist:          "已经是好友",
	ErrorFriendNotExist:       "好友关系不存在",
	ErrorFriendRequestPending: "好友请求待处理",

	ErrorGroupNotExist:    "群组不存在",
	ErrorGroupPermission:  "无权限操作该群组",
	ErrorGroupMemberLimit: "群组成员数已达上限",

	ErrorMessageEmpty:      "消息内容不能为空",
	ErrorMessagePermission: "无权限发送消息",

	ErrorE2EEKeyNotFound: "加密密钥不存在",
	ErrorE2EEKeyInvalid:  "加密密钥无效",

	ErrorRTCCallNotFound: "通话不存在",
	ErrorRTCUserBusy:     "用户忙线中",
	ErrorRTCPermission:   "无权限操作该通话",
}

func GetMsg(code uint16) string {
	msg, right := MsgFlags[code]
	if right {
		return msg
	}
	return MsgFlags[InternalServerError]
}
