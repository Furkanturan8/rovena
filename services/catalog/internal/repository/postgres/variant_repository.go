package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/Furkanturan8/rovena/services/catalog/internal/repository"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type VariantRepository struct {
	db *bun.DB
}

func NewVariantRepository(db *bun.DB) *VariantRepository { return &VariantRepository{db: db} }

var _ repository.VariantRepository = (*VariantRepository)(nil)

func (r *VariantRepository) Create(ctx context.Context, v *domain.Variant) error {
	_, err := r.db.NewInsert().Model(variantRowFrom(v)).Exec(ctx)
	if err != nil {
		return mapUnique(err)
	}
	return nil
}

func (r *VariantRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Variant, error) {
	row := new(variantRow)
	err := r.db.NewSelect().Model(row).Where("v.id = ?", id).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrVariantNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *VariantRepository) Update(ctx context.Context, v *domain.Variant) error {
	v.UpdatedAt = time.Now()
	res, err := r.db.NewUpdate().Model((*variantRow)(nil)).
		Set("size = ?", v.Size).
		Set("color = ?", v.Color).
		Set("price_cents = ?", v.PriceCents).
		Set("active = ?", v.Active).
		Set("updated_at = ?", v.UpdatedAt).
		Where("id = ?", v.ID).
		Exec(ctx)
	if err != nil {
		return mapUnique(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrVariantNotFound
	}
	return nil
}

func (r *VariantRepository) CountActive(ctx context.Context, productID uuid.UUID) (int, error) {
	n, err := r.db.NewSelect().Model((*variantRow)(nil)).
		Where("v.product_id = ?", productID).Where("v.active").Count(ctx)
	return int(n), err
}
