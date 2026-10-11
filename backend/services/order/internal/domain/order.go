package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	MaxLineQuantity = 99
	MaxCartLines    = 50
)

type Status string

const (
	// StatusPending: the order exists but no stock is held yet.
	StatusPending Status = "pending"
	// StatusAwaitingPayment: stock is held and PaymentRequested has been queued.
	StatusAwaitingPayment Status = "awaiting_payment"
	// StatusPaid: the payment succeeded; committing the stock is still owed to Inventory.
	StatusPaid Status = "paid"
	// StatusConfirmed and StatusCancelled are final.
	StatusConfirmed Status = "confirmed"
	StatusCancelled Status = "cancelled"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusAwaitingPayment, StatusPaid, StatusConfirmed, StatusCancelled:
		return true
	}
	return false
}

// ReservationState is the order's view of its stock reservation in Inventory.
type ReservationState string

const (
	ReservationNone      ReservationState = "none"
	ReservationHeld      ReservationState = "held"
	ReservationCommitted ReservationState = "committed"
	ReservationReleased  ReservationState = "released"
)

// Failure codes of cancellations decided by Order itself. A payment failure keeps the
// code Payment gave (card_declined, ...).
const (
	CodeOutOfStock       = "out_of_stock"
	CodeItemUnavailable  = "item_unavailable"
	CodeCheckoutTimeout  = "checkout_timeout"
	CodePaymentTimeout   = "payment_timeout"
	CodeReservationLost  = "reservation_lost"
	CodePaymentCancelled = "payment_failed"
)

type CartItem struct {
	VariantID uuid.UUID
	Quantity  int
}

type OrderItem struct {
	VariantID      uuid.UUID
	ProductID      uuid.UUID
	SKU            string
	ProductName    string
	Size           string
	Color          string
	UnitPriceCents int64
	Quantity       int
}

func (i OrderItem) LineTotalCents() int64 { return i.UnitPriceCents * int64(i.Quantity) }

type Order struct {
	ID               uuid.UUID
	CustomerID       uuid.UUID
	Status           Status
	TotalCents       int64
	Currency         string
	IdempotencyKey   string
	PaymentMethod    string
	ReservationID    *uuid.UUID
	ReservationState ReservationState
	PaymentRequests  int
	FailureCode      string
	FailureMessage   string
	Items            []OrderItem
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// PaymentIdempotencyKey is derived from the order, so every PaymentRequested for the
// same order carries the same key and Payment charges at most once.
func (o *Order) PaymentIdempotencyKey() string { return "order:" + o.ID.String() }

type StatusChange struct {
	FromStatus *Status
	ToStatus   Status
	Reason     string
	CreatedAt  time.Time
}

type Filter struct {
	CustomerID *uuid.UUID
	Status     Status
	Page       int
	PageSize   int
}

var (
	ErrOrderNotFound        = errors.New("order not found")
	ErrCartEmpty            = errors.New("cart is empty")
	ErrCartChanged          = errors.New("cart changed during checkout")
	ErrCartFull             = errors.New("cart has too many different items")
	ErrInvalidQuantity      = errors.New("quantity out of range")
	ErrIdempotencyKeyReused = errors.New("idempotency key was used with a different request")
	ErrInvalidRequest       = errors.New("invalid request")
	// ErrDuplicateCheckout is returned by the repository when the (customer, key) pair
	// already has an order; the service then returns that order.
	ErrDuplicateCheckout = errors.New("checkout already exists for this idempotency key")
	ErrMixedCurrencies   = errors.New("cart mixes currencies")
	// ErrDependencyUnavailable means Catalog or Inventory could not answer. Nothing is
	// known to be wrong with the request; trying again later may work.
	ErrDependencyUnavailable = errors.New("dependency unavailable")
)

// ItemError names the cart line that stopped a request, so the customer can fix it.
type ItemError struct {
	VariantID uuid.UUID
	Err       error
}

var (
	ErrItemUnavailable = errors.New("item is no longer available")
	ErrOutOfStock      = errors.New("not enough stock")
)

func (e *ItemError) Error() string { return fmt.Sprintf("%v: variant %s", e.Err, e.VariantID) }
func (e *ItemError) Unwrap() error { return e.Err }
