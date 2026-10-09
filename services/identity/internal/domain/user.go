package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleCustomer Role = "customer"
	RoleAdmin    Role = "admin"
)

func (r Role) Valid() bool { return r == RoleCustomer || r == RoleAdmin }

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
	Role         Role
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (t *RefreshToken) Expired(now time.Time) bool { return !now.Before(t.ExpiresAt) }

var (
	ErrEmailTaken          = errors.New("email already registered")
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrInvalidRole         = errors.New("invalid role")
	ErrLastAdmin           = errors.New("cannot demote the last admin")
	ErrPasswordTooLong     = errors.New("password exceeds 72 bytes")
)
