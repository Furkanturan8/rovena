package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MaxRefundRequests bounds how often the recovery job asks Payment to refund the same order.
const MaxRefundRequests = 5

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
	// StatusConfirmed: paid and the stock is committed; the shop can now pack the order.
	StatusConfirmed Status = "confirmed"
	// StatusShipped and StatusDelivered are set by an operator after the saga is done.
	StatusShipped   Status = "shipped"
	StatusDelivered Status = "delivered"
	// StatusCancelled is final, like delivered.
	StatusCancelled Status = "cancelled"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusAwaitingPayment, StatusPaid, StatusConfirmed, StatusShipped, StatusDelivered, StatusCancelled:
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
	// CodeCancelledByCustomer is the only cancellation that leaves the cart empty: the customer
	// chose it. After every other cancellation the items go back into the cart.
	CodeCancelledByCustomer = "cancelled_by_customer"
)

// RefundState tracks whether money taken for a cancelled order was given back.
type RefundState string

const (
	RefundNone      RefundState = "none"
	RefundRequested RefundState = "requested"
	RefundRefunded  RefundState = "refunded"
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

// ShippingAddress is the delivery address copied into the order at checkout.
type ShippingAddress struct {
	FullName   string `json:"full_name"`
	Phone      string `json:"phone"`
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	District   string `json:"district"`
	City       string `json:"city"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

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
	RefundState      RefundState
	RefundRequests   int
	FailureCode      string
	FailureMessage   string
	Items            []OrderItem
	ShippingAddress  ShippingAddress
	Carrier          string
	TrackingCode     string
	ShippedAt        *time.Time
	DeliveredAt      *time.Time
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
	ErrOrderNotFound = errors.New("order not found")
	// ErrCannotShip / ErrCannotDeliver: the order is not in the status that step needs.
	ErrCannotShip           = errors.New("only a confirmed order can be shipped")
	ErrCannotDeliver        = errors.New("only a shipped order can be marked delivered")
	ErrCartEmpty            = errors.New("cart is empty")
	ErrCartChanged          = errors.New("cart changed during checkout")
	ErrCartFull             = errors.New("cart has too many different items")
	ErrInvalidQuantity      = errors.New("quantity out of range")
	ErrIdempotencyKeyReused = errors.New("idempotency key was used with a different request")
	ErrInvalidRequest       = errors.New("invalid request")
	// ErrCannotCancel means the order is already paid for, confirmed or on its way: the customer
	// can no longer cancel it by themselves.
	ErrCannotCancel = errors.New("order can no longer be cancelled")
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
