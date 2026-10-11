package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/shared/errorx"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// appWith builds an app whose error handler renders AppErrors the way the services do.
func appWith(handlers ...fiber.Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		if ae, ok := err.(*errorx.AppError); ok {
			return c.Status(ae.Code).SendString(ae.Message)
		}
		return c.SendStatus(500)
	}})
	app.Get("/x", append(handlers, func(c *fiber.Ctx) error {
		claims, _ := FromContext(c)
		return c.SendString(claims.UserID() + ":" + claims.Role)
	})...)
	return app
}

func call(t *testing.T, app *fiber.App, authorization string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("GET", "/x", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func tokenFor(t *testing.T, role string) string {
	t.Helper()
	c := validClaims()
	c.Role = role
	return sign(t, jwt.SigningMethodHS256, secret, c)
}

func TestMiddlewareRejectsMissingOrMalformedHeaders(t *testing.T) {
	app := appWith(Middleware(NewVerifier(secret)))
	good := tokenFor(t, RoleCustomer)
	for name, header := range map[string]string{
		"no header":     "",
		"no scheme":     good,
		"wrong scheme":  "Basic " + good,
		"empty token":   "Bearer ",
		"only scheme":   "Bearer",
		"garbage token": "Bearer not.a.token",
		"other secret":  "Bearer " + sign(t, jwt.SigningMethodHS256, []byte("another-secret-another-secret-xx"), validClaims()),
		"two tokens":    "Bearer " + good + " " + good,
	} {
		if status, _ := call(t, app, header); status != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, status)
		}
	}
}

func TestMiddlewareAcceptsAValidTokenCaseInsensitively(t *testing.T) {
	app := appWith(Middleware(NewVerifier(secret)))
	good := tokenFor(t, RoleCustomer)
	for _, scheme := range []string{"Bearer", "bearer", "BEARER"} {
		status, body := call(t, app, scheme+" "+good)
		if status != 200 || body != "user-1:customer" {
			t.Errorf("%s: %d %q", scheme, status, body)
		}
	}
}

func TestExpiredTokenIsRejectedEvenWhenSignedCorrectly(t *testing.T) {
	c := validClaims()
	c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Second))
	app := appWith(Middleware(NewVerifier(secret)))
	if status, _ := call(t, app, "Bearer "+sign(t, jwt.SigningMethodHS256, secret, c)); status != 401 {
		t.Fatalf("status %d", status)
	}
}

func TestRequireRole(t *testing.T) {
	app := appWith(Middleware(NewVerifier(secret)), RequireRole(RoleAdmin))
	if status, _ := call(t, app, "Bearer "+tokenFor(t, RoleCustomer)); status != http.StatusForbidden {
		t.Errorf("customer on an admin route: %d, want 403", status)
	}
	if status, _ := call(t, app, "Bearer "+tokenFor(t, "somebody-new")); status != http.StatusForbidden {
		t.Errorf("unknown role: %d, want 403", status)
	}
	if status, body := call(t, app, "Bearer "+tokenFor(t, RoleAdmin)); status != 200 || body != "user-1:admin" {
		t.Errorf("admin: %d %q", status, body)
	}
}

func TestRequireRoleWithoutMiddlewareDeniesInsteadOfAllowing(t *testing.T) {
	// A route that forgot Middleware must fail closed, not open.
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error { return c.SendStatus(errorx.ErrUnauthorized.Code) }})
	app.Get("/x", RequireRole(RoleAdmin), func(c *fiber.Ctx) error { return c.SendString("secret") })
	resp, _ := app.Test(httptest.NewRequest("GET", "/x", nil))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 401 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRequireRoleAcceptsAnyOfSeveral(t *testing.T) {
	app := appWith(Middleware(NewVerifier(secret)), RequireRole(RoleAdmin, RoleCustomer))
	for _, r := range []string{RoleAdmin, RoleCustomer} {
		if status, _ := call(t, app, "Bearer "+tokenFor(t, r)); status != 200 {
			t.Errorf("%s: %d", r, status)
		}
	}
}
