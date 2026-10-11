// Package consumer connects RabbitMQ deliveries to the application layer.
package consumer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/application"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/Furkanturan8/rovena/shared/messaging"
)

// Queue is the work queue of the Order service.
const Queue = "order.payment-results"

type PaymentResultHandler struct {
	orders *application.OrderService
}

func NewPaymentResultHandler(orders *application.OrderService) *PaymentResultHandler {
	return &PaymentResultHandler{orders: orders}
}

// Subscription returns the queue setup. A payment result is retried several times
// because the usual reason for an error is Inventory being briefly unreachable. If it
// stays unreachable the message ends in the dead-letter queue, and the recovery job
// finishes the commit or release from the order's own state.
func Subscription(retryDelay time.Duration) messaging.Subscription {
	return messaging.Subscription{
		Queue:      Queue,
		Types:      []string{events.PaymentSucceeded, events.PaymentFailed, events.PaymentRefunded},
		MaxRetries: 5,
		RetryDelay: retryDelay,
	}
}

// Handle is the messaging.Handler. It does not use an Inbox: the service's steps are
// guarded transitions plus idempotent Inventory calls, so a repeated delivery is harmless,
// and an Inbox could not cover the Inventory call anyway.
func (h *PaymentResultHandler) Handle(ctx context.Context, env messaging.Envelope) error {
	var err error
	switch env.Type {
	case events.PaymentSucceeded:
		if env.Version != events.PaymentSucceededVersion {
			return messaging.Permanent(fmt.Errorf("unsupported %s version %d", env.Type, env.Version))
		}
		var p events.PaymentSucceededPayload
		if err := env.DecodePayload(&p); err != nil {
			return messaging.Permanent(err)
		}
		err = h.orders.OnPaymentSucceeded(ctx, p)
	case events.PaymentFailed:
		if env.Version != events.PaymentFailedVersion {
			return messaging.Permanent(fmt.Errorf("unsupported %s version %d", env.Type, env.Version))
		}
		var p events.PaymentFailedPayload
		if err := env.DecodePayload(&p); err != nil {
			return messaging.Permanent(err)
		}
		err = h.orders.OnPaymentFailed(ctx, p)
	case events.PaymentRefunded:
		if env.Version != events.PaymentRefundedVersion {
			return messaging.Permanent(fmt.Errorf("unsupported %s version %d", env.Type, env.Version))
		}
		var p events.PaymentRefundedPayload
		if err := env.DecodePayload(&p); err != nil {
			return messaging.Permanent(err)
		}
		err = h.orders.OnPaymentRefunded(ctx, p)
	default:
		return messaging.Permanent(fmt.Errorf("unexpected event type %q", env.Type))
	}

	if errors.Is(err, application.ErrUnexpectedEvent) {
		return messaging.Permanent(err)
	}
	return err
}
