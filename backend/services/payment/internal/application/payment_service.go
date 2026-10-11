package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/services/payment/internal/repository"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/Furkanturan8/rovena/shared/telemetry"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var paymentsRefunded = telemetry.Counter("payments.refunded", "Payments given back, by the reason Order named")

var paymentsCompleted = telemetry.Counter("payments.completed", "Payments that reached a final status, by status and failure code")

// Provider is the payment gateway behind the service (a bank or PSP in production, a
// scripted fake in development). It is defined here, where it is used, so a real
// provider can be plugged in without touching the rules below.
type Provider interface {
	// Charge returns a definite answer (approved or declined) or an error when the
	// provider could not answer at all. The IdempotencyKey makes repeated calls safe.
	Charge(ctx context.Context, req domain.ChargeRequest) (domain.ChargeResult, error)
	// Refund gives a charge back. It returns an error when the provider could not answer;
	// the IdempotencyKey makes repeated calls safe.
	Refund(ctx context.Context, req domain.RefundRequest) (domain.RefundResult, error)
}

type Config struct {
	// MaxAttempts is how many times the provider is called for one payment while it keeps
	// failing to answer. After that the payment fails with provider_unavailable, so the
	// order gets an answer instead of waiting forever.
	MaxAttempts int
	// ChargeTimeout bounds one provider call.
	ChargeTimeout time.Duration
}

type PaymentService struct {
	repo     repository.PaymentRepository
	provider Provider
	log      *slog.Logger
	cfg      Config
}

func NewPaymentService(repo repository.PaymentRepository, provider Provider, log *slog.Logger, cfg Config) *PaymentService {
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 1
	}
	if log == nil {
		log = slog.Default()
	}
	return &PaymentService{repo: repo, provider: provider, log: log, cfg: cfg}
}

// ProcessInput is a request to charge an order.
type ProcessInput struct {
	OrderID        uuid.UUID
	CustomerID     uuid.UUID
	IdempotencyKey string
	AmountCents    int64
	Currency       string
	PaymentMethod  string
}

// Process charges the order and records the result as an event. It is safe to call any
// number of times for the same IdempotencyKey, from any number of workers:
//
//   - the first call creates the payment and talks to the provider;
//   - a call for a payment that already finished charges nothing and publishes the
//     stored result again, which repairs the case where Order never received it;
//   - a call for a payment still pending (a crash in between) continues where it stopped,
//     and the provider recognises the key if it had already charged.
//
// A non-nil error means "try again later": the payment is still pending.
func (s *PaymentService) Process(ctx context.Context, in ProcessInput) error {
	if err := validate(in); err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	now := time.Now()
	candidate := &domain.Payment{
		ID: id, OrderID: in.OrderID, CustomerID: in.CustomerID, IdempotencyKey: in.IdempotencyKey,
		AmountCents: in.AmountCents, Currency: in.Currency, PaymentMethod: in.PaymentMethod,
		Status: domain.StatusPending, CreatedAt: now, UpdatedAt: now,
	}

	p, created, err := s.repo.Begin(ctx, candidate)
	if err != nil {
		return err
	}
	if !created && !p.SameRequest(candidate) {
		return fmt.Errorf("%w: %q", domain.ErrIdempotencyKeyReused, in.IdempotencyKey)
	}
	if p.Status != domain.StatusPending {
		return s.republish(ctx, p)
	}
	return s.charge(ctx, p)
}

func (s *PaymentService) charge(ctx context.Context, p *domain.Payment) error {
	// An order is paid once, whatever the key. Checked before the provider is called,
	// because a charge cannot be undone here.
	paid, err := s.repo.HasSucceeded(ctx, p.OrderID)
	if err != nil {
		return err
	}
	if paid {
		return s.complete(ctx, p, domain.Failed(domain.CodeOrderAlreadyPaid, "This order has already been paid"))
	}

	p, err = s.repo.StartAttempt(ctx, p.ID)
	if err != nil {
		return err
	}
	if p.Status != domain.StatusPending {
		return nil // another worker finished it and published the result
	}

	cctx, cancel := context.WithTimeout(ctx, s.cfg.ChargeTimeout)
	defer cancel()
	res, err := s.provider.Charge(cctx, domain.ChargeRequest{
		IdempotencyKey: p.IdempotencyKey, AmountCents: p.AmountCents,
		Currency: p.Currency, PaymentMethod: p.PaymentMethod,
	})
	switch {
	case err != nil:
		s.log.WarnContext(ctx, "payment provider did not answer",
			"payment_id", p.ID, "order_id", p.OrderID, "attempt", p.Attempts, "error", err)
		if p.Attempts >= s.cfg.MaxAttempts {
			return s.complete(ctx, p, domain.Failed(domain.CodeProviderUnavailable,
				"The payment provider is unavailable, please try again later"))
		}
		return fmt.Errorf("charge payment %s (attempt %d of %d): %w", p.ID, p.Attempts, s.cfg.MaxAttempts, err)
	case res.Approved:
		return s.complete(ctx, p, domain.Succeeded(res.Reference))
	default:
		return s.complete(ctx, p, domain.Failed(res.DeclineCode, res.DeclineMessage))
	}
}

// complete stores the outcome and its event together. If the payment was completed
// by someone else in the meantime, nothing is written: they published the event.
func (s *PaymentService) complete(ctx context.Context, p *domain.Payment, o domain.Outcome) error {
	ev, err := newEvent(ctx, p, o)
	if err != nil {
		return err
	}
	_, applied, err := s.repo.Complete(ctx, p.ID, o, ev)
	if err != nil {
		if errors.Is(err, domain.ErrOrderAlreadyPaid) {
			return s.refundDuplicate(ctx, p, o)
		}
		return err
	}
	if applied {
		paymentsCompleted.Add(ctx, 1, metric.WithAttributes(
			attribute.String("status", string(o.Status)), attribute.String("failure_code", o.FailureCode)))
		s.log.InfoContext(ctx, "payment completed",
			"payment_id", p.ID, "order_id", p.OrderID, "status", o.Status, "failure_code", o.FailureCode)
	}
	return nil
}

// refundDuplicate handles the one case where the provider took money for a second payment of
// an order that is already paid (two requests with different keys raced past the pre-check).
// The database refused to record the second success, so the money is given back at once and the
// payment ends as failed with order_already_paid. If even the refund fails, the error is returned:
// the message is retried, and the log line names what a person would need.
func (s *PaymentService) refundDuplicate(ctx context.Context, p *domain.Payment, charged domain.Outcome) error {
	ref, err := s.provider.Refund(ctx, domain.RefundRequest{
		IdempotencyKey: "duplicate:" + p.ID.String(), ChargeRef: charged.ProviderRef,
		AmountCents: p.AmountCents, Currency: p.Currency,
	})
	if err != nil {
		s.log.ErrorContext(ctx, "DOUBLE CHARGE: order paid twice and the refund failed, will retry",
			"payment_id", p.ID, "order_id", p.OrderID, "provider_ref", charged.ProviderRef, "error", err)
		return fmt.Errorf("refund duplicate charge of payment %s: %w", p.ID, err)
	}
	s.log.WarnContext(ctx, "order was paid twice; the duplicate charge was refunded",
		"payment_id", p.ID, "order_id", p.OrderID, "provider_ref", charged.ProviderRef, "refund_ref", ref.Reference)
	return s.complete(ctx, p, domain.Failed(domain.CodeOrderAlreadyPaid, "This order has already been paid; the duplicate charge was refunded"))
}

// RefundInput asks to give back the money of an order.
type RefundInput struct {
	OrderID uuid.UUID
	Reason  string
}

// Refund gives the money of an order's succeeded payment back and announces it with
// PaymentRefunded. It is safe to call any number of times: a refunded payment is not refunded
// again, the stored result is published again (which repairs a PaymentRefunded Order never got).
// A non-nil error that is not ErrNothingToRefund means "try again later".
func (s *PaymentService) Refund(ctx context.Context, in RefundInput) error {
	if in.OrderID == uuid.Nil {
		return invalid("order_id is required")
	}
	p, err := s.repo.PaidByOrder(ctx, in.OrderID)
	if errors.Is(err, domain.ErrPaymentNotFound) {
		return fmt.Errorf("%w: order %s", domain.ErrNothingToRefund, in.OrderID)
	}
	if err != nil {
		return err
	}
	if p.Status == domain.StatusRefunded {
		ev, err := newRefundedEvent(ctx, p)
		if err != nil {
			return err
		}
		s.log.InfoContext(ctx, "duplicate refund request, result sent again", "payment_id", p.ID, "order_id", p.OrderID)
		return s.repo.Enqueue(ctx, ev)
	}

	cctx, cancel := context.WithTimeout(ctx, s.cfg.ChargeTimeout)
	defer cancel()
	res, err := s.provider.Refund(cctx, domain.RefundRequest{
		IdempotencyKey: "refund:" + p.ID.String(), ChargeRef: p.ProviderRef,
		AmountCents: p.AmountCents, Currency: p.Currency,
	})
	if err != nil {
		s.log.WarnContext(ctx, "refund: payment provider did not answer",
			"payment_id", p.ID, "order_id", p.OrderID, "error", err)
		return fmt.Errorf("refund payment %s: %w", p.ID, err)
	}
	done := *p
	done.RefundRef = res.Reference
	ev, err := newRefundedEvent(ctx, &done)
	if err != nil {
		return err
	}
	_, applied, err := s.repo.Refund(ctx, p.ID, res.Reference, ev)
	if err != nil {
		return err
	}
	if applied {
		paymentsRefunded.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", in.Reason)))
		s.log.InfoContext(ctx, "payment refunded", "payment_id", p.ID, "order_id", p.OrderID,
			"reason", in.Reason, "refund_ref", res.Reference)
	}
	return nil
}

func newRefundedEvent(ctx context.Context, p *domain.Payment) (messaging.Envelope, error) {
	return messaging.NewEnvelope(ctx, events.PaymentRefunded, events.PaymentRefundedVersion,
		events.PaymentRefundedPayload{
			PaymentID: p.ID, OrderID: p.OrderID, AmountCents: p.AmountCents,
			Currency: p.Currency, RefundRef: p.RefundRef,
		})
}

// republish sends the stored result of a finished payment again.
func (s *PaymentService) republish(ctx context.Context, p *domain.Payment) error {
	var ev messaging.Envelope
	var err error
	if p.Status == domain.StatusRefunded {
		// Order asked for this payment again after it was refunded (its cancel and refund raced
		// with a retry): the answer is the refund, not a success.
		ev, err = newRefundedEvent(ctx, p)
	} else {
		ev, err = newEvent(ctx, p, domain.Outcome{
			Status: p.Status, ProviderRef: p.ProviderRef,
			FailureCode: p.FailureCode, FailureMessage: p.FailureMessage,
		})
	}
	if err != nil {
		return err
	}
	s.log.InfoContext(ctx, "duplicate payment request, result sent again",
		"payment_id", p.ID, "order_id", p.OrderID, "status", p.Status)
	return s.repo.Enqueue(ctx, ev)
}

func newEvent(ctx context.Context, p *domain.Payment, o domain.Outcome) (messaging.Envelope, error) {
	if o.Status == domain.StatusSucceeded {
		return messaging.NewEnvelope(ctx, events.PaymentSucceeded, events.PaymentSucceededVersion,
			events.PaymentSucceededPayload{
				PaymentID: p.ID, OrderID: p.OrderID, AmountCents: p.AmountCents,
				Currency: p.Currency, ProviderRef: o.ProviderRef,
			})
	}
	return messaging.NewEnvelope(ctx, events.PaymentFailed, events.PaymentFailedVersion,
		events.PaymentFailedPayload{
			PaymentID: p.ID, OrderID: p.OrderID, AmountCents: p.AmountCents, Currency: p.Currency,
			FailureCode: o.FailureCode, FailureMessage: o.FailureMessage,
		})
}

func validate(in ProcessInput) error {
	switch {
	case in.OrderID == uuid.Nil:
		return invalid("order_id is required")
	case in.CustomerID == uuid.Nil:
		return invalid("customer_id is required")
	case in.IdempotencyKey == "" || len(in.IdempotencyKey) > 200:
		return invalid("idempotency_key must be 1-200 characters")
	case in.AmountCents <= 0:
		return invalid("amount_cents must be positive")
	case in.Currency != domain.DefaultCurrency:
		return invalid("only " + domain.DefaultCurrency + " is supported")
	case in.PaymentMethod == "" || len(in.PaymentMethod) > 100:
		return invalid("payment_method must be 1-100 characters")
	}
	return nil
}

func invalid(detail string) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalidRequest, detail)
}
