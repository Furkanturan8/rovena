package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/Furkanturan8/rovena/services/identity/internal/repository"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type RefreshTokenRepository struct {
	db *bun.DB
}

func NewRefreshTokenRepository(db *bun.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

var _ repository.RefreshTokenRepository = (*RefreshTokenRepository)(nil)

func (r *RefreshTokenRepository) Create(ctx context.Context, t *domain.RefreshToken) error {
	row := &refreshTokenRow{
		ID: t.ID, UserID: t.UserID, TokenHash: t.TokenHash,
		ExpiresAt: t.ExpiresAt, CreatedAt: t.CreatedAt,
	}
	_, err := r.db.NewInsert().Model(row).Exec(ctx)
	return err
}

func (r *RefreshTokenRepository) GetByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	row := new(refreshTokenRow)
	err := r.db.NewSelect().Model(row).Where("token_hash = ?", hash).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrInvalidRefreshToken
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID) (bool, error) {
	res, err := r.db.NewUpdate().Model((*refreshTokenRow)(nil)).
		Set("revoked_at = ?", time.Now()).
		Where("id = ?", id).
		Where("revoked_at IS NULL").
		Exec(ctx)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.NewUpdate().Model((*refreshTokenRow)(nil)).
		Set("revoked_at = ?", time.Now()).
		Where("user_id = ?", userID).
		Where("revoked_at IS NULL").
		Exec(ctx)
	return err
}
