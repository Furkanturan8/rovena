package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Furkanturan8/rovena/shared/errorx"
	"github.com/gofiber/fiber/v2"
)

func do1(t *testing.T, app *fiber.App, req string, path string, body string) (int, map[string]any, *httptest.ResponseRecorder) {
	t.Helper()
	r := httptest.NewRequest(req, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(r)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out, nil
}

// ---- error handler

var errDomain = errors.New("domain: thing is missing")

func errorApp(log *slog.Logger) *fiber.App {
	mapper := func(err error) (*errorx.AppError, bool) {
		if errors.Is(err, errDomain) {
			return errorx.WithMessage(errorx.ErrNotFound, "Thing not found"), true
		}
		return nil, false
	}
	app := fiber.New(fiber.Config{ErrorHandler: NewErrorHandler(log, mapper)})
	app.Get("/app", func(*fiber.Ctx) error {
		return errorx.WithFields(errorx.ErrValidation, map[string]string{"name": "is required"})
	})
	app.Get("/wrapped", func(*fiber.Ctx) error { return errors.Join(errors.New("context"), errorx.ErrForbidden) })
	app.Get("/domain", func(*fiber.Ctx) error { return errDomain })
	app.Get("/fiber", func(*fiber.Ctx) error { return fiber.NewError(418, "teapot") })
	app.Get("/boom", func(*fiber.Ctx) error { return errors.New("db password is hunter2") })
	return app
}

func TestErrorHandlerRendersEachKindOfError(t *testing.T) {
	app := errorApp(slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, tc := range []struct {
		path   string
		status int
		msg    string
	}{
		{"/app", 422, "Validation failed"},
		{"/wrapped", 403, "Forbidden"},
		{"/domain", 404, "Thing not found"},
		{"/fiber", 418, "teapot"},
		{"/missing", 404, "Cannot GET /missing"},
	} {
		status, body, _ := do1(t, app, "GET", tc.path, "")
		if status != tc.status || body["message"] != tc.msg || body["success"] != false {
			t.Errorf("%s: %d %v", tc.path, status, body)
		}
	}
	_, body, _ := do1(t, app, "GET", "/app", "")
	if f, _ := body["fields"].(map[string]any); f["name"] != "is required" {
		t.Errorf("fields lost: %v", body)
	}
}

func TestUnexpectedErrorsAreHiddenFromClientsButLogged(t *testing.T) {
	var logs bytes.Buffer
	app := errorApp(slog.New(slog.NewTextHandler(&logs, nil)))
	status, body, _ := do1(t, app, "GET", "/boom", "")
	if status != 500 || body["message"] != "Internal server error" {
		t.Fatalf("%d %v", status, body)
	}
	raw, _ := json.Marshal(body)
	if strings.Contains(string(raw), "hunter2") {
		t.Fatal("the internal error text reached the client")
	}
	if !strings.Contains(logs.String(), "hunter2") {
		t.Fatal("the real cause must be in the log")
	}
}

func TestClientErrorsAreNotLoggedAsFailures(t *testing.T) {
	var logs bytes.Buffer
	app := errorApp(slog.New(slog.NewTextHandler(&logs, nil)))
	do1(t, app, "GET", "/app", "")
	do1(t, app, "GET", "/domain", "")
	if logs.Len() != 0 {
		t.Fatalf("4xx responses must not fill the error log: %s", logs.String())
	}
}

// ---- validation

type sample struct {
	Name   string `json:"name" validate:"required,min=2,max=5"`
	Email  string `json:"email" validate:"omitempty,email"`
	Kind   string `json:"kind" validate:"omitempty,oneof=a b"`
	ID     string `json:"id" validate:"omitempty,uuid"`
	Nested []struct {
		URL string `json:"url" validate:"required"`
	} `json:"nested" validate:"dive"`
}

func bindApp() *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: NewErrorHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))})
	app.Post("/", func(c *fiber.Ctx) error {
		var s sample
		if err := BindAndValidate(c, &s); err != nil {
			return err
		}
		return c.SendString("ok " + s.Name)
	})
	return app
}

func TestBindAndValidate(t *testing.T) {
	app := bindApp()

	status, body, _ := do1(t, app, "POST", "/", `{"name":"abc"}`)
	if status != 200 {
		t.Fatalf("valid body: %d %v", status, body)
	}

	status, body, _ = do1(t, app, "POST", "/", `{"name":"a","email":"x","kind":"z","id":"nope","nested":[{}]}`)
	fields, _ := body["fields"].(map[string]any)
	want := map[string]string{
		"name":          "must be at least 2 characters",
		"email":         "must be a valid email address",
		"kind":          "must be one of: a b",
		"id":            "must be a valid UUID",
		"nested[0].url": "is required",
	}
	if status != 422 || len(fields) != len(want) {
		t.Fatalf("%d %v", status, body)
	}
	for k, v := range want {
		if fields[k] != v {
			t.Errorf("%s = %v, want %q", k, fields[k], v)
		}
	}

	// field names are the JSON names, not Go struct names
	for k := range fields {
		if k != strings.ToLower(k[:1])+k[1:] {
			t.Errorf("field %q looks like a Go name", k)
		}
	}

	if status, _, _ := do1(t, app, "POST", "/", `{"name":`); status != 400 {
		t.Errorf("malformed JSON: %d, want 400", status)
	}
	if status, _, _ := do1(t, app, "POST", "/", `{"name":"toolongname"}`); status != 422 {
		t.Errorf("too long: %d, want 422", status)
	}
}

// ---- pagination

func TestPaginationClampsInput(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		p, s := Pagination(c, 20, 100)
		return c.JSON(fiber.Map{"page": p, "size": s})
	})
	for _, tc := range []struct {
		q          string
		page, size float64
	}{
		{"", 1, 20},
		{"?page=3&page_size=50", 3, 50},
		{"?page=0&page_size=0", 1, 1},
		{"?page=-5&page_size=-5", 1, 1},
		{"?page=2&page_size=100000", 2, 100},
		{"?page=abc&page_size=xyz", 1, 20},
	} {
		_, body, _ := do1(t, app, "GET", "/"+tc.q, "")
		if body["page"] != tc.page || body["size"] != tc.size {
			t.Errorf("%q: got %v/%v, want %v/%v", tc.q, body["page"], body["size"], tc.page, tc.size)
		}
	}
}

// ---- health

func TestReadinessReflectsEveryDependency(t *testing.T) {
	broken := false
	app := fiber.New()
	RegisterHealth(app, map[string]ReadinessCheck{
		"postgres": func(context.Context) error { return nil },
		"rabbitmq": func(context.Context) error {
			if broken {
				return errors.New("down")
			}
			return nil
		},
	})
	if status, body, _ := do1(t, app, "GET", "/healthz", ""); status != 200 || body["status"] != "ok" {
		t.Fatalf("healthz: %d %v", status, body)
	}
	status, body, _ := do1(t, app, "GET", "/readyz", "")
	if status != 200 || body["ready"] != true {
		t.Fatalf("readyz: %d %v", status, body)
	}
	broken = true
	status, body, _ = do1(t, app, "GET", "/readyz", "")
	checks, _ := body["checks"].(map[string]any)
	if status != 503 || body["ready"] != false || checks["rabbitmq"] != "unavailable" || checks["postgres"] != "ok" {
		t.Fatalf("readyz with a broken dependency: %d %v", status, body)
	}
	// Liveness must not depend on dependencies, or an outage would restart healthy processes.
	if status, _, _ := do1(t, app, "GET", "/healthz", ""); status != 200 {
		t.Fatalf("healthz must stay up, got %d", status)
	}
}

// ---- request id and access log

func TestRequestIDIsKeptOrGenerated(t *testing.T) {
	app := fiber.New()
	app.Use(RequestIDMiddleware())
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(RequestID(c)) })

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Request-ID", "abc-123")
	resp, _ := app.Test(r)
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(b) != "abc-123" || resp.Header.Get("X-Request-ID") != "abc-123" {
		t.Fatalf("incoming id lost: %q / %q", b, resp.Header.Get("X-Request-ID"))
	}

	resp, _ = app.Test(httptest.NewRequest("GET", "/", nil))
	b, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if len(b) < 20 || resp.Header.Get("X-Request-ID") != string(b) {
		t.Fatalf("generated id missing: %q", b)
	}
}

func TestAccessLogRecordsTheRealStatusButNeverSecrets(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	app := fiber.New(fiber.Config{ErrorHandler: NewErrorHandler(log)})
	app.Use(RequestIDMiddleware())
	app.Use(AccessLog(log))
	app.Post("/login", func(*fiber.Ctx) error { return errorx.ErrUnauthorized })

	r := httptest.NewRequest("POST", "/login", strings.NewReader(`{"password":"s3cret-pass"}`))
	r.Header.Set("Authorization", "Bearer top-secret-token")
	resp, _ := app.Test(r)
	_ = resp.Body.Close()

	line := logs.String()
	if !strings.Contains(line, `"status":401`) || !strings.Contains(line, `"path":"/login"`) || !strings.Contains(line, `"request_id"`) {
		t.Fatalf("log line incomplete: %s", line)
	}
	if strings.Contains(line, "s3cret-pass") || strings.Contains(line, "top-secret-token") {
		t.Fatalf("a secret reached the access log: %s", line)
	}
}
