// Package consumer connects RabbitMQ deliveries to the application layer.
package consumer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/application"
	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/Furkanturan8/rovena/shared/messaging"
)

// Queue is the work queue of the Payment service.
const Queue = "payment.requests"

type PaymentRequestedHandler struct {
	payments *application.PaymentService
}

func NewPaymentRequestedHandler(payments *application.PaymentService) *PaymentRequestedHandler {
	return &PaymentRequestedHandler{payments: payments}
}

// Subscription returns the queue setup. The consumer gets a few retries more than the
// service's provider attempts: those extra deliveries absorb short database outages
// without eating into the attempts the provider is allowed.
func Subscription(providerAttempts int, retryDelay time.Duration) messaging.Subscription {
	return messaging.Subscription{
		Queue:      Queue,
		Types:      []string{events.PaymentRequested},
		MaxRetries: providerAttempts + 2,
		RetryDelay: retryDelay,
	}
}

// Handle is the messaging.Handler. Idempotency is the service's job (it keys on the
// payment's idempotency key), so the handler does not use an Inbox: the effect it guards
// is a call to an external provider, which a database transaction cannot cover.
func (h *PaymentRequestedHandler) Handle(ctx context.Context, env messaging.Envelope) error {
	if env.Version != events.PaymentRequestedVersion {
		return messaging.Permanent(fmt.Errorf("unsupported %s version %d", env.Type, env.Version))
	}
	var p events.PaymentRequestedPayload
	if err := env.DecodePayload(&p); err != nil {
		return messaging.Permanent(err)
	}

	err := h.payments.Process(ctx, application.ProcessInput{
		OrderID: p.OrderID, CustomerID: p.CustomerID, IdempotencyKey: p.IdempotencyKey,
		AmountCents: p.AmountCents, Currency: p.Currency, PaymentMethod: p.PaymentMethod,
	})
	// These will fail the same way every time; waiting and retrying cannot help.
	if errors.Is(err, domain.ErrInvalidRequest) ||
		errors.Is(err, domain.ErrIdempotencyKeyReused) ||
		errors.Is(err, domain.ErrOrderAlreadyPaid) {
		return messaging.Permanent(err)
	}
	return err
}
