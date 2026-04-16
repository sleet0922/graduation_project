package errcode

// 统一定义系统错误码
const (
	Success = 200

	// 基本错误
	InvalidParams       = 400
	Unauthorized        = 401
	Forbidden           = 403
	NotFound            = 404
	InternalServerError = 500

	// 用户
	ErrorUserExist     = 10001
	ErrorUserNotExist  = 10002
	ErrorPasswordCheck = 10003
	ErrorTokenGenerate = 10004
	ErrorTokenParse    = 10005
	ErrorTokenExpired  = 10006

	// 好友
	ErrorFriendExist    = 20001
	ErrorFriendNotExist = 20002
	ErrorFriendRequest  = 20003

	// OSS
	ErrorUploadFailed   = 30001
	ErrorDownloadFailed = 30002
)

// MsgFlags 错误码对应的中文提示信息
var MsgFlags = map[uint16]string{
	Success:             "ok",
	InvalidParams:       "请求参数错误",
	Unauthorized:        "未授权",
	Forbidden:           "禁止访问",
	NotFound:            "资源不存在",
	InternalServerError: "服务器内部错误",

	ErrorUserExist:     "用户已存在",
	ErrorUserNotExist:  "用户不存在",
	ErrorPasswordCheck: "密码错误",
	ErrorTokenGenerate: "Token生成失败",
	ErrorTokenParse:    "Token解析失败",
	ErrorTokenExpired:  "Token已过期",

	ErrorFriendExist:    "好友已存在",
	ErrorFriendNotExist: "好友不存在",
	ErrorFriendRequest:  "好友请求处理失败",

	ErrorUploadFailed:   "文件上传失败",
	ErrorDownloadFailed: "文件下载失败",
}

func GetMsg(code uint16) string {
	msg, right := MsgFlags[code]
	if right {
		return msg
	}
	return MsgFlags[InternalServerError]
}
