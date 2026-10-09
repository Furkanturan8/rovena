package repository

import (
	"context"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/google/uuid"
)

type ListUsersParams struct {
	Page     int
	PageSize int
}

type UserRepository interface {
	// Create returns domain.ErrEmailTaken when the email is already registered.
	Create(ctx context.Context, u *domain.User) error
	// GetByID and GetByEmail return domain.ErrUserNotFound when no row matches.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context, p ListUsersParams) ([]domain.User, int, error)
	UpdateProfile(ctx context.Context, u *domain.User) error
	UpdateRole(ctx context.Context, id uuid.UUID, role domain.Role) error
	CountByRole(ctx context.Context, role domain.Role) (int, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, t *domain.RefreshToken) error
	// GetByHash returns domain.ErrInvalidRefreshToken when no row matches.
	GetByHash(ctx context.Context, hash string) (*domain.RefreshToken, error)
	// Revoke marks the token revoked and reports whether this call did it.
	// false means it was already revoked, which callers treat as token reuse.
	Revoke(ctx context.Context, id uuid.UUID) (bool, error)
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}
