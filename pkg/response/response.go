package response

import (
	"net/http"
	"sleet0922/graduation_project/pkg/errcode"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Success(c *gin.Context, data interface{}, msg string) {
	if msg == "" {
		msg = errcode.GetMsg(errcode.Success)
	}
	c.JSON(http.StatusOK, Response{
		Code:    errcode.Success,
		Message: msg,
		Data:    data,
	})
}

func Error(c *gin.Context, httpCode int, msg string) {
	c.JSON(httpCode, Response{
		Code:    httpCode,
		Message: msg,
		Data:    nil,
	})
}

// 使用统一定义的业务错误码
func Result(c *gin.Context, httpCode, errCode int, data interface{}) {
	c.JSON(httpCode, Response{
		Code:    errCode,
		Message: errcode.GetMsg(uint16(errCode)),
		Data:    data,
	})
}
