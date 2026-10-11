package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/application"
	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/Furkanturan8/rovena/services/identity/internal/infrastructure/security"
	transport "github.com/Furkanturan8/rovena/services/identity/internal/transport/http"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	testSecret = "test-secret-test-secret-test-secret-1234"
	goodPass   = "s3cretpass1"
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

type env struct {
	t      *testing.T
	app    *fiber.App
	store  *store
	logs   *syncBuffer
	hasher *security.BcryptHasher
	issuer *security.JWTIssuer

	readyErr error
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		t: t, store: newStore(), logs: &syncBuffer{},
		hasher: security.NewBcryptHasher(4), // minimum cost keeps the suite fast
		issuer: security.NewJWTIssuer(testSecret, 15*time.Minute),
	}
	users, tokens := userRepo{e.store}, tokenRepo{e.store}
	authSvc, err := application.NewAuthService(users, tokens, e.hasher, e.issuer, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	e.app = transport.NewApp(transport.Options{
		Logger:         slog.New(slog.NewJSONHandler(e.logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		AllowedOrigins: "https://shop.example",
		TrustedProxies: []string{"127.0.0.1"},
		Verifier:       auth.NewVerifier([]byte(testSecret)),
		Readiness: map[string]httpx.ReadinessCheck{
			"postgres": func(context.Context) error { return e.readyErr },
		},
	}, transport.Services{Auth: authSvc, Users: application.NewUserService(users)})
	return e
}

// ------------------------------------------------------------------ request helpers

type result struct {
	Status int
	Header http.Header
	Raw    []byte
	Body   map[string]any
}

func (r result) data() map[string]any { d, _ := r.Body["data"].(map[string]any); return d }
func (r result) fields() map[string]any {
	f, _ := r.Body["fields"].(map[string]any)
	return f
}
func (r result) message() string { m, _ := r.Body["message"].(string); return m }
func (r result) str(path ...string) string {
	cur := r.data()
	for i, p := range path {
		if i == len(path)-1 {
			s, _ := cur[p].(string)
			return s
		}
		cur, _ = cur[p].(map[string]any)
	}
	return ""
}

// do sends a request; body may be nil, a string (sent verbatim) or a value to JSON-encode.
func (e *env) do(method, path, bearer string, body any) result {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return e.send(req)
}

func (e *env) send(req *http.Request) result {
	e.t.Helper()
	resp, err := e.app.Test(req, -1)
	if err != nil {
		e.t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	res := result{Status: resp.StatusCode, Header: resp.Header, Raw: raw}
	_ = json.Unmarshal(raw, &res.Body)
	return res
}

func (e *env) register(email string) result {
	e.t.Helper()
	return e.do("POST", "/api/v1/auth/register", "", map[string]any{"email": email, "password": goodPass})
}

func (e *env) login(email, password string) result {
	e.t.Helper()
	return e.do("POST", "/api/v1/auth/login", "", map[string]any{"email": email, "password": password})
}

// seedUser inserts a user straight into the store (no auth rate limit involved).
func (e *env) seedUser(email string, role domain.Role) *domain.User {
	e.t.Helper()
	hash, err := e.hasher.Hash(goodPass)
	if err != nil {
		e.t.Fatal(err)
	}
	id, _ := uuid.NewV7()
	u := &domain.User{
		ID: id, Email: email, PasswordHash: hash, FirstName: "Seed", Role: role,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	e.store.users[id] = u
	return u
}

func (e *env) tokenFor(u *domain.User) string {
	e.t.Helper()
	tok, _, err := e.issuer.Issue(u)
	if err != nil {
		e.t.Fatal(err)
	}
	return tok
}

func rawToken(t *testing.T, sub, role, secret string, expiresIn time.Duration) string {
	t.Helper()
	claims := auth.Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: auth.Issuer, Subject: sub, ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// ------------------------------------------------------------------ register

func TestRegister_Success(t *testing.T) {
	e := newEnv(t)
	res := e.do("POST", "/api/v1/auth/register", "", map[string]any{
		"email": "Jane@Example.com", "password": goodPass, "first_name": "  Jane ", "last_name": "Doe",
	})
	if res.Status != 201 || res.Body["success"] != true {
		t.Fatalf("status %d: %s", res.Status, res.Raw)
	}
	user := res.data()["user"].(map[string]any)
	if user["email"] != "jane@example.com" || user["first_name"] != "Jane" || user["last_name"] != "Doe" || user["role"] != "customer" {
		t.Errorf("user = %v", user)
	}
	if _, err := uuid.Parse(user["id"].(string)); err != nil {
		t.Errorf("id is not a uuid: %v", user["id"])
	}

	tokens := res.data()["tokens"].(map[string]any)
	access, refresh := tokens["access_token"].(string), tokens["refresh_token"].(string)
	if tokens["token_type"] != "Bearer" || access == "" || refresh == "" || access == refresh {
		t.Errorf("tokens = %v", tokens)
	}
	if exp, _ := time.Parse(time.RFC3339, tokens["access_expires_at"].(string)); !exp.After(time.Now()) {
		t.Errorf("access token already expired: %v", tokens["access_expires_at"])
	}
	claims, err := auth.NewVerifier([]byte(testSecret)).Verify(access)
	if err != nil || claims.UserID() != user["id"] || claims.Role != "customer" {
		t.Errorf("claims = %+v, err = %v", claims, err)
	}
	if lower := strings.ToLower(string(res.Raw)); strings.Contains(lower, "password") || strings.Contains(lower, "$2a$") {
		t.Errorf("response exposes credentials: %s", res.Raw)
	}
}

func TestRegister_IgnoresPrivilegedFields(t *testing.T) {
	e := newEnv(t)
	res := e.do("POST", "/api/v1/auth/register", "", `{"email":"evil@example.com","password":"`+goodPass+`","role":"admin","id":"00000000-0000-0000-0000-000000000001","is_admin":true}`)
	if res.Status != 201 {
		t.Fatalf("status %d: %s", res.Status, res.Raw)
	}
	user := res.data()["user"].(map[string]any)
	if user["role"] != "customer" || user["id"] == "00000000-0000-0000-0000-000000000001" {
		t.Errorf("client controlled privileged fields: %v", user)
	}
}

func TestRegister_Validation(t *testing.T) {
	multibyte := strings.Repeat("ş", 72) // 72 characters but 144 bytes
	tests := []struct {
		name      string
		body      any
		status    int
		wantField string
		wantMsg   string
	}{
		{"empty object", map[string]any{}, 422, "email", ""},
		{"missing password", map[string]any{"email": "a@example.com"}, 422, "password", ""},
		{"not an email", map[string]any{"email": "nope", "password": goodPass}, 422, "email", ""},
		{"email with spaces", map[string]any{"email": " a@example.com ", "password": goodPass}, 422, "email", ""},
		{"email too long", map[string]any{"email": strings.Repeat("a", 250) + "@example.com", "password": goodPass}, 422, "email", ""},
		{"password 7 chars", map[string]any{"email": "a@example.com", "password": "1234567"}, 422, "password", ""},
		{"password 73 chars", map[string]any{"email": "a@example.com", "password": strings.Repeat("a", 73)}, 422, "password", ""},
		{"password over 72 bytes", map[string]any{"email": "a@example.com", "password": multibyte}, 422, "", "Password must be at most 72 bytes"},
		{"first name too long", map[string]any{"email": "a@example.com", "password": goodPass, "first_name": strings.Repeat("x", 101)}, 422, "first_name", ""},
		{"password as number", `{"email":"a@example.com","password":12345678}`, 400, "", ""},
		{"body is an array", `[]`, 400, "", ""},
		{"malformed json", `{"email": `, 400, "", ""},
		{"empty body", "", 400, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			res := e.do("POST", "/api/v1/auth/register", "", tt.body)
			if res.Status != tt.status || res.Body["success"] != false {
				t.Fatalf("status = %d, want %d: %s", res.Status, tt.status, res.Raw)
			}
			if tt.wantField != "" {
				if _, ok := res.fields()[tt.wantField]; !ok {
					t.Errorf("fields = %v, want key %q", res.fields(), tt.wantField)
				}
			}
			if tt.wantMsg != "" && res.message() != tt.wantMsg {
				t.Errorf("message = %q, want %q", res.message(), tt.wantMsg)
			}
			if e.store.userCount() != 0 {
				t.Error("invalid request created a user")
			}
		})
	}
}

func TestRegister_PasswordLengthBoundaries(t *testing.T) {
	for _, pw := range []string{strings.Repeat("a", 8), strings.Repeat("a", 72)} {
		e := newEnv(t)
		if res := e.do("POST", "/api/v1/auth/register", "", map[string]any{"email": "a@example.com", "password": pw}); res.Status != 201 {
			t.Errorf("%d-char password = %d, want 201: %s", len(pw), res.Status, res.Raw)
		}
	}
}

func TestRegister_DuplicateEmailIsCaseInsensitive(t *testing.T) {
	e := newEnv(t)
	if res := e.register("jane@example.com"); res.Status != 201 {
		t.Fatalf("first = %d", res.Status)
	}
	for _, email := range []string{"jane@example.com", "JANE@EXAMPLE.COM", "Jane@Example.com"} {
		res := e.register(email)
		if res.Status != 409 || res.message() != "Email is already registered" {
			t.Errorf("%s: got %d %s", email, res.Status, res.Raw)
		}
	}
	if e.store.userCount() != 1 {
		t.Errorf("%d users stored", e.store.userCount())
	}
}

// ------------------------------------------------------------------ login

func TestLogin_Success(t *testing.T) {
	e := newEnv(t)
	e.register("jane@example.com")

	for _, email := range []string{"jane@example.com", "JANE@example.com"} {
		res := e.login(email, goodPass)
		if res.Status != 200 {
			t.Fatalf("login %s = %d: %s", email, res.Status, res.Raw)
		}
		tok := res.str("tokens", "access_token")
		if me := e.do("GET", "/api/v1/users/me", tok, nil); me.Status != 200 || me.str("email") != "jane@example.com" {
			t.Errorf("token from login rejected: %d %s", me.Status, me.Raw)
		}
	}
}

func TestLogin_FailuresAreIndistinguishable(t *testing.T) {
	e := newEnv(t)
	e.register("jane@example.com")

	wrongPassword := e.login("jane@example.com", "totally-wrong")
	unknownEmail := e.login("nobody@example.com", goodPass)
	if wrongPassword.Status != 401 || unknownEmail.Status != 401 {
		t.Fatalf("statuses = %d / %d, want 401 / 401", wrongPassword.Status, unknownEmail.Status)
	}
	if !bytes.Equal(wrongPassword.Raw, unknownEmail.Raw) {
		t.Errorf("responses differ, which reveals registered emails:\n%s\n%s", wrongPassword.Raw, unknownEmail.Raw)
	}
	if res := e.do("POST", "/api/v1/auth/login", "", map[string]any{"email": "jane@example.com"}); res.Status != 422 {
		t.Errorf("missing password = %d", res.Status)
	}
	if res := e.do("POST", "/api/v1/auth/login", "", map[string]any{"email": "x", "password": "y"}); res.Status != 422 {
		t.Errorf("malformed email = %d", res.Status)
	}
}

// ------------------------------------------------------------------ refresh & logout

func TestRefresh_RotationAndReuseDetection(t *testing.T) {
	e := newEnv(t)
	first := e.register("jane@example.com").str("tokens", "refresh_token")

	rotated := e.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": first})
	if rotated.Status != 200 || rotated.str("token_type") != "Bearer" {
		t.Fatalf("refresh = %d %s", rotated.Status, rotated.Raw)
	}
	second, newAccess := rotated.str("refresh_token"), rotated.str("access_token")
	if second == "" || second == first {
		t.Fatal("refresh token was not rotated")
	}
	if me := e.do("GET", "/api/v1/users/me", newAccess, nil); me.Status != 200 {
		t.Errorf("new access token rejected: %d", me.Status)
	}

	// Presenting the old token again looks like theft: it is rejected and the whole family dies.
	if res := e.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": first}); res.Status != 401 {
		t.Errorf("reused token = %d, want 401", res.Status)
	}
	if res := e.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": second}); res.Status != 401 {
		t.Errorf("descendant token = %d, want 401 after reuse was detected", res.Status)
	}
}

func TestRefresh_RejectsInvalidInput(t *testing.T) {
	e := newEnv(t)
	res := e.register("jane@example.com")
	access, refresh := res.str("tokens", "access_token"), res.str("tokens", "refresh_token")

	for name, body := range map[string]any{
		"unknown token":     map[string]any{"refresh_token": "does-not-exist"},
		"access as refresh": map[string]any{"refresh_token": access},
		"empty string":      map[string]any{"refresh_token": ""},
		"missing field":     map[string]any{},
		"wrong field type":  `{"refresh_token": 123}`,
		"malformed json":    `{`,
	} {
		got := e.do("POST", "/api/v1/auth/refresh", "", body)
		if got.Status < 400 || got.Status > 422 || got.Body["success"] != false {
			t.Errorf("%s: status %d body %s", name, got.Status, got.Raw)
		}
	}
	if got := e.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": "does-not-exist"}); got.Status != 401 {
		t.Errorf("unknown refresh token = %d, want 401", got.Status)
	}
	// A refresh token is not an access token.
	if got := e.do("GET", "/api/v1/users/me", refresh, nil); got.Status != 401 {
		t.Errorf("refresh token accepted as bearer: %d", got.Status)
	}
}

func TestLogout(t *testing.T) {
	e := newEnv(t)
	refresh := e.register("jane@example.com").str("tokens", "refresh_token")

	if res := e.do("POST", "/api/v1/auth/logout", "", map[string]any{"refresh_token": refresh}); res.Status != 204 || len(res.Raw) != 0 {
		t.Fatalf("logout = %d %q", res.Status, res.Raw)
	}
	if res := e.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": refresh}); res.Status != 401 {
		t.Errorf("refresh after logout = %d, want 401", res.Status)
	}
	// Idempotent, and it does not reveal whether a token existed.
	for _, tok := range []string{refresh, "never-issued"} {
		if res := e.do("POST", "/api/v1/auth/logout", "", map[string]any{"refresh_token": tok}); res.Status != 204 {
			t.Errorf("logout with %q = %d, want 204", tok, res.Status)
		}
	}
	if res := e.do("POST", "/api/v1/auth/logout", "", map[string]any{}); res.Status != 422 {
		t.Errorf("logout without token = %d, want 422", res.Status)
	}
}

func TestAuthRateLimit(t *testing.T) {
	e := newEnv(t)
	e.register("jane@example.com")

	for i := 1; i < 20; i++ { // register above used 1 of 20
		if res := e.login("jane@example.com", "wrong"); res.Status != 401 {
			t.Fatalf("attempt %d = %d, want 401", i, res.Status)
		}
	}
	limited := e.login("jane@example.com", goodPass) // correct password, but the budget is spent
	if limited.Status != 429 || limited.Body["success"] != false || limited.message() == "" {
		t.Fatalf("21st attempt = %d %s, want 429", limited.Status, limited.Raw)
	}
	for _, path := range []string{"/api/v1/auth/register", "/api/v1/auth/refresh", "/api/v1/auth/logout"} {
		if res := e.do("POST", path, "", map[string]any{}); res.Status != 429 {
			t.Errorf("%s while limited = %d, want 429 (one budget for all auth routes)", path, res.Status)
		}
	}
	// Everything outside /auth is unaffected.
	if res := e.do("GET", "/healthz", "", nil); res.Status != 200 {
		t.Errorf("healthz = %d", res.Status)
	}
	u := e.seedUser("admin@example.com", domain.RoleAdmin)
	if res := e.do("GET", "/api/v1/users/me", e.tokenFor(u), nil); res.Status != 200 {
		t.Errorf("users/me while auth is limited = %d", res.Status)
	}
}

// ------------------------------------------------------------------ authentication & authorization

func TestUserRoutes_Authorization(t *testing.T) {
	e := newEnv(t)
	customer := e.seedUser("c@example.com", domain.RoleCustomer)
	admin := e.seedUser("a@example.com", domain.RoleAdmin)
	id := customer.ID.String()

	noneToken, _ := jwt.NewWithClaims(jwt.SigningMethodNone, auth.Claims{
		Role: auth.RoleAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: auth.Issuer, Subject: admin.ID.String(), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	routes := []struct {
		method, path string
		adminOnly    bool
	}{
		{"GET", "/api/v1/users/me", false},
		{"PATCH", "/api/v1/users/me", false},
		{"GET", "/api/v1/users", true},
		{"GET", "/api/v1/users/" + id, true},
		{"PATCH", "/api/v1/users/" + id + "/role", true},
	}
	creds := []struct {
		name      string
		header    string
		bearer    string
		anonymous bool
		want      func(adminOnly bool) int // 0 = must pass authentication and authorization
	}{
		{name: "no header", anonymous: true, want: func(bool) int { return 401 }},
		{name: "basic scheme", header: "Basic dXNlcjpwYXNz", want: func(bool) int { return 401 }},
		{name: "empty bearer", header: "Bearer ", want: func(bool) int { return 401 }},
		{name: "garbage", bearer: "not.a.jwt", want: func(bool) int { return 401 }},
		{name: "wrong signature", bearer: rawToken(t, admin.ID.String(), auth.RoleAdmin, "another-secret-another-secret-123456", time.Hour), want: func(bool) int { return 401 }},
		{name: "expired", bearer: rawToken(t, admin.ID.String(), auth.RoleAdmin, testSecret, -time.Minute), want: func(bool) int { return 401 }},
		{name: "alg none", bearer: noneToken, want: func(bool) int { return 401 }},
		{name: "customer", bearer: e.tokenFor(customer), want: func(adminOnly bool) int {
			if adminOnly {
				return 403
			}
			return 0
		}},
		{name: "admin", bearer: e.tokenFor(admin), want: func(bool) int { return 0 }},
	}
	for _, r := range routes {
		for _, c := range creds {
			req := httptest.NewRequest(r.method, r.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			switch {
			case c.header != "":
				req.Header.Set("Authorization", c.header)
			case c.bearer != "":
				req.Header.Set("Authorization", "Bearer "+c.bearer)
			}
			got, want := e.send(req).Status, c.want(r.adminOnly)
			if want != 0 && got != want {
				t.Errorf("%s %s [%s] = %d, want %d", r.method, r.path, c.name, got, want)
			}
			if want == 0 && (got == 401 || got == 403) {
				t.Errorf("%s %s [%s] = %d, must pass authorization", r.method, r.path, c.name, got)
			}
		}
	}
}

func TestTokenForVanishedUser(t *testing.T) {
	e := newEnv(t)
	ghost := &domain.User{ID: uuid.New(), Role: domain.RoleCustomer}
	res := e.do("GET", "/api/v1/users/me", e.tokenFor(ghost), nil)
	if res.Status != 404 || res.message() != "User not found" {
		t.Errorf("got %d %s", res.Status, res.Raw)
	}
}

// ------------------------------------------------------------------ profile

func TestProfile_GetAndPartialUpdate(t *testing.T) {
	e := newEnv(t)
	u := e.seedUser("jane@example.com", domain.RoleCustomer)
	tok := e.tokenFor(u)

	me := e.do("GET", "/api/v1/users/me", tok, nil)
	if me.Status != 200 || me.str("id") != u.ID.String() || me.str("first_name") != "Seed" {
		t.Fatalf("me = %d %s", me.Status, me.Raw)
	}
	if strings.Contains(strings.ToLower(string(me.Raw)), "password") {
		t.Errorf("profile exposes credentials: %s", me.Raw)
	}

	upd := e.do("PATCH", "/api/v1/users/me", tok, map[string]any{"last_name": "  Doe  "})
	if upd.Status != 200 || upd.str("last_name") != "Doe" || upd.str("first_name") != "Seed" {
		t.Errorf("partial update = %d %s (first name must stay)", upd.Status, upd.Raw)
	}
	if res := e.do("PATCH", "/api/v1/users/me", tok, map[string]any{}); res.Status != 200 {
		t.Errorf("empty patch = %d", res.Status)
	}
	if res := e.do("PATCH", "/api/v1/users/me", tok, map[string]any{"first_name": strings.Repeat("x", 101)}); res.Status != 422 {
		t.Errorf("long name = %d", res.Status)
	}
}

func TestProfile_CannotChangeEmailOrRole(t *testing.T) {
	e := newEnv(t)
	u := e.seedUser("jane@example.com", domain.RoleCustomer)
	res := e.do("PATCH", "/api/v1/users/me", e.tokenFor(u), `{"first_name":"Jane","role":"admin","email":"evil@example.com","id":"x"}`)
	if res.Status != 200 {
		t.Fatalf("status %d: %s", res.Status, res.Raw)
	}
	if res.str("role") != "customer" || res.str("email") != "jane@example.com" {
		t.Errorf("mass assignment: %s", res.Raw)
	}
	if stored := e.store.users[u.ID]; stored.Role != domain.RoleCustomer || stored.Email != "jane@example.com" {
		t.Errorf("stored user = %+v", stored)
	}
}

// ------------------------------------------------------------------ admin endpoints

func TestAdminList(t *testing.T) {
	e := newEnv(t)
	admin := e.seedUser("admin@example.com", domain.RoleAdmin)
	for i := 0; i < 4; i++ {
		e.seedUser(fmt.Sprintf("u%d@example.com", i), domain.RoleCustomer)
		time.Sleep(time.Millisecond) // distinct created_at values
	}
	tok := e.tokenFor(admin)

	res := e.do("GET", "/api/v1/users?page=2&page_size=2", tok, nil)
	d := res.data()
	if res.Status != 200 || d["total"] != float64(5) || d["page"] != float64(2) || d["page_size"] != float64(2) || len(d["items"].([]any)) != 2 {
		t.Fatalf("page = %d %s", res.Status, res.Raw)
	}
	if strings.Contains(strings.ToLower(string(res.Raw)), "password") {
		t.Errorf("list exposes credentials: %s", res.Raw)
	}

	// Out-of-range paging is clamped, never an error and never unbounded.
	big := e.do("GET", "/api/v1/users?page=-3&page_size=100000", tok, nil)
	if big.Status != 200 || big.data()["page"] != float64(1) || big.data()["page_size"] != float64(100) {
		t.Errorf("clamping: %d %v", big.Status, big.data())
	}
	if res := e.do("GET", "/api/v1/users?page=99", tok, nil); res.Status != 200 || !strings.Contains(string(res.Raw), `"items":[]`) {
		t.Errorf("page past the end = %d %s", res.Status, res.Raw)
	}
}

func TestAdminGetByID(t *testing.T) {
	e := newEnv(t)
	admin := e.seedUser("admin@example.com", domain.RoleAdmin)
	target := e.seedUser("t@example.com", domain.RoleCustomer)
	tok := e.tokenFor(admin)

	if res := e.do("GET", "/api/v1/users/"+target.ID.String(), tok, nil); res.Status != 200 || res.str("email") != "t@example.com" {
		t.Errorf("get = %d %s", res.Status, res.Raw)
	}
	if res := e.do("GET", "/api/v1/users/"+uuid.NewString(), tok, nil); res.Status != 404 {
		t.Errorf("unknown id = %d", res.Status)
	}
	for _, bad := range []string{"nope", "123", "%27%3B%20DROP%20TABLE%20users%3B--"} {
		if res := e.do("GET", "/api/v1/users/"+bad, tok, nil); res.Status != 400 {
			t.Errorf("id %q = %d, want 400", bad, res.Status)
		}
	}
}

func TestAdminChangeRole(t *testing.T) {
	e := newEnv(t)
	admin := e.seedUser("admin@example.com", domain.RoleAdmin)
	target := e.seedUser("t@example.com", domain.RoleCustomer)
	tok := e.tokenFor(admin)
	path := "/api/v1/users/" + target.ID.String() + "/role"

	promoted := e.do("PATCH", path, tok, map[string]any{"role": "admin"})
	if promoted.Status != 200 || promoted.str("role") != "admin" || e.store.users[target.ID].Role != domain.RoleAdmin {
		t.Fatalf("promote = %d %s", promoted.Status, promoted.Raw)
	}
	// Two admins now: demoting one is fine.
	if res := e.do("PATCH", path, tok, map[string]any{"role": "customer"}); res.Status != 200 || res.str("role") != "customer" {
		t.Errorf("demote = %d %s", res.Status, res.Raw)
	}
	// Only one admin left: that one is protected.
	last := e.do("PATCH", "/api/v1/users/"+admin.ID.String()+"/role", tok, map[string]any{"role": "customer"})
	if last.Status != 409 || last.message() != "Cannot demote the last admin" {
		t.Errorf("last admin = %d %s", last.Status, last.Raw)
	}
	if e.store.users[admin.ID].Role != domain.RoleAdmin {
		t.Error("last admin was demoted")
	}
}

func TestAdminChangeRole_Validation(t *testing.T) {
	e := newEnv(t)
	admin := e.seedUser("admin@example.com", domain.RoleAdmin)
	target := e.seedUser("t@example.com", domain.RoleCustomer)
	tok := e.tokenFor(admin)
	path := "/api/v1/users/" + target.ID.String() + "/role"

	for name, body := range map[string]any{
		"unknown role": map[string]any{"role": "superuser"},
		"upper case":   map[string]any{"role": "ADMIN"},
		"missing":      map[string]any{},
		"empty":        map[string]any{"role": ""},
		"sql in role":  map[string]any{"role": "admin'; --"},
	} {
		if res := e.do("PATCH", path, tok, body); res.Status != 422 {
			t.Errorf("%s = %d, want 422 (%s)", name, res.Status, res.Raw)
		}
	}
	if e.store.users[target.ID].Role != domain.RoleCustomer {
		t.Error("invalid request changed the role")
	}
	if res := e.do("PATCH", "/api/v1/users/"+uuid.NewString()+"/role", tok, map[string]any{"role": "admin"}); res.Status != 404 {
		t.Errorf("unknown user = %d", res.Status)
	}
	if res := e.do("PATCH", "/api/v1/users/nope/role", tok, map[string]any{"role": "admin"}); res.Status != 400 {
		t.Errorf("bad id = %d", res.Status)
	}
}

// ------------------------------------------------------------------ errors & logging

func TestUnexpectedErrorsDoNotLeakDetails(t *testing.T) {
	e := newEnv(t)
	e.store.createUserErr = errors.New("dial tcp 10.0.0.5:5432: connection refused (password=hunter2)")
	res := e.register("jane@example.com")
	if res.Status != 500 || res.message() != "Internal server error" {
		t.Fatalf("got %d %s", res.Status, res.Raw)
	}
	if strings.Contains(string(res.Raw), "hunter2") || strings.Contains(string(res.Raw), "10.0.0.5") {
		t.Errorf("response leaks internals: %s", res.Raw)
	}
	// Operators still need the cause in the server log.
	if !strings.Contains(e.logs.String(), "connection refused") {
		t.Error("the underlying error was not logged")
	}
}

func TestLogsNeverContainCredentialsOrTokens(t *testing.T) {
	e := newEnv(t)
	const secretPass = "Sup3r-Secret-Passw0rd"

	reg := e.do("POST", "/api/v1/auth/register", "", map[string]any{"email": "jane@example.com", "password": secretPass})
	access, refresh := reg.str("tokens", "access_token"), reg.str("tokens", "refresh_token")
	hash := e.store.users[mustUUID(t, reg.str("user", "id"))].PasswordHash

	login := e.login("jane@example.com", secretPass)
	e.login("jane@example.com", "Wrong-Attempt-Passw0rd")
	rotated := e.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": refresh})
	e.do("GET", "/api/v1/users/me", access, nil)
	e.do("POST", "/api/v1/auth/logout", "", map[string]any{"refresh_token": rotated.str("refresh_token")})

	logs := e.logs.String()
	if !strings.Contains(logs, `"msg":"http request"`) || !strings.Contains(logs, "/api/v1/auth/login") {
		t.Fatalf("expected access log lines, got: %s", logs)
	}
	secrets := map[string]string{
		"password":          secretPass,
		"wrong password":    "Wrong-Attempt-Passw0rd",
		"password hash":     hash,
		"access token":      access,
		"refresh token":     refresh,
		"login access":      login.str("tokens", "access_token"),
		"rotated refresh":   rotated.str("refresh_token"),
		"authorization hdr": "Bearer ",
	}
	for name, s := range secrets {
		if s != "" && strings.Contains(logs, s) {
			t.Errorf("logs contain the %s", name)
		}
	}
}

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// ------------------------------------------------------------------ cross-cutting

func TestHealthEndpoints(t *testing.T) {
	e := newEnv(t)
	if res := e.do("GET", "/healthz", "", nil); res.Status != 200 {
		t.Errorf("healthz = %d", res.Status)
	}
	if res := e.do("GET", "/readyz", "", nil); res.Status != 200 || res.Body["ready"] != true {
		t.Errorf("readyz = %d %s", res.Status, res.Raw)
	}
	e.readyErr = errors.New("dial tcp 10.0.0.5:5432: connection refused")
	res := e.do("GET", "/readyz", "", nil)
	if res.Status != 503 || res.Body["ready"] != false || strings.Contains(string(res.Raw), "10.0.0.5") {
		t.Errorf("readyz with failing db = %d %s", res.Status, res.Raw)
	}
	if res := e.do("GET", "/healthz", "", nil); res.Status != 200 {
		t.Errorf("liveness must not depend on the database, got %d", res.Status)
	}
}

func TestRequestIDAndSecurityHeaders(t *testing.T) {
	e := newEnv(t)
	res := e.do("GET", "/healthz", "", nil)
	if res.Header.Get("X-Request-ID") == "" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("headers = %v", res.Header)
	}
	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	req.Header.Set("X-Request-ID", "trace-abc-123")
	got := e.send(req)
	if got.Status != 401 || got.Header.Get("X-Request-ID") != "trace-abc-123" {
		t.Errorf("error response: %d, request id %q", got.Status, got.Header.Get("X-Request-ID"))
	}
}

func TestCORS(t *testing.T) {
	e := newEnv(t)
	preflight := func(origin string) result {
		req := httptest.NewRequest("OPTIONS", "/api/v1/users/me", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "PATCH")
		req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
		return e.send(req)
	}
	ok := preflight("https://shop.example")
	if ok.Status != 204 || ok.Header.Get("Access-Control-Allow-Origin") != "https://shop.example" {
		t.Errorf("allowed origin: %d ACAO=%q", ok.Status, ok.Header.Get("Access-Control-Allow-Origin"))
	}
	if got := preflight("https://evil.example").Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed origin got ACAO=%q", got)
	}
}

func TestUnknownRoutesAndMethods(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(e.seedUser("admin@example.com", domain.RoleAdmin))
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"GET", "/api/v1/nothing", 404},
		{"GET", "/api/v2/users/me", 404},
		{"GET", "/api/v1/auth/login", 405},
		{"DELETE", "/api/v1/users/" + uuid.NewString(), 405},
		{"PUT", "/api/v1/users/me", 405},
	} {
		res := e.do(tc.method, tc.path, tok, nil)
		if res.Status != tc.want || res.Body["success"] != false {
			t.Errorf("%s %s = %d %s, want %d in the standard envelope", tc.method, tc.path, res.Status, res.Raw, tc.want)
		}
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

	post := func(body string) int {
		resp, err := http.Post("http://"+ln.Addr().String()+"/api/v1/auth/register", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	if got := post(`{"email":"a@example.com","password":"` + strings.Repeat("a", 70*1024) + `"}`); got != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body = %d, want 413", got)
	}
	if got := post(`{}`); got != http.StatusUnprocessableEntity {
		t.Errorf("small body = %d, want 422", got)
	}
}
