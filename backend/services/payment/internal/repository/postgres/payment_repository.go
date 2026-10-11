package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/services/payment/internal/repository"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/Furkanturan8/rovena/shared/outbox"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type PaymentRepository struct {
	db *bun.DB
}

func NewPaymentRepository(db *bun.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

var _ repository.PaymentRepository = (*PaymentRepository)(nil)

func (r *PaymentRepository) Begin(ctx context.Context, p *domain.Payment) (*domain.Payment, bool, error) {
	// DO NOTHING instead of catching the unique violation: concurrent requests with the
	// same key all get through this statement without an error, and exactly one inserts.
	res, err := r.db.NewInsert().Model(rowFrom(p)).
		On("CONFLICT ON CONSTRAINT payments_idempotency_key_key DO NOTHING").
		Exec(ctx)
	if err != nil {
		return nil, false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, false, err
	}
	if n == 1 {
		return p, true, nil
	}
	row := new(paymentRow)
	if err := r.db.NewSelect().Model(row).Where("idempotency_key = ?", p.IdempotencyKey).Scan(ctx); err != nil {
		return nil, false, err
	}
	return row.toDomain(), false, nil
}

func (r *PaymentRepository) StartAttempt(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	row := new(paymentRow)
	err := r.db.NewUpdate().Model(row).
		Set("attempts = attempts + 1").
		Set("updated_at = now()").
		Where("id = ?", id).
		Where("status = ?", string(domain.StatusPending)).
		Returning("*").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return r.GetByID(ctx, id) // already finished (or gone)
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *PaymentRepository) Complete(ctx context.Context, id uuid.UUID, o domain.Outcome, event messaging.Envelope) (*domain.Payment, bool, error) {
	var current *domain.Payment
	applied := false
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		row := new(paymentRow)
		err := tx.NewUpdate().Model(row).
			Set("status = ?", string(o.Status)).
			Set("provider_ref = ?", o.ProviderRef).
			Set("failure_code = ?", o.FailureCode).
			Set("failure_message = ?", o.FailureMessage).
			Set("completed_at = now()").
			Set("updated_at = now()").
			Where("id = ?", id).
			Where("status = ?", string(domain.StatusPending)).
			Returning("*").
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil // someone else completed it; they stored their event
		}
		if err != nil {
			return err
		}
		// Same transaction as the status change: the payment is finished if and only
		// if its event is queued.
		if err := outbox.Add(ctx, tx, event); err != nil {
			return err
		}
		current, applied = row.toDomain(), true
		return nil
	})
	if err != nil {
		return nil, false, mapConstraint(err)
	}
	if !applied {
		current, err = r.GetByID(ctx, id)
		if err != nil {
			return nil, false, err
		}
	}
	return current, applied, nil
}

func (r *PaymentRepository) PaidByOrder(ctx context.Context, orderID uuid.UUID) (*domain.Payment, error) {
	row := new(paymentRow)
	err := r.db.NewSelect().Model(row).
		Where("order_id = ?", orderID).
		Where("status IN (?)", bun.List([]string{string(domain.StatusSucceeded), string(domain.StatusRefunded)})).
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *PaymentRepository) Refund(ctx context.Context, id uuid.UUID, refundRef string, event messaging.Envelope) (*domain.Payment, bool, error) {
	var current *domain.Payment
	applied := false
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		row := new(paymentRow)
		err := tx.NewUpdate().Model(row).
			Set("status = ?", string(domain.StatusRefunded)).
			Set("refund_ref = ?", refundRef).
			Set("refunded_at = now()").
			Set("updated_at = now()").
			Where("id = ?", id).
			Where("status = ?", string(domain.StatusSucceeded)).
			Returning("*").
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil // refunded by someone else, who stored the event
		}
		if err != nil {
			return err
		}
		if err := outbox.Add(ctx, tx, event); err != nil {
			return err
		}
		current, applied = row.toDomain(), true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if !applied {
		var err error
		if current, err = r.GetByID(ctx, id); err != nil {
			return nil, false, err
		}
	}
	return current, applied, nil
}

func (r *PaymentRepository) Enqueue(ctx context.Context, event messaging.Envelope) error {
	return outbox.Add(ctx, r.db, event)
}

func (r *PaymentRepository) HasSucceeded(ctx context.Context, orderID uuid.UUID) (bool, error) {
	return r.db.NewSelect().Model((*paymentRow)(nil)).
		Where("order_id = ?", orderID).
		Where("status IN (?)", bun.List([]string{string(domain.StatusSucceeded), string(domain.StatusRefunded)})).
		Exists(ctx)
}

func (r *PaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	row := new(paymentRow)
	err := r.db.NewSelect().Model(row).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *PaymentRepository) LatestByOrder(ctx context.Context, orderID uuid.UUID) (*domain.Payment, error) {
	row := new(paymentRow)
	err := r.db.NewSelect().Model(row).
		Where("order_id = ?", orderID).
		OrderExpr("created_at DESC, id DESC").
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *PaymentRepository) List(ctx context.Context, f domain.Filter) ([]domain.Payment, int, error) {
	var rows []paymentRow
	q := r.db.NewSelect().Model(&rows)
	if f.Status != "" {
		q = q.Where("status = ?", string(f.Status))
	}
	if f.OrderID != nil {
		q = q.Where("order_id = ?", *f.OrderID)
	}
	total, err := q.
		OrderExpr("created_at DESC, id DESC").
		Limit(int64(f.PageSize)).
		Offset(int64((f.Page - 1) * f.PageSize)).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.Payment, len(rows))
	for i := range rows {
		out[i] = *rows[i].toDomain()
	}
	return out, int(total), nil
}
