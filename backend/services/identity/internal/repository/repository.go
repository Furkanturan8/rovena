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
	// UpdateRole returns ErrUserNotFound for an unknown id and ErrLastAdmin when it would
	// remove the last admin. The check and the write happen atomically, even across
	// concurrent callers.
	UpdateRole(ctx context.Context, id uuid.UUID, role domain.Role) error
	// UpdateEmail returns ErrEmailTaken when another account already uses the email.
	UpdateEmail(ctx context.Context, id uuid.UUID, email string) error
	UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error
}

type AddressRepository interface {
	List(ctx context.Context, userID uuid.UUID) ([]domain.Address, error)
	// Create stores the address. The user's first address becomes the default; a new default
	// replaces the old one. Returns ErrTooManyAddresses beyond domain.MaxAddressesPerUser.
	Create(ctx context.Context, a *domain.Address) error
	// Update changes the editable fields (and the default flag when MakeDefault is set).
	// ErrAddressNotFound covers unknown ids and addresses that belong to someone else.
	Update(ctx context.Context, a *domain.Address, makeDefault bool) error
	// Delete removes the address; when it was the default, the newest remaining one takes over.
	Delete(ctx context.Context, userID, id uuid.UUID) error
	SetDefault(ctx context.Context, userID, id uuid.UUID) (*domain.Address, error)
	Get(ctx context.Context, userID, id uuid.UUID) (*domain.Address, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, t *domain.RefreshToken) error
	// GetByHash returns domain.ErrInvalidRefreshToken when no row matches.
	GetByHash(ctx context.Context, hash string) (*domain.RefreshToken, error)
	// Revoke marks the token revoked and reports whether this call did it.
	// false means it was already revoked, which callers treat as token reuse.
	Revoke(ctx context.Context, id uuid.UUID) (bool, error)
	// Rotate revokes the token because a refresh is replacing it, and reports whether this call did it.
	// false means someone revoked it first.
	Rotate(ctx context.Context, id uuid.UUID) (bool, error)
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}
