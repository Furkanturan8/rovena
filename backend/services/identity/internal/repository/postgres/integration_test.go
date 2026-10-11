//go:build integration

package postgres_test

// Runs against a real PostgreSQL:
//   IDENTITY_TEST_DATABASE_URL=postgres://... go test -tags integration ./internal/repository/postgres
// Each test gets its own throw-away schema with the migrations applied, so tests are isolated
// from each other and from the development data, and global rules ("at least one admin") can be tested.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/application"
	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/Furkanturan8/rovena/services/identity/internal/infrastructure/security"
	"github.com/Furkanturan8/rovena/services/identity/internal/repository"
	"github.com/Furkanturan8/rovena/services/identity/internal/repository/postgres"
	"github.com/Furkanturan8/rovena/services/identity/migrations"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

type harness struct {
	ctx    context.Context
	db     *bun.DB
	users  *postgres.UserRepository
	tokens *postgres.RefreshTokenRepository
	auth   *application.AuthService
	svc    *application.UserService
}

func setup(t *testing.T) *harness {
	t.Helper()
	dsn := os.Getenv("IDENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("IDENTITY_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	schema := "itest_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	admin := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close()
	})

	sqldb := sql.OpenDB(pgdriver.NewConnector(
		pgdriver.WithDSN(dsn),
		pgdriver.WithConnParams(map[string]interface{}{"search_path": schema}),
	))
	sqldb.SetMaxOpenConns(20)
	t.Cleanup(func() { _ = sqldb.Close() })

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(sqldb, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db := bun.NewDB(sqldb, pgdialect.New())
	h := &harness{ctx: ctx, db: db, users: postgres.NewUserRepository(db), tokens: postgres.NewRefreshTokenRepository(db)}
	auth, err := application.NewAuthService(h.users, h.tokens, security.NewBcryptHasher(4),
		security.NewJWTIssuer("itest-secret-itest-secret-itest-secret", time.Minute), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h.auth = auth
	h.svc = application.NewUserService(h.users)
	return h
}

// register creates a customer through the real AuthService.
func (h *harness) register(t *testing.T, email string) *application.AuthResult {
	t.Helper()
	res, err := h.auth.Register(h.ctx, application.RegisterInput{Email: email, Password: "s3cretpass1", FirstName: "T"})
	if err != nil {
		t.Fatalf("register %s: %v", email, err)
	}
	return res
}

func (h *harness) makeAdmin(t *testing.T, email string) *domain.User {
	t.Helper()
	u := h.register(t, email).User
	if _, err := h.svc.ChangeRole(h.ctx, u.ID, domain.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	return u
}

func (h *harness) countRole(t *testing.T, role domain.Role) int {
	t.Helper()
	n, err := h.db.NewSelect().TableExpr("users").Where("role = ?", string(role)).Count(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	return int(n)
}

// ------------------------------------------------------------------ users

func TestUserRepository_CreateAndLookups(t *testing.T) {
	h := setup(t)
	res := h.register(t, "Jane@Example.com")

	byEmail, err := h.users.GetByEmail(h.ctx, "jane@example.com")
	if err != nil || byEmail.ID != res.User.ID || byEmail.Role != domain.RoleCustomer || byEmail.FirstName != "T" {
		t.Fatalf("GetByEmail = %+v, %v", byEmail, err)
	}
	if byEmail.PasswordHash == "" || byEmail.PasswordHash == "s3cretpass1" {
		t.Errorf("password hash = %q", byEmail.PasswordHash)
	}
	byID, err := h.users.GetByID(h.ctx, res.User.ID)
	if err != nil || byID.Email != "jane@example.com" {
		t.Fatalf("GetByID = %+v, %v", byID, err)
	}

	if _, err := h.users.GetByEmail(h.ctx, "nobody@example.com"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("unknown email err = %v", err)
	}
	if _, err := h.users.GetByID(h.ctx, uuid.New()); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("unknown id err = %v", err)
	}
}

func TestUserRepository_DuplicateEmail(t *testing.T) {
	h := setup(t)
	h.register(t, "dup@example.com")

	// Same address, different case: the service normalizes, so it must be seen as a duplicate.
	_, err := h.auth.Register(h.ctx, application.RegisterInput{Email: "  DUP@Example.COM ", Password: "s3cretpass1"})
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}

	// Bypassing the service: the database itself refuses a non-normalized address.
	id, _ := uuid.NewV7()
	raw := &domain.User{ID: id, Email: "Mixed@Example.com", PasswordHash: "x", Role: domain.RoleCustomer, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := h.users.Create(h.ctx, raw); err == nil {
		t.Error("database accepted a mixed-case email")
	}
}

func TestUserRepository_UpdateProfileAndRole(t *testing.T) {
	h := setup(t)
	u := h.register(t, "p@example.com").User

	u.FirstName, u.LastName = "Jane", "Doe"
	if err := h.users.UpdateProfile(h.ctx, u); err != nil {
		t.Fatal(err)
	}
	got, _ := h.users.GetByID(h.ctx, u.ID)
	if got.FirstName != "Jane" || got.LastName != "Doe" || got.Email != "p@example.com" || got.Role != domain.RoleCustomer {
		t.Errorf("after profile update: %+v", got)
	}
	if !got.UpdatedAt.After(got.CreatedAt) {
		t.Errorf("updated_at %v not after created_at %v", got.UpdatedAt, got.CreatedAt)
	}

	if err := h.users.UpdateRole(h.ctx, u.ID, domain.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if got, _ := h.users.GetByID(h.ctx, u.ID); got.Role != domain.RoleAdmin {
		t.Errorf("role = %q", got.Role)
	}
	if err := h.users.UpdateRole(h.ctx, uuid.New(), domain.RoleAdmin); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("unknown user err = %v", err)
	}
	// The role CHECK constraint rejects anything but the known roles.
	if err := h.users.UpdateRole(h.ctx, u.ID, "superuser"); err == nil {
		t.Error("database accepted an unknown role")
	}
}

func TestUserRepository_ListPagination(t *testing.T) {
	h := setup(t)
	var emails []string
	for i := 0; i < 5; i++ {
		e := fmt.Sprintf("u%d@example.com", i)
		h.register(t, e)
		emails = append(emails, e)
	}
	page := func(n, size int) ([]string, int) {
		users, total, err := h.users.List(h.ctx, repository.ListUsersParams{Page: n, PageSize: size})
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(users))
		for i, u := range users {
			out[i] = u.Email
		}
		return out, total
	}
	// Newest first.
	p1, total := page(1, 2)
	if total != 5 || fmt.Sprint(p1) != fmt.Sprint([]string{emails[4], emails[3]}) {
		t.Errorf("page 1 = %v total %d", p1, total)
	}
	if p3, _ := page(3, 2); fmt.Sprint(p3) != fmt.Sprint([]string{emails[0]}) {
		t.Errorf("page 3 = %v", p3)
	}
	if p4, total := page(4, 2); len(p4) != 0 || total != 5 {
		t.Errorf("page 4 = %v total %d, want empty with total 5", p4, total)
	}
}

// ------------------------------------------------------------------ refresh tokens

func TestRefreshTokenRepository(t *testing.T) {
	h := setup(t)
	u := h.register(t, "t@example.com").User
	id, _ := uuid.NewV7()
	now := time.Now()
	tok := &domain.RefreshToken{ID: id, UserID: u.ID, TokenHash: "hash-1", ExpiresAt: now.Add(time.Hour), CreatedAt: now}

	if err := h.tokens.Create(h.ctx, tok); err != nil {
		t.Fatal(err)
	}
	got, err := h.tokens.GetByHash(h.ctx, "hash-1")
	if err != nil || got.ID != id || got.UserID != u.ID || got.RevokedAt != nil || got.Expired(now) {
		t.Fatalf("GetByHash = %+v, %v", got, err)
	}
	if _, err := h.tokens.GetByHash(h.ctx, "nope"); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Errorf("unknown hash err = %v", err)
	}

	id2, _ := uuid.NewV7()
	dup := &domain.RefreshToken{ID: id2, UserID: u.ID, TokenHash: "hash-1", ExpiresAt: now.Add(time.Hour), CreatedAt: now}
	if err := h.tokens.Create(h.ctx, dup); err == nil {
		t.Error("duplicate token hash accepted")
	}

	first, err := h.tokens.Revoke(h.ctx, id)
	second, err2 := h.tokens.Revoke(h.ctx, id)
	if err != nil || err2 != nil || !first || second {
		t.Errorf("Revoke twice = %v/%v (errs %v, %v), want true then false", first, second, err, err2)
	}
	if got, _ := h.tokens.GetByHash(h.ctx, "hash-1"); got.RevokedAt == nil {
		t.Error("revoked_at not set")
	}
}

func TestRefreshTokenRepository_RevokeAllAndCascade(t *testing.T) {
	h := setup(t)
	a, b := h.register(t, "a@example.com"), h.register(t, "b@example.com")
	// register issued one token each; add a second for a.
	if _, err := h.auth.Login(h.ctx, "a@example.com", "s3cretpass1"); err != nil {
		t.Fatal(err)
	}
	if err := h.tokens.RevokeAllForUser(h.ctx, a.User.ID); err != nil {
		t.Fatal(err)
	}
	active := func(uid uuid.UUID) int {
		n, _ := h.db.NewSelect().TableExpr("refresh_tokens").Where("user_id = ? AND revoked_at IS NULL", uid).Count(h.ctx)
		return int(n)
	}
	if active(a.User.ID) != 0 || active(b.User.ID) != 1 {
		t.Errorf("active tokens a=%d b=%d, want 0 and 1 (other users untouched)", active(a.User.ID), active(b.User.ID))
	}

	if _, err := h.db.ExecContext(h.ctx, "DELETE FROM users WHERE id = ?", b.User.ID); err != nil {
		t.Fatal(err)
	}
	var left int
	_ = h.db.NewSelect().TableExpr("refresh_tokens").ColumnExpr("count(*)").Where("user_id = ?", b.User.ID).Scan(h.ctx, &left)
	if left != 0 {
		t.Errorf("%d tokens survived user deletion", left)
	}
}

// ------------------------------------------------------------------ races (real repositories)

func runConcurrently(n int, fn func(i int)) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			fn(i)
		}()
	}
	close(start)
	wg.Wait()
}

func TestConcurrentRegistration_SameEmail(t *testing.T) {
	h := setup(t)
	const n = 8
	errs := make([]error, n)
	runConcurrently(n, func(i int) {
		_, errs[i] = h.auth.Register(h.ctx, application.RegisterInput{Email: "race@example.com", Password: "s3cretpass1"})
	})
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, domain.ErrEmailTaken):
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Errorf("%d registrations succeeded, want exactly 1", ok)
	}
	if n, _ := h.db.NewSelect().TableExpr("users").Count(h.ctx); n != 1 {
		t.Errorf("%d users stored", n)
	}
}

func TestConcurrentRefresh_SameToken(t *testing.T) {
	h := setup(t)
	res := h.register(t, "r@example.com")

	const n = 8
	pairs := make([]*application.TokenPair, n)
	errs := make([]error, n)
	runConcurrently(n, func(i int) {
		pairs[i], errs[i] = h.auth.Refresh(h.ctx, res.Tokens.RefreshToken)
	})
	ok := 0
	for i := range errs {
		switch {
		case errs[i] == nil:
			ok++
		case !errors.Is(errs[i], domain.ErrInvalidRefreshToken):
			t.Errorf("unexpected error: %v", errs[i])
		}
	}
	if ok > 1 {
		t.Fatalf("%d concurrent refreshes of one token succeeded, want at most 1", ok)
	}
	if ok == 0 {
		t.Log("all attempts were rejected as token reuse (acceptable)")
	}
}

func TestRefreshReuse_RevokesTheWholeFamily(t *testing.T) {
	h := setup(t)
	res := h.register(t, "reuse@example.com")

	next, err := h.auth.Refresh(h.ctx, res.Tokens.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.auth.Refresh(h.ctx, res.Tokens.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("reuse err = %v", err)
	}
	if _, err := h.auth.Refresh(h.ctx, next.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Errorf("token issued before the reuse must be dead too, err = %v", err)
	}
}

func TestLastAdmin_SingleAdminCannotBeDemoted(t *testing.T) {
	h := setup(t)
	admin := h.makeAdmin(t, "root@example.com")
	if _, err := h.svc.ChangeRole(h.ctx, admin.ID, domain.RoleCustomer); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("err = %v, want ErrLastAdmin", err)
	}
	if h.countRole(t, domain.RoleAdmin) != 1 {
		t.Error("admin count changed")
	}
}

// Two admins demote each other at the same moment. The system must never end up without an admin.
func TestLastAdmin_ConcurrentMutualDemotion(t *testing.T) {
	h := setup(t)
	rounds := 300
	if testing.Short() {
		rounds = 30
	}
	for round := 0; round < rounds; round++ {
		if _, err := h.db.ExecContext(h.ctx, "DELETE FROM users"); err != nil {
			t.Fatal(err)
		}
		a := h.makeAdmin(t, "a@example.com")
		b := h.makeAdmin(t, "b@example.com")

		errs := make([]error, 2)
		targets := []uuid.UUID{b.ID, a.ID}
		runConcurrently(2, func(i int) {
			_, errs[i] = h.svc.ChangeRole(h.ctx, targets[i], domain.RoleCustomer)
		})

		if got := h.countRole(t, domain.RoleAdmin); got != 1 {
			t.Fatalf("round %d: %d admins left, want 1 (errs: %v, %v)", round, got, errs[0], errs[1])
		}
		rejected := 0
		for _, err := range errs {
			if errors.Is(err, domain.ErrLastAdmin) {
				rejected++
			} else if err != nil {
				t.Fatalf("round %d: unexpected error %v", round, err)
			}
		}
		if rejected != 1 {
			t.Fatalf("round %d: %d requests rejected, want exactly 1", round, rejected)
		}
	}
}

func TestRefreshTokenRepository_RotateIsRecordedAndOnlyOnce(t *testing.T) {
	h := setup(t)
	u := h.register(t, "rot@example.com").User
	now := time.Now()
	rotated, loggedOut := uuid.New(), uuid.New()
	for id, hash := range map[uuid.UUID]string{rotated: "h-rot", loggedOut: "h-out"} {
		if err := h.tokens.Create(h.ctx, &domain.RefreshToken{ID: id, UserID: u.ID, TokenHash: hash, ExpiresAt: now.Add(time.Hour), CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := h.tokens.Rotate(h.ctx, rotated)
	second, err2 := h.tokens.Rotate(h.ctx, rotated)
	if err != nil || err2 != nil || !first || second {
		t.Fatalf("Rotate twice = %v/%v (%v, %v), want true then false", first, second, err, err2)
	}
	if got, _ := h.tokens.GetByHash(h.ctx, "h-rot"); got.RevokedAt == nil || got.RotatedAt == nil {
		t.Fatalf("a rotation must set both revoked_at and rotated_at: %+v", got)
	}
	if ok, _ := h.tokens.Revoke(h.ctx, loggedOut); !ok {
		t.Fatal("revoke failed")
	}
	if got, _ := h.tokens.GetByHash(h.ctx, "h-out"); got.RevokedAt == nil || got.RotatedAt != nil {
		t.Fatalf("a logout is not a rotation: %+v", got)
	}
	if again, _ := h.tokens.Rotate(h.ctx, loggedOut); again {
		t.Fatal("a revoked token can not be rotated")
	}
}

// Two tabs refresh with the same token at the same moment. Without the grace one of them would
// be taken for a thief and every session would end; with it they all get working tokens.
func TestConcurrentRefreshWithGrace_NobodyIsLoggedOut(t *testing.T) {
	h := setup(t)
	h.auth.WithRefreshGrace(10 * time.Second)
	res := h.register(t, "grace@example.com")

	const n = 8
	pairs := make([]*application.TokenPair, n)
	errs := make([]error, n)
	runConcurrently(n, func(i int) {
		pairs[i], errs[i] = h.auth.Refresh(h.ctx, res.Tokens.RefreshToken)
	})
	seen := map[string]bool{}
	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("refresh %d failed: %v", i, errs[i])
		}
		seen[pairs[i].RefreshToken] = true
	}
	if len(seen) != n {
		t.Fatalf("%d distinct tokens for %d refreshes", len(seen), n)
	}
	for tk := range seen {
		if _, err := h.auth.Refresh(h.ctx, tk); err != nil {
			t.Fatalf("a token issued during the burst is dead: %v", err)
		}
	}
}

func TestRefreshGrace_ReplayAfterItEndsStillKillsTheFamily(t *testing.T) {
	h := setup(t)
	h.auth.WithRefreshGrace(time.Nanosecond) // expires immediately
	res := h.register(t, "late@example.com")
	next, err := h.auth.Refresh(h.ctx, res.Tokens.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := h.auth.Refresh(h.ctx, res.Tokens.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("err = %v", err)
	}
	if _, err := h.auth.Refresh(h.ctx, next.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatal("the whole family must be dead after a late replay")
	}
}
