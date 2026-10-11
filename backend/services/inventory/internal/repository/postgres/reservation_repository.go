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

type ReservationRepository struct {
	db *bun.DB
}

func NewReservationRepository(db *bun.DB) *ReservationRepository {
	return &ReservationRepository{db: db}
}

var _ repository.ReservationRepository = (*ReservationRepository)(nil)

func (r *ReservationRepository) Reserve(ctx context.Context, res *domain.Reservation) (*domain.Reservation, bool, error) {
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Inserted first: a redelivered request for the same order fails here on the
		// unique order_id and never touches stock.
		if _, err := tx.NewInsert().Model(reservationRowFrom(res)).Exec(ctx); err != nil {
			return err
		}
		now := time.Now()
		for _, it := range res.Items {
			if err := hold(ctx, tx, it, now); err != nil {
				return err
			}
		}
		rows := make([]reservationItemRow, len(res.Items))
		for i, it := range res.Items {
			rows[i] = reservationItemRow{ReservationID: res.ID, VariantID: it.VariantID, Quantity: it.Quantity}
		}
		_, err := tx.NewInsert().Model(&rows).Exec(ctx)
		return err
	})
	if code, constraint := pgCode(err); code == uniqueViolation && constraint == "reservations_order_id_key" {
		// The transaction above was rolled back, so nothing was reserved twice.
		existing, getErr := r.GetByOrderID(ctx, res.OrderID)
		if getErr != nil {
			return nil, false, getErr
		}
		return existing, false, nil
	}
	if err != nil {
		return nil, false, mapConstraint(err)
	}
	return res, true, nil
}

// hold raises the reserved quantity of one item. The WHERE clause does the
// checking, so two orders competing for the last unit cannot both win: whoever
// runs second sees the first one's increment and matches no row.
func hold(ctx context.Context, tx bun.Tx, it domain.ReservationItem, now time.Time) error {
	res, err := tx.NewUpdate().Model((*stockItemRow)(nil)).
		Set("reserved = reserved + ?", it.Quantity).
		Set("updated_at = ?", now).
		Where("variant_id = ?", it.VariantID).
		Where("on_hand - reserved >= ?", it.Quantity).
		Exec(ctx)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	// Nothing was updated: either the variant is unknown or there is not enough left.
	row := new(stockItemRow)
	err = tx.NewSelect().Model(row).Where("s.variant_id = ?", it.VariantID).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return &domain.StockError{VariantID: it.VariantID, Requested: it.Quantity, Err: domain.ErrStockItemNotFound}
	}
	if err != nil {
		return err
	}
	return &domain.StockError{
		VariantID: it.VariantID, Requested: it.Quantity,
		Available: row.toDomain().Available(), Err: domain.ErrInsufficientStock,
	}
}

func (r *ReservationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	return r.getOne(ctx, "r.id = ?", id)
}

func (r *ReservationRepository) GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.Reservation, error) {
	return r.getOne(ctx, "r.order_id = ?", orderID)
}

func (r *ReservationRepository) getOne(ctx context.Context, where string, arg any) (*domain.Reservation, error) {
	row := new(reservationRow)
	err := r.db.NewSelect().Model(row).Where(where, arg).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrReservationNotFound
	}
	if err != nil {
		return nil, err
	}
	res := row.toDomain()
	if res.Items, err = loadItems(ctx, r.db, res.ID); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *ReservationRepository) Commit(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	var out *domain.Reservation
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := lockReservation(ctx, tx, id)
		if err != nil {
			return err
		}
		if res.Status == domain.ReservationCommitted {
			out = res
			return nil
		}
		if res.Status != domain.ReservationPending {
			return domain.ErrReservationClosed
		}
		now := time.Now()
		for _, it := range res.Items {
			// The held units leave the warehouse: both counters drop by the same amount.
			if _, err := tx.NewUpdate().Model((*stockItemRow)(nil)).
				Set("on_hand = on_hand - ?", it.Quantity).
				Set("reserved = reserved - ?", it.Quantity).
				Set("updated_at = ?", now).
				Where("variant_id = ?", it.VariantID).
				Exec(ctx); err != nil {
				return err
			}
			if err := insertMovement(ctx, tx, it.VariantID, domain.MovementSale, -it.Quantity, "", &res.ID); err != nil {
				return err
			}
		}
		if err := setStatus(ctx, tx, res, domain.ReservationCommitted, now); err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, mapConstraint(err)
	}
	return out, nil
}

func (r *ReservationRepository) Release(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	res, _, err := r.release(ctx, id, domain.ReservationReleased)
	return res, err
}

// release gives the held units back and reports whether it actually changed anything,
// so a second release (or a competing expirer) is a no-op instead of double counting.
func (r *ReservationRepository) release(ctx context.Context, id uuid.UUID, status domain.ReservationStatus) (*domain.Reservation, bool, error) {
	var (
		out     *domain.Reservation
		changed bool
	)
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := lockReservation(ctx, tx, id)
		if err != nil {
			return err
		}
		if res.Status == domain.ReservationCommitted {
			return domain.ErrReservationCommitted
		}
		if res.Status != domain.ReservationPending {
			out = res
			return nil
		}
		now := time.Now()
		for _, it := range res.Items {
			if _, err := tx.NewUpdate().Model((*stockItemRow)(nil)).
				Set("reserved = reserved - ?", it.Quantity).
				Set("updated_at = ?", now).
				Where("variant_id = ?", it.VariantID).
				Exec(ctx); err != nil {
				return err
			}
		}
		if err := setStatus(ctx, tx, res, status, now); err != nil {
			return err
		}
		out, changed = res, true
		return nil
	})
	if err != nil {
		return nil, false, mapConstraint(err)
	}
	return out, changed, nil
}

func (r *ReservationRepository) ExpireDue(ctx context.Context, now time.Time, limit int) (int, error) {
	var ids []uuid.UUID
	err := r.db.NewSelect().Model((*reservationRow)(nil)).
		Column("r.id").
		Where("r.status = ?", string(domain.ReservationPending)).
		Where("r.expires_at <= ?", now).
		OrderExpr("r.expires_at").
		Limit(int64(limit)).
		Scan(ctx, &ids)
	if err != nil {
		return 0, err
	}

	released := 0
	for _, id := range ids {
		_, changed, err := r.release(ctx, id, domain.ReservationExpired)
		switch {
		case err == nil:
			if changed {
				released++
			}
		case errors.Is(err, domain.ErrReservationCommitted), errors.Is(err, domain.ErrReservationNotFound):
			// The order service got there first between the select and the lock.
		default:
			return released, err
		}
	}
	return released, nil
}

// lockReservation locks the reservation row for the rest of the transaction, so
// commit, release and expiry can never run against the same reservation at once.
func lockReservation(ctx context.Context, tx bun.Tx, id uuid.UUID) (*domain.Reservation, error) {
	row := new(reservationRow)
	err := tx.NewSelect().Model(row).Where("r.id = ?", id).For("UPDATE").Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrReservationNotFound
	}
	if err != nil {
		return nil, err
	}
	res := row.toDomain()
	if res.Items, err = loadItems(ctx, tx, res.ID); err != nil {
		return nil, err
	}
	return res, nil
}

// loadItems returns the items ordered by variant, which is also the order every
// transaction locks stock rows in. A fixed order rules out deadlocks between two
// reservations that share variants.
func loadItems(ctx context.Context, db bun.IDB, reservationID uuid.UUID) ([]domain.ReservationItem, error) {
	var rows []reservationItemRow
	err := db.NewSelect().Model(&rows).
		Where("ri.reservation_id = ?", reservationID).
		OrderExpr("ri.variant_id").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.ReservationItem, len(rows))
	for i, row := range rows {
		items[i] = domain.ReservationItem{VariantID: row.VariantID, Quantity: row.Quantity}
	}
	return items, nil
}

func setStatus(ctx context.Context, tx bun.Tx, res *domain.Reservation, status domain.ReservationStatus, now time.Time) error {
	_, err := tx.NewUpdate().Model((*reservationRow)(nil)).
		Set("status = ?", string(status)).
		Set("updated_at = ?", now).
		Where("id = ?", res.ID).
		Exec(ctx)
	if err != nil {
		return err
	}
	res.Status, res.UpdatedAt = status, now
	return nil
}
