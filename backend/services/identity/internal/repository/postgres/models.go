package postgres

import (
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type userRow struct {
	bun.BaseModel `bun:"table:users"`

	ID           uuid.UUID `bun:",pk,type:uuid"`
	Email        string    `bun:",notnull"`
	PasswordHash string    `bun:",notnull"`
	FirstName    string    `bun:",notnull"`
	LastName     string    `bun:",notnull"`
	Role         string    `bun:",notnull"`
	CreatedAt    time.Time `bun:",notnull"`
	UpdatedAt    time.Time `bun:",notnull"`
}

func (r *userRow) toDomain() *domain.User {
	return &domain.User{
		ID: r.ID, Email: r.Email, PasswordHash: r.PasswordHash,
		FirstName: r.FirstName, LastName: r.LastName, Role: domain.Role(r.Role),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func userRowFrom(u *domain.User) *userRow {
	return &userRow{
		ID: u.ID, Email: u.Email, PasswordHash: u.PasswordHash,
		FirstName: u.FirstName, LastName: u.LastName, Role: string(u.Role),
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

type refreshTokenRow struct {
	bun.BaseModel `bun:"table:refresh_tokens"`

	ID        uuid.UUID  `bun:",pk,type:uuid"`
	UserID    uuid.UUID  `bun:",type:uuid,notnull"`
	TokenHash string     `bun:",notnull"`
	ExpiresAt time.Time  `bun:",notnull"`
	RevokedAt *time.Time `bun:",nullzero"`
	RotatedAt *time.Time `bun:",nullzero"`
	CreatedAt time.Time  `bun:",notnull"`
}

func (r *refreshTokenRow) toDomain() *domain.RefreshToken {
	return &domain.RefreshToken{
		ID: r.ID, UserID: r.UserID, TokenHash: r.TokenHash,
		ExpiresAt: r.ExpiresAt, RevokedAt: r.RevokedAt, RotatedAt: r.RotatedAt, CreatedAt: r.CreatedAt,
	}
}
