package response

import (
	"net/http"
	"sleet0922/graduation_project/pkg/errcode"

	"github.com/gofiber/fiber/v2"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Success(c *fiber.Ctx, data interface{}, msg string) error {
	if msg == "" {
		msg = errcode.GetMsg(errcode.Success)
	}
	return c.Status(http.StatusOK).JSON(Response{
		Code:    errcode.Success,
		Message: msg,
		Data:    data,
	})
}

func Error(c *fiber.Ctx, httpCode int, msg string) error {
	return c.Status(httpCode).JSON(Response{
		Code:    httpCode,
		Message: msg,
		Data:    nil,
	})
}

// 使用统一定义的业务错误码
func Result(c *fiber.Ctx, httpCode, errCode int, data interface{}) error {
	return c.Status(httpCode).JSON(Response{
		Code:    errCode,
		Message: errcode.GetMsg(uint16(errCode)),
		Data:    data,
	})
}
