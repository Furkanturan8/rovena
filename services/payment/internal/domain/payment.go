package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	// StatusPending means the provider has not given a final answer yet.
	StatusPending Status = "pending"
	// StatusSucceeded means the money was taken.
	StatusSucceeded Status = "succeeded"
	// StatusFailed means no money was taken and none will be for this attempt.
	StatusFailed Status = "failed"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusSucceeded, StatusFailed:
		return true
	}
	return false
}

// Failure codes are part of the event contract: Order decides what to tell the
// customer from them, so they only ever get added, never renamed.
const (
	CodeCardDeclined        = "card_declined"
	CodeInsufficientFunds   = "insufficient_funds"
	CodeExpiredCard         = "expired_card"
	CodeInvalidMethod       = "invalid_payment_method"
	CodeAmountLimit         = "amount_limit_exceeded"
	CodeProviderUnavailable = "provider_unavailable"
	CodeOrderAlreadyPaid    = "order_already_paid"
)

// DefaultCurrency is the only currency supported for now, as in Catalog.
const DefaultCurrency = "TRY"

// Payment is one attempt to charge an order. Amounts are in minor units (kuruş).
type Payment struct {
	ID             uuid.UUID
	OrderID        uuid.UUID
	CustomerID     uuid.UUID
	IdempotencyKey string
	AmountCents    int64
	Currency       string
	PaymentMethod  string
	Status         Status
	Attempts       int
	ProviderRef    string
	FailureCode    string
	FailureMessage string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

// SameRequest reports whether p was created for the same charge as other. A reused
// idempotency key with different details is a client bug, and must not be answered
// with the result of the first request.
func (p *Payment) SameRequest(other *Payment) bool {
	return p.OrderID == other.OrderID &&
		p.CustomerID == other.CustomerID &&
		p.AmountCents == other.AmountCents &&
		p.Currency == other.Currency
}

// Outcome is the final answer for a payment.
type Outcome struct {
	Status         Status
	ProviderRef    string
	FailureCode    string
	FailureMessage string
}

func Succeeded(providerRef string) Outcome {
	return Outcome{Status: StatusSucceeded, ProviderRef: providerRef}
}

func Failed(code, message string) Outcome {
	return Outcome{Status: StatusFailed, FailureCode: code, FailureMessage: message}
}

// ChargeRequest is what the payment provider needs. IdempotencyKey lets the provider
// recognise a repeated call and answer it without charging again.
type ChargeRequest struct {
	IdempotencyKey string
	AmountCents    int64
	Currency       string
	PaymentMethod  string
}

// ChargeResult is a definite answer from the provider: the card was charged or it was
// declined. When the provider cannot answer (timeout, outage) Charge returns an error
// instead, and the outcome stays unknown.
type ChargeResult struct {
	Approved       bool
	Reference      string
	DeclineCode    string
	DeclineMessage string
}

type Filter struct {
	Status   Status // empty: all
	OrderID  *uuid.UUID
	Page     int
	PageSize int
}

var (
	ErrPaymentNotFound = errors.New("payment not found")
	// ErrIdempotencyKeyReused means the key was used before for a different charge.
	ErrIdempotencyKeyReused = errors.New("idempotency key was used for a different payment")
	// ErrOrderAlreadyPaid is raised by the database when a second payment for the same
	// order tries to succeed.
	ErrOrderAlreadyPaid = errors.New("order is already paid")
	ErrInvalidRequest   = errors.New("invalid payment request")
)
