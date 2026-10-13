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

type addressRow struct {
	bun.BaseModel `bun:"table:addresses,alias:a"`

	ID         uuid.UUID `bun:",pk,type:uuid"`
	UserID     uuid.UUID `bun:",type:uuid,notnull"`
	Title      string    `bun:",notnull"`
	FullName   string    `bun:",notnull"`
	Phone      string    `bun:",notnull"`
	Line1      string    `bun:"line1,notnull"`
	Line2      string    `bun:"line2,notnull"`
	District   string    `bun:",notnull"`
	City       string    `bun:",notnull"`
	PostalCode string    `bun:",notnull"`
	Country    string    `bun:",notnull"`
	IsDefault  bool      `bun:",notnull"`
	CreatedAt  time.Time `bun:",notnull"`
	UpdatedAt  time.Time `bun:",notnull"`
}

func (r *addressRow) toDomain() *domain.Address {
	return &domain.Address{
		ID: r.ID, UserID: r.UserID, Title: r.Title, FullName: r.FullName, Phone: r.Phone,
		Line1: r.Line1, Line2: r.Line2, District: r.District, City: r.City, PostalCode: r.PostalCode,
		Country: r.Country, IsDefault: r.IsDefault, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func addressRowFrom(a *domain.Address) *addressRow {
	return &addressRow{
		ID: a.ID, UserID: a.UserID, Title: a.Title, FullName: a.FullName, Phone: a.Phone,
		Line1: a.Line1, Line2: a.Line2, District: a.District, City: a.City, PostalCode: a.PostalCode,
		Country: a.Country, IsDefault: a.IsDefault, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}
