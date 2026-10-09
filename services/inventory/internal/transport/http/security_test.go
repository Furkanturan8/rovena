package http_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/application"
	transport "github.com/Furkanturan8/rovena/services/inventory/internal/transport/http"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// syncBuffer collects log output safely; handlers run on other goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newLoggedEnv is newEnv with a logger whose output the test can read.
func newLoggedEnv(t *testing.T) (*env, *syncBuffer) {
	t.Helper()
	logs := &syncBuffer{}
	e := &env{t: t, store: newStore()}
	e.app = transport.NewApp(transport.Options{
		Logger:         slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		AllowedOrigins: "https://shop.example",
		TrustedProxies: []string{"127.0.0.1"},
		Verifier:       auth.NewVerifier([]byte(testSecret)),
		InternalAPIKey: testInternalKey,
		Readiness: map[string]httpx.ReadinessCheck{
			"postgres": func(context.Context) error { return e.readyErr },
		},
	}, transport.Services{
		Stock:        application.NewStockService(stockRepo{e.store}),
		Reservations: application.NewReservationService(reservationRepo{e.store}, 15*time.Minute),
	})
	return e, logs
}

// raw sends a hand-built request, for cases the request{} helper cannot express.
func (e *env) raw(req *http.Request) (int, http.Header, string) {
	e.t.Helper()
	res, err := e.app.Test(req, 5000)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, string(body)
}

const reservePath = "/api/v1/internal/reservations"

func validReserveBody(variantID uuid.UUID) string {
	return `{"order_id":"` + uuid.NewString() + `","items":[{"variant_id":"` + variantID.String() + `","quantity":1}]}`
}

// ---------------------------------------------------------------- internal key

func TestInternalKey_RejectsLookalikes(t *testing.T) {
	e := newEnv(t)
	variantID := e.store.seed(5)
	body := validReserveBody(variantID)

	tests := []struct {
		name  string
		apply func(r *http.Request)
	}{
		{"empty header", func(r *http.Request) { r.Header.Set("X-Internal-Key", "") }},
		{"one character short", func(r *http.Request) { r.Header.Set("X-Internal-Key", testInternalKey[:len(testInternalKey)-1]) }},
		{"one character too long", func(r *http.Request) { r.Header.Set("X-Internal-Key", testInternalKey+"x") }},
		{"upper case", func(r *http.Request) { r.Header.Set("X-Internal-Key", strings.ToUpper(testInternalKey)) }},
		// Leading/trailing spaces are stripped by the HTTP parser, so a space inside the value is the real test.
		{"space inside the key", func(r *http.Request) {
			r.Header.Set("X-Internal-Key", testInternalKey[:10]+" "+testInternalKey[10:])
		}},
		{"only a prefix", func(r *http.Request) { r.Header.Set("X-Internal-Key", testInternalKey[:8]) }},
		{"other header name", func(r *http.Request) { r.Header.Set("X-Api-Key", testInternalKey) }},
		{"as bearer token", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+testInternalKey) }},
		{"as cookie", func(r *http.Request) { r.Header.Set("Cookie", "X-Internal-Key="+testInternalKey) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", reservePath, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			tt.apply(req)
			status, _, resp := e.raw(req)
			if status != 401 {
				t.Errorf("status = %d, want 401: %s", status, resp)
			}
		})
	}

	t.Run("key in the query string", func(t *testing.T) {
		req := httptest.NewRequest("POST", reservePath+"?X-Internal-Key="+testInternalKey, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if status, _, _ := e.raw(req); status != 401 {
			t.Errorf("status = %d, want 401", status)
		}
	})
	t.Run("key in the body", func(t *testing.T) {
		req := httptest.NewRequest("POST", reservePath, strings.NewReader(`{"X-Internal-Key":"`+testInternalKey+`"}`))
		req.Header.Set("Content-Type", "application/json")
		if status, _, _ := e.raw(req); status != 401 {
			t.Errorf("status = %d, want 401", status)
		}
	})
	if got := len(e.store.reservations); got != 0 {
		t.Errorf("%d reservations were created without a valid key", got)
	}

	// The exact key is accepted.
	req := httptest.NewRequest("POST", reservePath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", testInternalKey)
	if status, _, resp := e.raw(req); status != 201 {
		t.Errorf("valid key = %d: %s", status, resp)
	}
}

func TestInternalKey_DoesNotOpenAdminEndpoints(t *testing.T) {
	e := newEnv(t)
	variantID := e.store.seed(5)
	for _, p := range []struct{ method, path string }{
		{"GET", "/api/v1/admin/stock"},
		{"GET", "/api/v1/admin/stock/" + variantID.String()},
		{"POST", "/api/v1/admin/stock/" + variantID.String() + "/adjustments"},
	} {
		res := e.do(request{method: p.method, path: p.path, internalKey: testInternalKey})
		if res.status != 401 {
			t.Errorf("%s %s with the internal key = %d, want 401", p.method, p.path, res.status)
		}
	}
}

func TestAdminRejectsAlgNoneToken(t *testing.T) {
	e := newEnv(t)
	forged, err := jwt.NewWithClaims(jwt.SigningMethodNone, auth.Claims{
		Role: auth.RoleAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: auth.Issuer, Subject: uuid.NewString(), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if res := e.do(request{method: "GET", path: "/api/v1/admin/stock", bearer: forged}); res.status != 401 {
		t.Errorf("alg none token = %d, want 401", res.status)
	}
}

// ---------------------------------------------------------------- logs

func TestLogsNeverContainSecrets(t *testing.T) {
	e, logs := newLoggedEnv(t)
	variantID := e.store.seed(5)
	adminToken := token(t, auth.RoleAdmin, testSecret, time.Hour)
	const wrongKey = "wrong-wrong-wrong-wrong-wrong-wrong-key"

	do := func(method, path, key, bearer, body string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("X-Internal-Key", key)
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		e.raw(req)
	}
	do("POST", reservePath, testInternalKey, "", validReserveBody(variantID))
	do("POST", reservePath, wrongKey, "", validReserveBody(variantID))
	do("GET", "/api/v1/admin/stock", "", adminToken, "")
	do("GET", "/api/v1/admin/stock", "", "forged.token.value", "")

	out := logs.String()
	if !strings.Contains(out, `"msg":"http request"`) || !strings.Contains(out, "/api/v1/internal/reservations") {
		t.Fatalf("expected access log lines, got: %s", out)
	}
	for name, secret := range map[string]string{
		"internal key": testInternalKey, "rejected key": wrongKey,
		"admin token": adminToken, "forged token": "forged.token.value", "jwt secret": testSecret,
	} {
		if strings.Contains(out, secret) {
			t.Errorf("logs contain the %s", name)
		}
	}
}

// ---------------------------------------------------------------- cross-cutting

func TestRequestIDAndSecurityHeaders(t *testing.T) {
	e := newEnv(t)
	status, hdr, _ := e.raw(httptest.NewRequest("GET", "/healthz", nil))
	if status != 200 || hdr.Get("X-Request-ID") == "" || hdr.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("status %d, headers %v", status, hdr)
	}
	req := httptest.NewRequest("GET", "/api/v1/admin/stock", nil)
	req.Header.Set("X-Request-ID", "trace-abc-123")
	status, hdr, _ = e.raw(req)
	if status != 401 || hdr.Get("X-Request-ID") != "trace-abc-123" {
		t.Errorf("error response: %d, request id %q (must be echoed so a failed call can be traced)", status, hdr.Get("X-Request-ID"))
	}
}

func TestCORS(t *testing.T) {
	e := newEnv(t)
	preflight := func(origin string) (int, http.Header) {
		req := httptest.NewRequest("OPTIONS", "/api/v1/admin/stock", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
		status, hdr, _ := e.raw(req)
		return status, hdr
	}
	status, hdr := preflight("https://shop.example")
	if status != 204 || hdr.Get("Access-Control-Allow-Origin") != "https://shop.example" {
		t.Errorf("allowed origin: %d ACAO=%q", status, hdr.Get("Access-Control-Allow-Origin"))
	}
	if _, hdr := preflight("https://evil.example"); hdr.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("disallowed origin got ACAO=%q", hdr.Get("Access-Control-Allow-Origin"))
	}
	// The internal key must never be something a browser page is allowed to send.
	if allowed := hdr.Get("Access-Control-Allow-Headers"); strings.Contains(strings.ToLower(allowed), "x-internal-key") {
		t.Errorf("CORS allows the X-Internal-Key header: %q", allowed)
	}
}

func TestUnknownRoutesAndMethods(t *testing.T) {
	e := newEnv(t)
	adminToken := token(t, auth.RoleAdmin, testSecret, time.Hour)
	tests := []struct {
		method, path string
		bearer, key  string
		want         int
	}{
		{"GET", "/api/v1/nothing", "", "", 404},
		{"GET", "/api/v2/stock", "", "", 404},
		{"DELETE", "/api/v1/stock", "", "", 405},
		{"PUT", "/api/v1/admin/stock", adminToken, "", 405},
		{"DELETE", "/api/v1/admin/stock/" + uuid.NewString(), adminToken, "", 405},
		{"GET", reservePath, "", testInternalKey, 405},
		{"DELETE", reservePath + "/" + uuid.NewString(), "", testInternalKey, 405},
	}
	for _, tt := range tests {
		res := e.do(request{method: tt.method, path: tt.path, bearer: tt.bearer, internalKey: tt.key})
		if res.status != tt.want || res.body["success"] != false {
			t.Errorf("%s %s = %d %v, want %d in the standard envelope", tt.method, tt.path, res.status, res.body, tt.want)
		}
	}
}

func TestReadinessDoesNotLeakDependencyErrors(t *testing.T) {
	e := newEnv(t)
	e.readyErr = io.ErrUnexpectedEOF
	status, _, body := e.raw(httptest.NewRequest("GET", "/readyz", nil))
	if status != 503 || strings.Contains(body, "EOF") {
		t.Errorf("readyz = %d %s", status, body)
	}
	if status, _, _ := e.raw(httptest.NewRequest("GET", "/healthz", nil)); status != 200 {
		t.Errorf("liveness must not depend on the database, got %d", status)
	}
}

// ---------------------------------------------------------------- input robustness

func TestMalformedBodiesAreRejectedBeforeAnyWork(t *testing.T) {
	e := newEnv(t)
	variantID := e.store.seed(5).String()
	order := uuid.NewString()
	tests := map[string]string{
		"malformed json":       `{"order_id":`,
		"empty body":           ``,
		"body is an array":     `[]`,
		"items is a string":    `{"order_id":"` + order + `","items":"nope"}`,
		"quantity is a float":  `{"order_id":"` + order + `","items":[{"variant_id":"` + variantID + `","quantity":1.5}]}`,
		"quantity is text":     `{"order_id":"` + order + `","items":[{"variant_id":"` + variantID + `","quantity":"3"}]}`,
		"order id is a number": `{"order_id":12345,"items":[{"variant_id":"` + variantID + `","quantity":1}]}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("POST", reservePath, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Internal-Key", testInternalKey)
			status, _, resp := e.raw(req)
			if status != 400 && status != 422 {
				t.Errorf("status = %d, want 400 or 422: %s", status, resp)
			}
		})
	}
	if len(e.store.reservations) != 0 {
		t.Error("a malformed request created a reservation")
	}
}

// app.Test short-circuits the network layer, so the body limit is checked over a real socket.
func TestRequestBodyLimit(t *testing.T) {
	e := newEnv(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = e.app.Listener(ln) }()
	t.Cleanup(func() { _ = e.app.Shutdown() })

	post := func(path, body string, headers map[string]string) int {
		req, _ := http.NewRequest("POST", "http://"+ln.Addr().String()+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	huge := `{"reason":"` + strings.Repeat("a", 70*1024) + `"}`
	if got := post(reservePath, huge, map[string]string{"X-Internal-Key": testInternalKey}); got != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized reservation body = %d, want 413", got)
	}
	adminHdr := map[string]string{"Authorization": "Bearer " + token(t, auth.RoleAdmin, testSecret, time.Hour)}
	if got := post("/api/v1/admin/stock", huge, adminHdr); got != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized stock body = %d, want 413", got)
	}
	if got := post("/api/v1/admin/stock", `{}`, adminHdr); got != http.StatusUnprocessableEntity {
		t.Errorf("small body = %d, want 422", got)
	}
}

var _ = fiber.MIMEApplicationJSON
