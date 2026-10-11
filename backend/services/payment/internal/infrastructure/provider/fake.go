// Package provider contains payment provider implementations.
package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
)

// Test payment methods. The token decides what the fake provider answers, so every
// outcome of the checkout flow can be triggered on purpose.
const (
	MethodApproved          = "tok_visa"               // approved
	MethodDeclined          = "tok_declined"           // declined: card_declined
	MethodInsufficientFunds = "tok_insufficient_funds" // declined: insufficient_funds
	MethodExpired           = "tok_expired"            // declined: expired_card
	MethodProviderError     = "tok_provider_error"     // never answers: error on every call
	MethodFlaky             = "tok_flaky"              // errors on the first two calls, then approves
	MethodSlow              = "tok_slow"               // takes longer than any sensible timeout
)

// MaxAmountCents is the largest amount the fake provider approves (100,000 TRY).
const MaxAmountCents = 10_000_000

// ErrUnavailable is what a provider outage looks like to the service.
var ErrUnavailable = errors.New("payment provider unavailable")

// Fake is a scripted stand-in for a real payment provider. Like a real one it
// remembers the idempotency keys it has seen: a repeated call returns the first answer
// instead of "charging again". State is in memory, so it resets when the service restarts.
type Fake struct {
	latency time.Duration

	mu      sync.Mutex
	results map[string]domain.ChargeResult // by idempotency key
	calls   map[string]int                 // provider calls per key, for flaky
	refunds map[string]domain.RefundResult // by idempotency key
}

// NewFake creates a fake provider that waits latency before every answer.
func NewFake(latency time.Duration) *Fake {
	return &Fake{
		latency: latency,
		results: make(map[string]domain.ChargeResult),
		calls:   make(map[string]int),
		refunds: make(map[string]domain.RefundResult),
	}
}

func (f *Fake) Charge(ctx context.Context, req domain.ChargeRequest) (domain.ChargeResult, error) {
	if err := sleep(ctx, f.latency); err != nil {
		return domain.ChargeResult{}, err
	}

	if req.PaymentMethod == MethodSlow {
		// Only the caller's timeout ends this call, as with a hung connection.
		return domain.ChargeResult{}, sleep(ctx, time.Hour)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if res, ok := f.results[req.IdempotencyKey]; ok {
		return res, nil
	}
	f.calls[req.IdempotencyKey]++

	switch req.PaymentMethod {
	case MethodProviderError:
		return domain.ChargeResult{}, ErrUnavailable
	case MethodFlaky:
		if f.calls[req.IdempotencyKey] <= 2 {
			return domain.ChargeResult{}, ErrUnavailable
		}
	}

	res := decide(req)
	f.results[req.IdempotencyKey] = res
	return res, nil
}

// Refund gives a charge back. Like a real provider it recognises a repeated idempotency key
// and answers with the first result instead of refunding twice.
func (f *Fake) Refund(ctx context.Context, req domain.RefundRequest) (domain.RefundResult, error) {
	if err := sleep(ctx, f.latency); err != nil {
		return domain.RefundResult{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if res, ok := f.refunds[req.IdempotencyKey]; ok {
		return res, nil
	}
	res := domain.RefundResult{Reference: "refund_" + req.ChargeRef}
	f.refunds[req.IdempotencyKey] = res
	return res, nil
}

func decide(req domain.ChargeRequest) domain.ChargeResult {
	if req.AmountCents > MaxAmountCents {
		return decline(domain.CodeAmountLimit, "The amount is above the allowed limit")
	}
	switch req.PaymentMethod {
	case MethodApproved, MethodFlaky:
		return domain.ChargeResult{Approved: true, Reference: fmt.Sprintf("fake_%s", req.IdempotencyKey)}
	case MethodDeclined:
		return decline(domain.CodeCardDeclined, "The card was declined")
	case MethodInsufficientFunds:
		return decline(domain.CodeInsufficientFunds, "The card has insufficient funds")
	case MethodExpired:
		return decline(domain.CodeExpiredCard, "The card has expired")
	}
	return decline(domain.CodeInvalidMethod, "The payment method is not valid")
}

func decline(code, message string) domain.ChargeResult {
	return domain.ChargeResult{DeclineCode: code, DeclineMessage: message}
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
