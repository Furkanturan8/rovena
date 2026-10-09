package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
)

type env struct {
	svc    *AuthService
	users  *fakeUsers
	tokens *fakeTokens
	clock  time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{users: newFakeUsers(), tokens: newFakeTokens(), clock: time.Now()}
	svc, err := NewAuthService(e.users, e.tokens, plainHasher{}, stubIssuer{}, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return e.clock }
	e.svc = svc
	return e
}

func register(t *testing.T, e *env) *AuthResult {
	t.Helper()
	res, err := e.svc.Register(context.Background(), RegisterInput{
		Email: "  Jane@Example.com ", Password: "s3cretpass", FirstName: " Jane ",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return res
}

func TestRegister_NormalizesAndAssignsCustomerRole(t *testing.T) {
	e := newEnv(t)
	res := register(t, e)

	if res.User.Email != "jane@example.com" {
		t.Errorf("email = %q, want normalized", res.User.Email)
	}
	if res.User.Role != domain.RoleCustomer {
		t.Errorf("role = %q, want customer", res.User.Role)
	}
	if res.User.FirstName != "Jane" {
		t.Errorf("first name = %q, want trimmed", res.User.FirstName)
	}
	if res.User.PasswordHash == "s3cretpass" {
		t.Error("password stored in plaintext")
	}
	if res.Tokens.AccessToken == "" || res.Tokens.RefreshToken == "" {
		t.Error("expected both tokens")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	e := newEnv(t)
	register(t, e)

	_, err := e.svc.Register(context.Background(), RegisterInput{Email: "JANE@example.com", Password: "anotherpass"})
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}
}

func TestRegister_PasswordTooLong(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Register(context.Background(), RegisterInput{
		Email: "a@b.com", Password: strings.Repeat("x", 73),
	})
	if !errors.Is(err, domain.ErrPasswordTooLong) {
		t.Fatalf("err = %v, want ErrPasswordTooLong", err)
	}
}

func TestLogin(t *testing.T) {
	e := newEnv(t)
	register(t, e)
	ctx := context.Background()

	if _, err := e.svc.Login(ctx, "jane@example.com", "s3cretpass"); err != nil {
		t.Errorf("valid login failed: %v", err)
	}
	if _, err := e.svc.Login(ctx, "jane@example.com", "wrong"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("wrong password err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := e.svc.Login(ctx, "nobody@example.com", "s3cretpass"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("unknown email err = %v, want ErrInvalidCredentials (must not leak existence)", err)
	}
}

func TestRefresh_RotatesToken(t *testing.T) {
	e := newEnv(t)
	res := register(t, e)
	ctx := context.Background()

	pair, err := e.svc.Refresh(ctx, res.Tokens.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if pair.RefreshToken == res.Tokens.RefreshToken {
		t.Error("refresh token was not rotated")
	}
	if got := e.tokens.activeTokens(res.User.ID); got != 1 {
		t.Errorf("active tokens = %d, want 1", got)
	}
}

func TestRefresh_ReuseRevokesAllSessions(t *testing.T) {
	e := newEnv(t)
	res := register(t, e)
	ctx := context.Background()

	rotated, err := e.svc.Refresh(ctx, res.Tokens.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := e.svc.Refresh(ctx, res.Tokens.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("reused token err = %v, want ErrInvalidRefreshToken", err)
	}
	if got := e.tokens.activeTokens(res.User.ID); got != 0 {
		t.Errorf("active tokens after reuse = %d, want 0", got)
	}
	if _, err := e.svc.Refresh(ctx, rotated.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Errorf("descendant token should be revoked, err = %v", err)
	}
}

func TestRefresh_ExpiredAndUnknown(t *testing.T) {
	e := newEnv(t)
	res := register(t, e)
	ctx := context.Background()

	if _, err := e.svc.Refresh(ctx, "does-not-exist"); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Errorf("unknown token err = %v", err)
	}

	e.clock = e.clock.Add(25 * time.Hour)
	if _, err := e.svc.Refresh(ctx, res.Tokens.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Errorf("expired token err = %v", err)
	}
}

func TestLogout_RevokesAndIsIdempotent(t *testing.T) {
	e := newEnv(t)
	res := register(t, e)
	ctx := context.Background()

	if err := e.svc.Logout(ctx, res.Tokens.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if err := e.svc.Logout(ctx, res.Tokens.RefreshToken); err != nil {
		t.Errorf("second logout: %v", err)
	}
	if err := e.svc.Logout(ctx, "unknown"); err != nil {
		t.Errorf("logout with unknown token: %v", err)
	}
	if _, err := e.svc.Refresh(ctx, res.Tokens.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Errorf("refresh after logout err = %v", err)
	}
}
