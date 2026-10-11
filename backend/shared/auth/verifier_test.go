package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secret = []byte("0123456789abcdef0123456789abcdef")

func sign(t *testing.T, method jwt.SigningMethod, key any, c Claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, c).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validClaims() Claims {
	return Claims{
		Role: RoleCustomer,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}
}

func TestVerify(t *testing.T) {
	v := NewVerifier(secret)

	expired := validClaims()
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	wrongIssuer := validClaims()
	wrongIssuer.Issuer = "someone-else"
	noSubject := validClaims()
	noSubject.Subject = ""
	noExp := validClaims()
	noExp.ExpiresAt = nil

	tests := []struct {
		name  string
		token string
		ok    bool
	}{
		{"valid", sign(t, jwt.SigningMethodHS256, secret, validClaims()), true},
		{"wrong secret", sign(t, jwt.SigningMethodHS256, []byte("another-secret-another-secret-xx"), validClaims()), false},
		{"expired", sign(t, jwt.SigningMethodHS256, secret, expired), false},
		{"wrong issuer", sign(t, jwt.SigningMethodHS256, secret, wrongIssuer), false},
		{"missing subject", sign(t, jwt.SigningMethodHS256, secret, noSubject), false},
		{"missing expiry", sign(t, jwt.SigningMethodHS256, secret, noExp), false},
		{"other hmac alg", sign(t, jwt.SigningMethodHS512, secret, validClaims()), false},
		{"alg none", sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, validClaims()), false},
		{"garbage", "not.a.token", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := v.Verify(tt.token)
			if tt.ok != (err == nil) {
				t.Fatalf("ok=%v err=%v", tt.ok, err)
			}
			if tt.ok && (claims.UserID() != "user-1" || claims.Role != RoleCustomer) {
				t.Errorf("unexpected claims: %+v", claims)
			}
		})
	}
}
