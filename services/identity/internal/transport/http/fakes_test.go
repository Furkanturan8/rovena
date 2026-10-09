package http_test

import (
	"context"
	"slices"
	"sync"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/Furkanturan8/rovena/services/identity/internal/repository"
	"github.com/google/uuid"
)

// In-memory repositories that follow the same contracts as the PostgreSQL ones.

type store struct {
	mu     sync.Mutex
	users  map[uuid.UUID]*domain.User
	tokens map[string]*domain.RefreshToken // by hash

	createUserErr error
}

func newStore() *store {
	return &store{users: map[uuid.UUID]*domain.User{}, tokens: map[string]*domain.RefreshToken{}}
}

func (s *store) userCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.users)
}

type userRepo struct{ s *store }

var _ repository.UserRepository = userRepo{}

func (r userRepo) Create(_ context.Context, u *domain.User) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if r.s.createUserErr != nil {
		return r.s.createUserErr
	}
	for _, o := range r.s.users {
		if o.Email == u.Email {
			return domain.ErrEmailTaken
		}
	}
	cp := *u
	r.s.users[u.ID] = &cp
	return nil
}

func (r userRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	u, ok := r.s.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	cp := *u
	return &cp, nil
}

func (r userRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, u := range r.s.users {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (r userRepo) List(_ context.Context, p repository.ListUsersParams) ([]domain.User, int, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	all := make([]domain.User, 0, len(r.s.users))
	for _, u := range r.s.users {
		all = append(all, *u)
	}
	slices.SortFunc(all, func(a, b domain.User) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return slices.Compare(b.ID[:], a.ID[:])
	})
	from := min((p.Page-1)*p.PageSize, len(all))
	to := min(from+p.PageSize, len(all))
	return all[from:to], len(all), nil
}

func (r userRepo) UpdateProfile(_ context.Context, u *domain.User) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	cur, ok := r.s.users[u.ID]
	if !ok {
		return domain.ErrUserNotFound
	}
	cur.FirstName, cur.LastName = u.FirstName, u.LastName
	return nil
}

func (r userRepo) UpdateRole(_ context.Context, id uuid.UUID, role domain.Role) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	u, ok := r.s.users[id]
	if !ok {
		return domain.ErrUserNotFound
	}
	if u.Role == domain.RoleAdmin && role != domain.RoleAdmin {
		admins := 0
		for _, o := range r.s.users {
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

type tokenRepo struct{ s *store }

var _ repository.RefreshTokenRepository = tokenRepo{}

func (r tokenRepo) Create(_ context.Context, t *domain.RefreshToken) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	cp := *t
	r.s.tokens[t.TokenHash] = &cp
	return nil
}

func (r tokenRepo) GetByHash(_ context.Context, hash string) (*domain.RefreshToken, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	t, ok := r.s.tokens[hash]
	if !ok {
		return nil, domain.ErrInvalidRefreshToken
	}
	cp := *t
	return &cp, nil
}

func (r tokenRepo) Revoke(_ context.Context, id uuid.UUID) (bool, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, t := range r.s.tokens {
		if t.ID == id {
			if t.RevokedAt != nil {
				return false, nil
			}
			now := t.CreatedAt
			t.RevokedAt = &now
			return true, nil
		}
	}
	return false, nil
}

func (r tokenRepo) RevokeAllForUser(_ context.Context, userID uuid.UUID) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, t := range r.s.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			now := t.CreatedAt
			t.RevokedAt = &now
		}
	}
	return nil
}
