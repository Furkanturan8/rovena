package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// OnPaymentSucceeded handles PaymentSucceeded. It is safe to call any number of times
// for the same payment: every step is a guarded transition and the Inventory calls are
// idempotent. A returned error that is not ErrUnexpectedEvent should be retried.
func (s *OrderService) OnPaymentSucceeded(ctx context.Context, p events.PaymentSucceededPayload) error {
	o, err := s.paymentOrder(ctx, p.OrderID)
	if err != nil {
		return err
	}
	// The amount that was charged must be the amount that was ordered. A mismatch
	// means money moved for something else than this order: stop and let a human look.
	if p.AmountCents != o.TotalCents || p.Currency != o.Currency {
		s.log.ErrorContext(ctx, "PAYMENT AMOUNT MISMATCH: order not confirmed",
			"order_id", o.ID, "ordered_cents", o.TotalCents, "charged_cents", p.AmountCents,
			"ordered_currency", o.Currency, "charged_currency", p.Currency)
		return fmt.Errorf("%w: charged %d %s for an order of %d %s", ErrUnexpectedEvent,
			p.AmountCents, p.Currency, o.TotalCents, o.Currency)
	}

	switch o.Status {
	case domain.StatusAwaitingPayment:
		if _, err := s.repo.MarkPaid(ctx, o.ID); err != nil {
			return err
		}
	case domain.StatusPaid:
		// A redelivery, or a retry after the stock commit failed.
	case domain.StatusConfirmed:
		return nil
	case domain.StatusCancelled:
		s.log.ErrorContext(ctx, "REFUND NEEDED: payment succeeded for a cancelled order",
			"order_id", o.ID, "payment_id", p.PaymentID, "amount_cents", p.AmountCents, "code", o.FailureCode)
		return nil
	default:
		return fmt.Errorf("%w: payment succeeded for order %s in status %s", ErrUnexpectedEvent, o.ID, o.Status)
	}

	return s.settleByID(ctx, o.ID)
}

// OnPaymentFailed handles PaymentFailed: the order is cancelled and its stock released.
func (s *OrderService) OnPaymentFailed(ctx context.Context, p events.PaymentFailedPayload) error {
	o, err := s.paymentOrder(ctx, p.OrderID)
	if err != nil {
		return err
	}
	// Payment refuses a second payment for an order that is already paid. That is
	// about the duplicate request, not about the order.
	if p.FailureCode == paymentOrderAlreadyPaid {
		s.log.InfoContext(ctx, "duplicate payment refused", "order_id", o.ID)
		return nil
	}

	switch o.Status {
	case domain.StatusAwaitingPayment:
		code := p.FailureCode
		if code == "" {
			code = domain.CodePaymentCancelled
		}
		s.cancel(ctx, o, []domain.Status{domain.StatusAwaitingPayment}, code, p.FailureMessage)
	case domain.StatusCancelled:
		// A redelivery, or a retry after the release failed.
	default:
		s.log.WarnContext(ctx, "ignoring PaymentFailed for an order that is not awaiting payment",
			"order_id", o.ID, "status", o.Status, "code", p.FailureCode)
		return nil
	}
	return s.settleByID(ctx, o.ID)
}

func (s *OrderService) paymentOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	o, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, domain.ErrOrderNotFound) {
		return nil, fmt.Errorf("%w: order %s does not exist", ErrUnexpectedEvent, id)
	}
	return o, err
}

func (s *OrderService) settleByID(ctx context.Context, id uuid.UUID) error {
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.settle(ctx, o)
}

// settle does the Inventory side of a decided order: commit the stock of a paid order,
// release the stock of a cancelled one. It returns an error when Inventory could not be
// reached, so the caller retries; the recovery job covers a caller that gives up.
func (s *OrderService) settle(ctx context.Context, o *domain.Order) error {
	if o.ReservationState != domain.ReservationHeld || o.ReservationID == nil {
		return nil
	}
	switch o.Status {
	case domain.StatusPaid:
		err := s.inventory.Commit(ctx, *o.ReservationID)
		if errors.Is(err, ErrReservationGone) {
			// The reservation expired before the payment arrived: the customer paid for
			// stock that may be sold elsewhere by now.
			s.log.ErrorContext(ctx, "REFUND NEEDED: reservation lost before payment arrived",
				"order_id", o.ID, "reservation_id", *o.ReservationID, "amount_cents", o.TotalCents)
			s.cancel(ctx, o, []domain.Status{domain.StatusPaid}, domain.CodeReservationLost,
				"The items could not be kept for you in time")
			_, err := s.repo.MarkReleased(ctx, o.ID)
			return err
		}
		if err != nil {
			return err
		}
		ev, err := s.event(ctx, o, events.OrderConfirmed, events.OrderConfirmedVersion, events.OrderConfirmedPayload{
			OrderID: o.ID, CustomerID: o.CustomerID, TotalCents: o.TotalCents, Currency: o.Currency,
		})
		if err != nil {
			return err
		}
		changed, err := s.repo.Confirm(ctx, o.ID, ev)
		if changed {
			ordersFinished.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "confirmed"), attribute.String("code", "")))
			s.log.InfoContext(ctx, "order confirmed", "order_id", o.ID)
		}
		return err
	case domain.StatusCancelled:
		if err := s.inventory.Release(ctx, *o.ReservationID); err != nil {
			return err
		}
		_, err := s.repo.MarkReleased(ctx, o.ID)
		return err
	}
	return nil
}
