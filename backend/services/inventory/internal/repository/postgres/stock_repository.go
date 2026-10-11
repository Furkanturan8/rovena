package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/Furkanturan8/rovena/services/inventory/internal/repository"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// availableExpr is what a new order may still take.
const availableExpr = "(s.on_hand - s.reserved)"

type StockRepository struct {
	db *bun.DB
}

func NewStockRepository(db *bun.DB) *StockRepository { return &StockRepository{db: db} }

var _ repository.StockRepository = (*StockRepository)(nil)

func (r *StockRepository) Create(ctx context.Context, item *domain.StockItem, reason string) error {
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(stockItemRowFrom(item)).Exec(ctx); err != nil {
			return err
		}
		if item.OnHand == 0 {
			return nil
		}
		return insertMovement(ctx, tx, item.VariantID, domain.MovementReceipt, item.OnHand, reason, nil)
	})
	if err != nil {
		return mapConstraint(err)
	}
	return nil
}

func (r *StockRepository) GetByVariantID(ctx context.Context, variantID uuid.UUID) (*domain.StockItem, error) {
	row := new(stockItemRow)
	err := r.db.NewSelect().Model(row).Where("s.variant_id = ?", variantID).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrStockItemNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *StockRepository) List(ctx context.Context, f domain.StockFilter) ([]domain.StockItem, int, error) {
	var rows []stockItemRow
	q := r.db.NewSelect().Model(&rows)

	if f.SKU != "" {
		// starts_with instead of LIKE: no user input can act as a wildcard.
		q = q.Where("starts_with(s.sku, ?)", f.SKU)
	}
	if f.Below != nil {
		q = q.Where(availableExpr+" < ?", *f.Below)
	}

	switch f.Sort {
	case domain.SortSKU:
		q = q.OrderExpr("s.sku ASC")
	case domain.SortAvailableAsc:
		q = q.OrderExpr(availableExpr + " ASC, s.sku ASC")
	default:
		q = q.OrderExpr("s.created_at DESC, s.id DESC")
	}

	total, err := q.Limit(int64(f.PageSize)).Offset(int64((f.Page - 1) * f.PageSize)).ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.StockItem, len(rows))
	for i := range rows {
		out[i] = *rows[i].toDomain()
	}
	return out, int(total), nil
}

func (r *StockRepository) Availability(ctx context.Context, variantIDs []uuid.UUID) ([]domain.Availability, error) {
	if len(variantIDs) == 0 {
		return nil, nil
	}
	var rows []availabilityRow
	err := r.db.NewSelect().Model((*stockItemRow)(nil)).
		Column("s.variant_id", "s.sku").
		ColumnExpr(availableExpr+" AS available").
		Where("s.variant_id IN (?)", bun.In(variantIDs)).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Availability, len(rows))
	for i, row := range rows {
		out[i] = domain.Availability{VariantID: row.VariantID, SKU: row.SKU, Available: row.Available}
	}
	return out, nil
}

func (r *StockRepository) ApplyDelta(ctx context.Context, variantID uuid.UUID, delta int, typ domain.MovementType, reason string) (*domain.StockItem, error) {
	row := new(stockItemRow)
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Conditional update: a negative delta is rejected rather than applied when
		// it would eat into stock that is already reserved for an order.
		res, err := tx.NewUpdate().Model(row).
			Set("on_hand = on_hand + ?", delta).
			Set("updated_at = ?", time.Now()).
			Where("variant_id = ?", variantID).
			Where("on_hand + ? >= reserved", delta).
			Returning("*").
			Exec(ctx)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			exists, err := tx.NewSelect().Model((*stockItemRow)(nil)).Where("s.variant_id = ?", variantID).Exists(ctx)
			if err != nil {
				return err
			}
			if !exists {
				return domain.ErrStockItemNotFound
			}
			return domain.ErrReservedExceedsOnHand
		}
		return insertMovement(ctx, tx, variantID, typ, delta, reason, nil)
	})
	if err != nil {
		return nil, mapConstraint(err)
	}
	return row.toDomain(), nil
}

func (r *StockRepository) ListMovements(ctx context.Context, f domain.MovementFilter) ([]domain.Movement, int, error) {
	var rows []movementRow
	total, err := r.db.NewSelect().Model(&rows).
		Where("m.variant_id = ?", f.VariantID).
		OrderExpr("m.created_at DESC, m.id DESC").
		Limit(int64(f.PageSize)).Offset(int64((f.Page - 1) * f.PageSize)).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.Movement, len(rows))
	for i := range rows {
		out[i] = *rows[i].toDomain()
	}
	return out, int(total), nil
}

func insertMovement(ctx context.Context, tx bun.Tx, variantID uuid.UUID, typ domain.MovementType, delta int, reason string, reservationID *uuid.UUID) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	_, err = tx.NewInsert().Model(&movementRow{
		ID: id, VariantID: variantID, Type: string(typ), QuantityDelta: delta,
		Reason: reason, ReservationID: reservationID, CreatedAt: time.Now(),
	}).Exec(ctx)
	return err
}
