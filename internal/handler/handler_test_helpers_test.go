package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	redisPkg "sleet0922/graduation_project/pkg/redis"
)

func testJSONRequest(method, target string, body any) *http.Request {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// testSessionStore keeps handler tests independent from a process-global
// Redis client. Login/refresh behavior has dedicated integration coverage;
// these endpoint tests only need a valid injected dependency.
type testSessionStore struct{}

var _ redisPkg.SessionStore = testSessionStore{}

func (testSessionStore) SetUserSession(uint, string, time.Duration) (string, error) {
	return "", nil
}
func (testSessionStore) GetUserSession(uint) (string, error)                         { return "", nil }
func (testSessionStore) ExpireUserSession(uint, time.Duration) error                 { return nil }
func (testSessionStore) DelUserSession(uint) error                                   { return nil }
func (testSessionStore) IsSessionValid(uint, string) (bool, error)                   { return true, nil }
func (testSessionStore) SetRefreshTokenID(uint, string, string, time.Duration) error { return nil }
func (testSessionStore) RotateRefreshTokenID(uint, string, string, string, time.Duration) error {
	return nil
}

func testResponse(t *testing.T, app *fiber.App, req *http.Request) (int, map[string]any) {
	t.Helper()
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	return resp.StatusCode, payload
}

func withUser(userID uint, next fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		return next(c)
	}
}
