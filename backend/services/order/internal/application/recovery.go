package application

import (
	"context"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/domain"
)

// RunRecovery calls Recover every interval until ctx ends.
func (s *OrderService) RunRecovery(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := s.Recover(ctx); err != nil {
				s.log.ErrorContext(ctx, "order recovery failed", "error", err)
			} else if n > 0 {
				s.log.InfoContext(ctx, "order recovery pass", "orders", n)
			}
		}
	}
}

// Recover finds orders that stopped between two saga steps and repairs them. It
// returns how many orders it looked at.
//
//   - pending: stock was never reserved (crash, or Inventory down). Try again; give up
//     and cancel after PendingMaxAge.
//   - awaiting_payment: no payment answer. Ask again up to MaxPaymentRequests times
//     (Payment answers a repeated request from its stored result), then cancel.
//   - paid / cancelled with the reservation still held: finish the commit or the release.
func (s *OrderService) Recover(ctx context.Context) (int, error) {
	now := s.now()
	orders, err := s.repo.Unfinished(ctx, now.Add(-s.cfg.StuckAfter), s.cfg.RecoveryBatch)
	if err != nil {
		return 0, err
	}
	handled := 0
	for i := range orders {
		if s.recoverOne(ctx, &orders[i], now) {
			handled++
		}
	}
	return handled, nil
}

func (s *OrderService) recoverOne(ctx context.Context, o *domain.Order, now time.Time) bool {
	switch o.Status {
	case domain.StatusPending:
		if now.Sub(o.CreatedAt) > s.cfg.PendingMaxAge {
			s.cancel(ctx, o, []domain.Status{domain.StatusPending}, domain.CodeCheckoutTimeout,
				"We could not reserve your items in time")
			countRecovery(ctx, "checkout_timeout")
			return true
		}
		countRecovery(ctx, "reserve_again")
		s.advance(ctx, o)
		if cur, err := s.repo.GetByID(ctx, o.ID); err == nil && cur.Status == domain.StatusPending {
			_ = s.repo.Touch(ctx, o.ID)
		}
		return true

	case domain.StatusAwaitingPayment:
		if now.Sub(o.UpdatedAt) < s.cfg.PaymentTimeout {
			return false
		}
		if o.PaymentRequests >= s.cfg.MaxPaymentRequests {
			if s.cancel(ctx, o, []domain.Status{domain.StatusAwaitingPayment}, domain.CodePaymentTimeout,
				"The payment did not complete in time") {
				s.settleOrLog(ctx, o)
			}
			countRecovery(ctx, "payment_timeout")
			return true
		}
		countRecovery(ctx, "payment_requested_again")
		ev, err := s.paymentRequested(ctx, o)
		if err != nil {
			s.log.ErrorContext(ctx, "build PaymentRequested", "order_id", o.ID, "error", err)
			return true
		}
		if _, err := s.repo.RequestPaymentAgain(ctx, o.ID, ev); err != nil {
			s.log.ErrorContext(ctx, "request payment again", "order_id", o.ID, "error", err)
		}
		return true

	case domain.StatusPaid, domain.StatusCancelled:
		countRecovery(ctx, "settle_reservation")
		s.settleOrLog(ctx, o)
		return true
	}
	return false
}

// settleOrLog settles the order's reservation; on failure it logs and moves updated_at
// so the next pass waits StuckAfter instead of retrying on every tick.
func (s *OrderService) settleOrLog(ctx context.Context, o *domain.Order) {
	cur, err := s.repo.GetByID(ctx, o.ID)
	if err == nil {
		err = s.settle(ctx, cur)
	}
	if err != nil {
		s.log.WarnContext(ctx, "settling reservation failed, will retry", "order_id", o.ID, "error", err)
		_ = s.repo.Touch(ctx, o.ID)
	}
}
