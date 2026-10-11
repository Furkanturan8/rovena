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
	"github.com/google/uuid"
)

// Provider is the payment gateway behind the service (a bank or PSP in production, a
// scripted fake in development). It is defined here, where it is used, so a real
// provider can be plugged in without touching the rules below.
type Provider interface {
	// Charge returns a definite answer (approved or declined) or an error when the
	// provider could not answer at all. The IdempotencyKey makes repeated calls safe.
	Charge(ctx context.Context, req domain.ChargeRequest) (domain.ChargeResult, error)
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
			// The provider already took the money for a second payment of the same order.
			// The database refused to record it; this needs a human and a refund.
			s.log.ErrorContext(ctx, "DOUBLE CHARGE: order paid twice, refund required",
				"payment_id", p.ID, "order_id", p.OrderID, "provider_ref", o.ProviderRef)
		}
		return err
	}
	if applied {
		s.log.InfoContext(ctx, "payment completed",
			"payment_id", p.ID, "order_id", p.OrderID, "status", o.Status, "failure_code", o.FailureCode)
	}
	return nil
}

// republish sends the stored result of a finished payment again.
func (s *PaymentService) republish(ctx context.Context, p *domain.Payment) error {
	ev, err := newEvent(ctx, p, domain.Outcome{
		Status: p.Status, ProviderRef: p.ProviderRef,
		FailureCode: p.FailureCode, FailureMessage: p.FailureMessage,
	})
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
