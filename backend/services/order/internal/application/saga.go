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
	case domain.StatusConfirmed, domain.StatusShipped, domain.StatusDelivered:
		return nil
	case domain.StatusCancelled:
		// The payment arrived after the order was cancelled (timeout, customer, ...). The order
		// stays cancelled; the money goes back.
		return s.requestRefund(ctx, o, o.FailureCode)
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

// OnPaymentRefunded handles PaymentRefunded: the customer has their money back.
func (s *OrderService) OnPaymentRefunded(ctx context.Context, p events.PaymentRefundedPayload) error {
	o, err := s.paymentOrder(ctx, p.OrderID)
	if err != nil {
		return err
	}
	if p.AmountCents != o.TotalCents || p.Currency != o.Currency {
		s.log.ErrorContext(ctx, "REFUND AMOUNT MISMATCH",
			"order_id", o.ID, "ordered_cents", o.TotalCents, "refunded_cents", p.AmountCents)
		return fmt.Errorf("%w: refunded %d %s for an order of %d %s", ErrUnexpectedEvent,
			p.AmountCents, p.Currency, o.TotalCents, o.Currency)
	}
	changed, err := s.repo.MarkRefunded(ctx, o.ID)
	if err != nil {
		return err
	}
	if changed {
		s.log.InfoContext(ctx, "order refunded", "order_id", o.ID, "refund_ref", p.RefundRef)
	}
	// Not changed: a repeated PaymentRefunded, or one for a payment this order never asked to refund
	// (Payment refunded a duplicate charge by itself). Nothing to do either way.
	return nil
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
			s.log.WarnContext(ctx, "reservation expired before the payment arrived; cancelling and refunding",
				"order_id", o.ID, "reservation_id", *o.ReservationID, "amount_cents", o.TotalCents)
			s.cancel(ctx, o, []domain.Status{domain.StatusPaid}, domain.CodeReservationLost,
				"The items could not be kept for you in time")
			if _, err := s.repo.MarkReleased(ctx, o.ID); err != nil {
				return err
			}
			return s.requestRefund(ctx, o, domain.CodeReservationLost)
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
