package http

import (
	"crypto/subtle"

	"github.com/Furkanturan8/rovena/shared/errorx"
	"github.com/gofiber/fiber/v2"
)

// InternalKeyHeader carries the shared key on service-to-service calls.
const InternalKeyHeader = "X-Internal-Key"

// RequireInternalKey guards the reservation endpoints. They are called by the
// Order service, not by browsers: there is no user whose JWT could authorize them,
// and the gateway does not route them at all. The key is the second lock, for
// anything that can reach the service port directly.
func RequireInternalKey(key string) fiber.Handler {
	want := []byte(key)
	return func(c *fiber.Ctx) error {
		got := []byte(c.Get(InternalKeyHeader))
		// Constant-time compare: a timing difference would leak the key byte by byte.
		if len(got) == 0 || subtle.ConstantTimeCompare(got, want) != 1 {
			return errorx.WithMessage(errorx.ErrUnauthorized, "Missing or invalid internal API key")
		}
		return c.Next()
	}
}
