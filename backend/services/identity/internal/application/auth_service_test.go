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
	e.tokens.now = svc.now
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

// ---------------------------------------------------------------- grace period for a second tab

func withGrace(t *testing.T, grace time.Duration) (*env, *AuthResult) {
	t.Helper()
	e := newEnv(t)
	e.svc.WithRefreshGrace(grace)
	return e, register(t, e)
}

func TestRefresh_SecondTabWithinTheGraceIsNotTheft(t *testing.T) {
	e, res := withGrace(t, 10*time.Second)
	ctx := context.Background()

	tabA, err := e.svc.Refresh(ctx, res.Tokens.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	e.clock = e.clock.Add(2 * time.Second)
	tabB, err := e.svc.Refresh(ctx, res.Tokens.RefreshToken) // the other tab, same old token
	if err != nil {
		t.Fatalf("a second tab must not be treated as theft: %v", err)
	}
	if tabB.RefreshToken == tabA.RefreshToken || tabB.RefreshToken == res.Tokens.RefreshToken {
		t.Fatal("each tab needs a token of its own")
	}
	// Both sessions stay usable.
	for name, token := range map[string]string{"tab A": tabA.RefreshToken, "tab B": tabB.RefreshToken} {
		if _, err := e.svc.Refresh(ctx, token); err != nil {
			t.Errorf("%s lost its session: %v", name, err)
		}
	}
}

func TestRefresh_ReplayAfterTheGraceIsStillTheft(t *testing.T) {
	e, res := withGrace(t, 10*time.Second)
	ctx := context.Background()
	rotated, _ := e.svc.Refresh(ctx, res.Tokens.RefreshToken)

	e.clock = e.clock.Add(11 * time.Second)
	if _, err := e.svc.Refresh(ctx, res.Tokens.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("err = %v", err)
	}
	if e.tokens.activeTokens(res.User.ID) != 0 {
		t.Fatal("a replay after the grace must end every session")
	}
	if _, err := e.svc.Refresh(ctx, rotated.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatal("the rotated token must be revoked as well")
	}
}

func TestRefresh_GraceIsOffByDefault(t *testing.T) {
	e := newEnv(t)
	res := register(t, e)
	_, _ = e.svc.Refresh(context.Background(), res.Tokens.RefreshToken)
	if _, err := e.svc.Refresh(context.Background(), res.Tokens.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("err = %v", err)
	}
}

// A logout revokes the token but is not a rotation: it can never be presented again.
func TestRefresh_ALoggedOutTokenGetsNoGrace(t *testing.T) {
	e, res := withGrace(t, 10*time.Second)
	ctx := context.Background()
	if err := e.svc.Logout(ctx, res.Tokens.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Refresh(ctx, res.Tokens.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("a logged-out token must stay dead, err = %v", err)
	}
}

func TestRefresh_ExpiredTokenGetsNoGraceEither(t *testing.T) {
	e, res := withGrace(t, time.Hour)
	ctx := context.Background()
	// Rotated one minute before it expires, then presented again after it expired: the rotation
	// is well inside the hour of grace, but an expired token is dead whatever happened to it.
	e.clock = e.clock.Add(24*time.Hour - time.Minute)
	if _, err := e.svc.Refresh(ctx, res.Tokens.RefreshToken); err != nil {
		t.Fatal(err)
	}
	e.clock = e.clock.Add(2 * time.Minute)
	if _, err := e.svc.Refresh(ctx, res.Tokens.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("err = %v", err)
	}
}

func TestRefresh_ConcurrentRefreshesOfOneTokenAllSucceedWithinTheGrace(t *testing.T) {
	e, res := withGrace(t, 10*time.Second)
	const n = 8
	results := make(chan error, n)
	tokens := make(chan string, n)
	for i := 0; i < n; i++ {
		go func() {
			pair, err := e.svc.Refresh(context.Background(), res.Tokens.RefreshToken)
			if err == nil {
				tokens <- pair.RefreshToken // before reporting, so the channel is complete when all results are in
			}
			results <- err
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-results; err != nil {
			t.Fatalf("a concurrent refresh failed: %v", err)
		}
	}
	close(tokens)
	seen := map[string]bool{}
	for tk := range tokens {
		if seen[tk] {
			t.Fatal("two refreshes got the same token")
		}
		seen[tk] = true
	}
	if got := e.tokens.activeTokens(res.User.ID); got != n {
		t.Fatalf("active tokens = %d, want %d", got, n)
	}
}

func TestChangePassword_RevokesOldRefreshTokens(t *testing.T) {
	e := newEnv(t)
	res := register(t, e)
	ctx := context.Background()

	if _, err := e.svc.ChangePassword(ctx, res.User.ID, "wrong-pass", "n3wsecretpass"); !errors.Is(err, domain.ErrWrongPassword) {
		t.Fatalf("wrong current: err = %v", err)
	}
	if _, err := e.svc.ChangePassword(ctx, res.User.ID, "s3cretpass", "s3cretpass"); !errors.Is(err, domain.ErrSamePassword) {
		t.Fatalf("same password: err = %v", err)
	}
	pair, err := e.svc.ChangePassword(ctx, res.User.ID, "s3cretpass", "n3wsecretpass")
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	if pair.RefreshToken == "" || pair.RefreshToken == res.Tokens.RefreshToken {
		t.Error("expected a fresh refresh token")
	}
	if _, err := e.svc.Refresh(ctx, res.Tokens.RefreshToken); err == nil {
		t.Error("old refresh token must be revoked")
	}
	if _, err := e.svc.Login(ctx, "jane@example.com", "n3wsecretpass"); err != nil {
		t.Errorf("login with new password: %v", err)
	}
}

func TestChangeEmail(t *testing.T) {
	e := newEnv(t)
	res := register(t, e)
	ctx := context.Background()

	if _, err := e.svc.ChangeEmail(ctx, res.User.ID, "x@example.com", "wrong-pass"); !errors.Is(err, domain.ErrWrongPassword) {
		t.Fatalf("wrong password: err = %v", err)
	}
	if _, err := e.svc.ChangeEmail(ctx, res.User.ID, " JANE@example.com ", "s3cretpass"); !errors.Is(err, domain.ErrEmailUnchanged) {
		t.Fatalf("unchanged: err = %v", err)
	}
	u, err := e.svc.ChangeEmail(ctx, res.User.ID, " New@Example.com ", "s3cretpass")
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	if u.Email != "new@example.com" {
		t.Errorf("email = %q", u.Email)
	}
}
