package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestCORSExplicitOriginPreflight(t *testing.T) {
	app := fiber.New()
	app.Use(CORS(CORSConfig{AllowedOrigins: "https://client.example, https://admin.example"}))
	app.Get("/resource", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	req.Header.Set("Origin", "https://client.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("preflight request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://client.example" {
		t.Fatalf("allow-origin = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("allow-credentials = %q, want true", got)
	}
}

func TestCORSRejectsUnlistedOrigin(t *testing.T) {
	app := fiber.New()
	app.Use(CORS(CORSConfig{AllowedOrigins: "https://client.example"}))
	app.Get("/resource", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	req.Header.Set("Origin", "https://attacker.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("preflight request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("rejected preflight status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestCORSWildcardDoesNotEnableCredentials(t *testing.T) {
	app := fiber.New()
	app.Use(Cors("*"))
	app.Get("/resource", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "/resource", nil)
	req.Header.Set("Origin", "https://client.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("allow-origin = %q, want wildcard", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("wildcard allow-credentials = %q, want empty", got)
	}
}
