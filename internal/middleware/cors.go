package middleware

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// CORSConfig defines the cross-origin policy for the API. Origins are
// comma-separated in the application config so deployments can configure this
// without changing code.
type CORSConfig struct {
	AllowedOrigins string
}

// CORS creates a middleware with an explicit origin allow-list. A literal "*"
// (or an empty value) allows public cross-origin requests but deliberately does
// not enable credentials, since browsers reject wildcard + credentials.
func CORS(cfg CORSConfig) fiber.Handler {
	origins := parseOrigins(cfg.AllowedOrigins)
	wildcard := len(origins) == 0 || contains(origins, "*")

	return func(c *fiber.Ctx) error {
		requestOrigin := strings.TrimSpace(c.Get("Origin"))
		allowed := wildcard
		if !wildcard && requestOrigin != "" {
			allowed = contains(origins, requestOrigin)
		}
		if requestOrigin != "" && !allowed {
			if c.Method() == http.MethodOptions {
				return c.SendStatus(fiber.StatusForbidden)
			}
			return c.Next()
		}

		if wildcard {
			c.Set("Access-Control-Allow-Origin", "*")
		} else if requestOrigin != "" {
			c.Set("Access-Control-Allow-Origin", requestOrigin)
			c.Set("Access-Control-Allow-Credentials", "true")
			c.Append("Vary", "Origin")
		}
		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Accept, Content-Type, Authorization, Origin, X-Requested-With")
		if c.Method() == http.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	}
}

// Cors retains the concise constructor used by existing callers while routing
// all behavior through the configurable implementation.
func Cors(allowedOrigins ...string) fiber.Handler {
	return CORS(CORSConfig{AllowedOrigins: strings.Join(allowedOrigins, ",")})
}

func parseOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if origin := strings.TrimSpace(part); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
