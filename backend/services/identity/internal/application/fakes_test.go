package application

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/Furkanturan8/rovena/services/identity/internal/repository"
	"github.com/google/uuid"
)

type fakeUsers struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*domain.User
	hashes map[string]uuid.UUID
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{byID: map[uuid.UUID]*domain.User{}, hashes: map[string]uuid.UUID{}}
}

func (f *fakeUsers) Create(_ context.Context, u *domain.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.hashes[u.Email]; ok {
		return domain.ErrEmailTaken
	}
	cp := *u
	f.byID[u.ID] = &cp
	f.hashes[u.Email] = u.ID
	return nil
}

func (f *fakeUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	cp := *u
	return &cp, nil
}

func (f *fakeUsers) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	f.mu.Lock()
	id, ok := f.hashes[email]
	f.mu.Unlock()
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return f.GetByID(ctx, id)
}

func (f *fakeUsers) List(context.Context, repository.ListUsersParams) ([]domain.User, int, error) {
	return nil, 0, errors.New("not implemented")
}

func (f *fakeUsers) UpdateProfile(_ context.Context, u *domain.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *u
	f.byID[u.ID] = &cp
	return nil
}

// UpdateRole mirrors the repository contract, including the last-admin rule.
func (f *fakeUsers) UpdateRole(_ context.Context, id uuid.UUID, role domain.Role) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return domain.ErrUserNotFound
	}
	if u.Role == domain.RoleAdmin && role != domain.RoleAdmin {
		admins := 0
		for _, o := range f.byID {
			if o.Role == domain.RoleAdmin {
				admins++
			}
		}
		if admins <= 1 {
			return domain.ErrLastAdmin
		}
	}
	u.Role = role
	return nil
}

type fakeTokens struct {
	mu     sync.Mutex
	byHash map[string]*domain.RefreshToken
}

func newFakeTokens() *fakeTokens {
	return &fakeTokens{byHash: map[string]*domain.RefreshToken{}}
}

func (f *fakeTokens) Create(_ context.Context, t *domain.RefreshToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *t
	f.byHash[t.TokenHash] = &cp
	return nil
}

func (f *fakeTokens) GetByHash(_ context.Context, hash string) (*domain.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.byHash[hash]
	if !ok {
		return nil, domain.ErrInvalidRefreshToken
	}
	cp := *t
	return &cp, nil
}

func (f *fakeTokens) Revoke(_ context.Context, id uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.byHash {
		if t.ID == id {
			if t.RevokedAt != nil {
				return false, nil
			}
			now := time.Now()
			t.RevokedAt = &now
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeTokens) RevokeAllForUser(_ context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	for _, t := range f.byHash {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &now
		}
	}
	return nil
}

// activeTokens counts non-revoked refresh tokens for a user.
func (f *fakeTokens) activeTokens(userID uuid.UUID) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, t := range f.byHash {
		if t.UserID == userID && t.RevokedAt == nil {
			n++
		}
	}
	return n
}

type plainHasher struct{}

func (plainHasher) Hash(p string) (string, error) { return "hashed:" + p, nil }
func (plainHasher) Compare(hash, p string) error {
	if hash != "hashed:"+p {
		return errors.New("mismatch")
	}
	return nil
}

type stubIssuer struct{}

func (stubIssuer) Issue(u *domain.User) (string, time.Time, error) {
	return "access-" + u.ID.String(), time.Now().Add(15 * time.Minute), nil
}
