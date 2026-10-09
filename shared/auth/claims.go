package auth

import "github.com/golang-jwt/jwt/v5"

const (
	RoleCustomer = "customer"
	RoleAdmin    = "admin"

	Issuer = "rovena-identity"
)

// Claims is the access token payload. Subject holds the user ID.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func (c *Claims) UserID() string { return c.Subject }
