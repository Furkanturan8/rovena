package application

import (
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	// Compare returns nil when password matches hash.
	Compare(hash, password string) error
}

type AccessTokenIssuer interface {
	Issue(u *domain.User) (token string, expiresAt time.Time, err error)
}

type TokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}
