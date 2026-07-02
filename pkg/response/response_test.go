package response

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"sleet0922/graduation_project/pkg/errcode"
)

func TestResponseHelpers(t *testing.T) {
	tests := []struct {
		name           string
		handler        fiber.Handler
		wantHTTPStatus int
		wantCode       int
		wantMessage    string
	}{
		{
			name: "success uses custom message",
			handler: func(c *fiber.Ctx) error {
				return Success(c, fiber.Map{"ok": true}, "完成")
			},
			wantHTTPStatus: 200,
			wantCode:       errcode.Success,
			wantMessage:    "完成",
		},
		{
			name: "success uses default message",
			handler: func(c *fiber.Ctx) error {
				return Success(c, nil, "")
			},
			wantHTTPStatus: 200,
			wantCode:       errcode.Success,
			wantMessage:    "ok",
		},
		{
			name: "result maps business code message",
			handler: func(c *fiber.Ctx) error {
				return Result(c, 400, errcode.InvalidParams, nil)
			},
			wantHTTPStatus: 400,
			wantCode:       errcode.InvalidParams,
			wantMessage:    "请求参数错误",
		},
		{
			name: "error uses http status as code",
			handler: func(c *fiber.Ctx) error {
				return Error(c, 418, "teapot")
			},
			wantHTTPStatus: 418,
			wantCode:       418,
			wantMessage:    "teapot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/", tt.handler)

			resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantHTTPStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantHTTPStatus)
			}

			var got Response
			if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
				t.Fatalf("decode response failed: %v", err)
			}
			if got.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d", got.Code, tt.wantCode)
			}
			if got.Message != tt.wantMessage {
				t.Fatalf("message = %q, want %q", got.Message, tt.wantMessage)
			}
		})
	}
}
