package repository

import (
	"context"

	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/google/uuid"
)

type PaymentRepository interface {
	// Begin stores p as a new pending payment unless one with the same idempotency key
	// exists, in which case that one is returned with created=false. Concurrent calls
	// with the same key all get the same row.
	Begin(ctx context.Context, p *domain.Payment) (stored *domain.Payment, created bool, err error)
	// StartAttempt counts one provider call on a pending payment and returns the row as
	// it is now. A payment that already finished is returned unchanged.
	StartAttempt(ctx context.Context, id uuid.UUID) (*domain.Payment, error)
	// Complete records the outcome and stores event in the outbox, in one transaction.
	// Only a pending payment can complete: if another worker got there first, nothing is
	// written, applied is false and the current row is returned.
	// Returns ErrOrderAlreadyPaid if the order already has a succeeded payment.
	Complete(ctx context.Context, id uuid.UUID, o domain.Outcome, event messaging.Envelope) (current *domain.Payment, applied bool, err error)
	// Enqueue stores event in the outbox on its own.
	Enqueue(ctx context.Context, event messaging.Envelope) error
	// PaidByOrder returns the payment of the order whose money was taken (succeeded, or refunded
	// afterwards), or ErrPaymentNotFound.
	PaidByOrder(ctx context.Context, orderID uuid.UUID) (*domain.Payment, error)
	// Refund moves a succeeded payment to refunded and stores event in the outbox, in one
	// transaction. Only a succeeded payment can be refunded: otherwise nothing is written,
	// applied is false and the current row is returned.
	Refund(ctx context.Context, id uuid.UUID, refundRef string, event messaging.Envelope) (current *domain.Payment, applied bool, err error)
	// HasSucceeded reports whether the order already has a payment whose money was taken.
	HasSucceeded(ctx context.Context, orderID uuid.UUID) (bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error)
	// LatestByOrder returns the newest attempt for the order.
	LatestByOrder(ctx context.Context, orderID uuid.UUID) (*domain.Payment, error)
	List(ctx context.Context, f domain.Filter) ([]domain.Payment, int, error)
}
