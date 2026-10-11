package repository

import (
	"context"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/google/uuid"
)

// OrderRepository stores carts and orders. Every state change is a guarded transition:
// it applies only if the order is still in one of the expected states and reports
// changed=false otherwise. That makes every step of the saga safe to repeat, and safe
// to race against the recovery job.
//
// A transition that carries an envelope writes it to the outbox in the same transaction.
type OrderRepository interface {
	GetCart(ctx context.Context, customerID uuid.UUID) ([]domain.CartItem, error)
	// SetCartItem inserts the line or replaces its quantity. Returns ErrCartFull when
	// a new line would exceed the limit.
	SetCartItem(ctx context.Context, customerID uuid.UUID, item domain.CartItem) error
	RemoveCartItem(ctx context.Context, customerID, variantID uuid.UUID) error
	ClearCart(ctx context.Context, customerID uuid.UUID) error

	// Create stores the order with its lines and empties the cart in one transaction,
	// provided the cart still holds exactly the lines the order was priced from
	// (ErrCartChanged otherwise). Returns ErrDuplicateCheckout for a repeated key.
	Create(ctx context.Context, o *domain.Order, cart []domain.CartItem) error

	GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	GetByIdempotencyKey(ctx context.Context, customerID uuid.UUID, key string) (*domain.Order, error)
	History(ctx context.Context, id uuid.UUID) ([]domain.StatusChange, error)
	List(ctx context.Context, f domain.Filter) ([]domain.Order, int, error)

	// MarkReserved: pending -> awaiting_payment. Records the reservation and queues PaymentRequested.
	MarkReserved(ctx context.Context, id, reservationID uuid.UUID, paymentRequested messaging.Envelope) (changed bool, err error)
	// MarkPaid: awaiting_payment -> paid.
	MarkPaid(ctx context.Context, id uuid.UUID) (changed bool, err error)
	// Confirm: paid -> confirmed, reservation held -> committed, and queues OrderConfirmed.
	Confirm(ctx context.Context, id uuid.UUID, confirmed messaging.Envelope) (changed bool, err error)
	// Cancel moves the order from any status in from to cancelled and queues OrderCancelled.
	Cancel(ctx context.Context, id uuid.UUID, from []domain.Status, code, message string, cancelled messaging.Envelope) (changed bool, err error)
	// MarkReleased: a cancelled order's held reservation -> released.
	MarkReleased(ctx context.Context, id uuid.UUID) (changed bool, err error)
	// RequestPaymentAgain queues another PaymentRequested for an order still awaiting payment.
	RequestPaymentAgain(ctx context.Context, id uuid.UUID, paymentRequested messaging.Envelope) (changed bool, err error)
	// RequestRefund marks a cancelled order's money as to be returned and queues RefundRequested.
	// It applies only while refund_state is none.
	RequestRefund(ctx context.Context, id uuid.UUID, refundRequested messaging.Envelope) (changed bool, err error)
	// RequestRefundAgain queues another RefundRequested while the refund is still outstanding.
	RequestRefundAgain(ctx context.Context, id uuid.UUID, refundRequested messaging.Envelope) (changed bool, err error)
	// MarkRefunded: refund requested -> refunded.
	MarkRefunded(ctx context.Context, id uuid.UUID) (changed bool, err error)
	// RestoreCart puts the items of a cancelled order back into the customer's cart. Lines the
	// customer already has again are left as they are, and the cart limit is respected.
	RestoreCart(ctx context.Context, customerID uuid.UUID, items []domain.CartItem) error
	// Touch moves updated_at forward, so a recovery attempt that changed nothing is not repeated at once.
	Touch(ctx context.Context, id uuid.UUID) error

	// Unfinished returns orders that still owe a step and have not changed since olderThan.
	Unfinished(ctx context.Context, olderThan time.Time, limit int) ([]domain.Order, error)
}
