package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/Furkanturan8/rovena/services/identity/internal/repository"
	"github.com/google/uuid"
)

type AuthService struct {
	users      repository.UserRepository
	tokens     repository.RefreshTokenRepository
	hasher     PasswordHasher
	issuer     AccessTokenIssuer
	refreshTTL time.Duration
	// refreshGrace is how long after a rotation the old refresh token is still accepted once more.
	refreshGrace time.Duration
	now          func() time.Time

	// dummyHash is compared against when the email is unknown so that login
	// timing does not reveal which emails are registered.
	dummyHash string
}

func NewAuthService(
	users repository.UserRepository,
	tokens repository.RefreshTokenRepository,
	hasher PasswordHasher,
	issuer AccessTokenIssuer,
	refreshTTL time.Duration,
) (*AuthService, error) {
	dummy, err := hasher.Hash("rovena-timing-equalizer")
	if err != nil {
		return nil, fmt.Errorf("prepare dummy hash: %w", err)
	}
	return &AuthService{
		users: users, tokens: tokens, hasher: hasher, issuer: issuer,
		refreshTTL: refreshTTL, now: time.Now, dummyHash: dummy,
	}, nil
}

// WithRefreshGrace lets a refresh token that was rotated less than d ago be presented again without
// being taken for theft; the second caller gets a pair of its own. This is for the benign case of
// two browser tabs refreshing at the same moment: without it the loser's request looks exactly like a
// stolen token being replayed, and every session of the user is closed. The cost is that a thief who
// replays a stolen token within d of its owner is not detected by that replay. Zero (the default)
// turns the allowance off.
func (s *AuthService) WithRefreshGrace(d time.Duration) *AuthService {
	s.refreshGrace = d
	return s
}

type RegisterInput struct {
	Email     string
	Password  string
	FirstName string
	LastName  string
}

type AuthResult struct {
	User   *domain.User
	Tokens TokenPair
}

func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// maxPasswordBytes is bcrypt's input limit.
const maxPasswordBytes = 72

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	if len(in.Password) > maxPasswordBytes {
		return nil, domain.ErrPasswordTooLong
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate user id: %w", err)
	}
	now := s.now()
	user := &domain.User{
		ID:           id,
		Email:        NormalizeEmail(in.Email),
		PasswordHash: hash,
		FirstName:    strings.TrimSpace(in.FirstName),
		LastName:     strings.TrimSpace(in.LastName),
		Role:         domain.RoleCustomer,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	pair, err := s.issuePair(ctx, user)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: user, Tokens: pair}, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	user, err := s.users.GetByEmail(ctx, NormalizeEmail(email))
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			_ = s.hasher.Compare(s.dummyHash, password)
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	if err := s.hasher.Compare(user.PasswordHash, password); err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	pair, err := s.issuePair(ctx, user)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: user, Tokens: pair}, nil
}

// Refresh rotates the refresh token. Presenting an already-revoked token is
// treated as theft: every session of that user is revoked.
func (s *AuthService) Refresh(ctx context.Context, rawToken string) (*TokenPair, error) {
	stored, err := s.tokens.GetByHash(ctx, hashToken(rawToken))
	if err != nil {
		return nil, err
	}
	if stored.RevokedAt != nil {
		return s.reused(ctx, stored)
	}
	if stored.Expired(s.now()) {
		return nil, domain.ErrInvalidRefreshToken
	}
	rotated, err := s.tokens.Rotate(ctx, stored.ID)
	if err != nil {
		return nil, err
	}
	if !rotated {
		// Another request rotated it between our read and our write. Look again to see whether that
		// was a rotation we may tolerate or something worse.
		current, err := s.tokens.GetByHash(ctx, hashToken(rawToken))
		if err != nil {
			return nil, err
		}
		return s.reused(ctx, current)
	}
	return s.pairFor(ctx, stored)
}

// reused handles a refresh token that is no longer active. Shortly after a rotation it is the second
// tab; in every other case it is treated as theft and all sessions of the user end.
func (s *AuthService) reused(ctx context.Context, stored *domain.RefreshToken) (*TokenPair, error) {
	if s.withinGrace(stored) && !stored.Expired(s.now()) {
		return s.pairFor(ctx, stored)
	}
	if err := s.tokens.RevokeAllForUser(ctx, stored.UserID); err != nil {
		return nil, err
	}
	return nil, domain.ErrInvalidRefreshToken
}

func (s *AuthService) withinGrace(stored *domain.RefreshToken) bool {
	return s.refreshGrace > 0 && stored.RotatedAt != nil && s.now().Sub(*stored.RotatedAt) <= s.refreshGrace
}

// pairFor issues a new token pair for the owner of stored.
func (s *AuthService) pairFor(ctx context.Context, stored *domain.RefreshToken) (*TokenPair, error) {
	user, err := s.users.GetByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrInvalidRefreshToken
		}
		return nil, err
	}
	pair, err := s.issuePair(ctx, user)
	if err != nil {
		return nil, err
	}
	return &pair, nil
}

// Logout revokes the presented refresh token. Unknown tokens are ignored so
// logout is idempotent.
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	stored, err := s.tokens.GetByHash(ctx, hashToken(rawToken))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidRefreshToken) {
			return nil
		}
		return err
	}
	_, err = s.tokens.Revoke(ctx, stored.ID)
	return err
}

func (s *AuthService) issuePair(ctx context.Context, user *domain.User) (TokenPair, error) {
	access, accessExp, err := s.issuer.Issue(user)
	if err != nil {
		return TokenPair{}, fmt.Errorf("issue access token: %w", err)
	}
	raw, err := newOpaqueToken()
	if err != nil {
		return TokenPair{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate token id: %w", err)
	}
	now := s.now()
	rt := &domain.RefreshToken{
		ID:        id,
		UserID:    user.ID,
		TokenHash: hashToken(raw),
		ExpiresAt: now.Add(s.refreshTTL),
		CreatedAt: now,
	}
	if err := s.tokens.Create(ctx, rt); err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     raw,
		RefreshExpiresAt: rt.ExpiresAt,
	}, nil
}

func newOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken is SHA-256 rather than bcrypt: refresh tokens are 256-bit random
// values, so a fast hash is sufficient and keeps lookup by hash possible.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
