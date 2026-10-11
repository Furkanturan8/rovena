package auth

import (
	"slices"
	"strings"

	"github.com/Furkanturan8/rovena/shared/errorx"
	"github.com/gofiber/fiber/v2"
)

const claimsKey = "auth_claims"

// Middleware requires a valid Bearer access token and stores its claims on the request.
func Middleware(v *Verifier) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)
		scheme, token, found := strings.Cut(header, " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
			return errorx.WithMessage(errorx.ErrUnauthorized, "Missing or malformed authorization header")
		}
		claims, err := v.Verify(token)
		if err != nil {
			return errorx.WithMessage(errorx.ErrUnauthorized, "Invalid or expired access token")
		}
		c.Locals(claimsKey, claims)
		return c.Next()
	}
}

// RequireRole must run after Middleware.
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := FromContext(c)
		if !ok {
			return errorx.ErrUnauthorized
		}
		if !slices.Contains(roles, claims.Role) {
			return errorx.ErrForbidden
		}
		return c.Next()
	}
}

func FromContext(c *fiber.Ctx) (*Claims, bool) {
	claims, ok := c.Locals(claimsKey).(*Claims)
	return claims, ok
}
