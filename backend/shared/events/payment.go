// Package events holds the contracts of the events that cross service boundaries:
// type names, versions and payload shapes. It contains no behaviour, so the
// publisher and every consumer of an event read the same definition.
package events

import "github.com/google/uuid"

const (
	// PaymentRequested is a command from Order to Payment: charge this order.
	PaymentRequested = "PaymentRequested"
	// PaymentSucceeded and PaymentFailed are facts published by Payment. Exactly one of
	// them is published per payment, but it may be published again (see PaymentService).
	PaymentSucceeded = "PaymentSucceeded"
	PaymentFailed    = "PaymentFailed"
)

const (
	PaymentRequestedVersion = 1
	PaymentSucceededVersion = 1
	PaymentFailedVersion    = 1
)

// PaymentRequestedPayload asks Payment to charge an order.
//
// IdempotencyKey identifies the customer's intent to pay, not the message: Order sends
// the same key again if it retries the request, and Payment then charges only once.
// Amounts are in minor units (kuruş), as everywhere in the system.
type PaymentRequestedPayload struct {
	OrderID        uuid.UUID `json:"order_id"`
	CustomerID     uuid.UUID `json:"customer_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	AmountCents    int64     `json:"amount_cents"`
	Currency       string    `json:"currency"`
	// PaymentMethod is a provider token for the card, never a card number.
	PaymentMethod string `json:"payment_method"`
}

type PaymentSucceededPayload struct {
	PaymentID   uuid.UUID `json:"payment_id"`
	OrderID     uuid.UUID `json:"order_id"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	ProviderRef string    `json:"provider_ref"`
}

type PaymentFailedPayload struct {
	PaymentID   uuid.UUID `json:"payment_id"`
	OrderID     uuid.UUID `json:"order_id"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	// FailureCode is a stable, machine-readable reason (card_declined, ...).
	FailureCode string `json:"failure_code"`
	// FailureMessage is safe to show to the customer.
	FailureMessage string `json:"failure_message"`
}

const (
	// RefundRequested is a command from Order to Payment: give the money for this order back.
	RefundRequested = "RefundRequested"
	// PaymentRefunded is the fact that the money was given back.
	PaymentRefunded = "PaymentRefunded"
)

const (
	RefundRequestedVersion = 1
	PaymentRefundedVersion = 1
)

// RefundRequestedPayload asks Payment to refund the succeeded payment of an order. Asking again
// for the same order is safe: a payment is refunded once and the stored result is published again.
type RefundRequestedPayload struct {
	OrderID    uuid.UUID `json:"order_id"`
	CustomerID uuid.UUID `json:"customer_id"`
	// Reason is for people reading logs and the admin view (payment_timeout, reservation_lost, ...).
	Reason string `json:"reason"`
}

type PaymentRefundedPayload struct {
	PaymentID   uuid.UUID `json:"payment_id"`
	OrderID     uuid.UUID `json:"order_id"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	RefundRef   string    `json:"refund_ref"`
}
