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
	if code, _ := pgCode(err); code == foreignKeyViolation {
		return domain.ErrProductNotFound
	}
	return mapUnique(err)
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
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		status, err := lockProduct(ctx, tx, v.ProductID)
		if err != nil {
			return err
		}
		if !v.Active && domain.ProductStatus(status) == domain.StatusActive {
			others, err := tx.NewSelect().Model((*variantRow)(nil)).
				Where("v.product_id = ?", v.ProductID).
				Where("v.active").
				Where("v.id <> ?", v.ID).
				Count(ctx)
			if err != nil {
				return err
			}
			if others == 0 {
				return domain.ErrNoActiveVariant
			}
		}
		res, err := tx.NewUpdate().Model((*variantRow)(nil)).
			Set("size = ?", v.Size).
			Set("color = ?", v.Color).
			Set("price_cents = ?", v.PriceCents).
			Set("active = ?", v.Active).
			Set("updated_at = ?", v.UpdatedAt).
			Where("id = ?", v.ID).
			Exec(ctx)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return domain.ErrVariantNotFound
		}
		return nil
	})
	return mapUnique(err)
}

// lockProduct takes a row lock on the product and returns its status. Holding it
// serializes every change that could break "an active product has an active variant".
func lockProduct(ctx context.Context, tx bun.Tx, id uuid.UUID) (string, error) {
	var status string
	err := tx.NewSelect().Model((*productRow)(nil)).
		ColumnExpr("p.status").
		Where("p.id = ?", id).
		For("UPDATE").
		Scan(ctx, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrProductNotFound
	}
	return status, err
}
