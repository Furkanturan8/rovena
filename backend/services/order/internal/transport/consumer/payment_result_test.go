package consumer_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/application"
	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/services/order/internal/repository/memory"
	"github.com/Furkanturan8/rovena/services/order/internal/transport/consumer"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/google/uuid"
)

type catalog struct{ id uuid.UUID }

func (c catalog) Lookup(context.Context, []uuid.UUID) ([]application.VariantInfo, error) {
	return []application.VariantInfo{{VariantID: c.id, ProductID: uuid.New(), ProductName: "Shirt", SKU: "S-1", PriceCents: 1000, Currency: "TRY", Sellable: true}}, nil
}

type inventory struct{ commitErr error }

func (inventory) Reserve(context.Context, uuid.UUID, []application.ReservationLine) (*application.Reservation, error) {
	return &application.Reservation{ID: uuid.New()}, nil
}
func (i inventory) Commit(context.Context, uuid.UUID) error { return i.commitErr }
func (inventory) Release(context.Context, uuid.UUID) error  { return nil }

type fixture struct {
	repo    *memory.Repo
	handler *consumer.PaymentResultHandler
	order   *domain.Order
}

func newFixture(t *testing.T, inv inventory) *fixture {
	t.Helper()
	repo := memory.New(time.Now)
	variant := uuid.New()
	svc := application.NewOrderService(repo, catalog{variant}, inv, slog.New(slog.NewTextHandler(io.Discard, nil)), application.Config{})
	customer := uuid.New()
	_ = repo.SetCartItem(context.Background(), customer, domain.CartItem{VariantID: variant, Quantity: 2})
	o, _, err := svc.Checkout(context.Background(), application.CheckoutInput{CustomerID: customer, IdempotencyKey: "k", PaymentMethod: "tok_visa"})
	if err != nil || o.Status != domain.StatusAwaitingPayment {
		t.Fatalf("setup: %+v %v", o, err)
	}
	return &fixture{repo: repo, handler: consumer.NewPaymentResultHandler(svc), order: o}
}

func envelope(t *testing.T, typ string, version int, payload any) messaging.Envelope {
	t.Helper()
	env, err := messaging.NewEnvelope(context.Background(), typ, version, payload)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func (f *fixture) status() domain.Status {
	o, _ := f.repo.GetByID(context.Background(), f.order.ID)
	return o.Status
}

func TestSucceededConfirmsTheOrder(t *testing.T) {
	f := newFixture(t, inventory{})
	env := envelope(t, events.PaymentSucceeded, 1, events.PaymentSucceededPayload{
		PaymentID: uuid.New(), OrderID: f.order.ID, AmountCents: f.order.TotalCents, Currency: "TRY", ProviderRef: "r",
	})
	if err := f.handler.Handle(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	if f.status() != domain.StatusConfirmed {
		t.Fatalf("status = %s", f.status())
	}
}

func TestFailedCancelsTheOrder(t *testing.T) {
	f := newFixture(t, inventory{})
	env := envelope(t, events.PaymentFailed, 1, events.PaymentFailedPayload{
		PaymentID: uuid.New(), OrderID: f.order.ID, AmountCents: f.order.TotalCents, Currency: "TRY", FailureCode: "card_declined",
	})
	if err := f.handler.Handle(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	if f.status() != domain.StatusCancelled {
		t.Fatalf("status = %s", f.status())
	}
}

// Errors that retrying can not fix must not loop in the retry queue.
func TestUnfixableMessagesAreDeadLettered(t *testing.T) {
	f := newFixture(t, inventory{})
	good := events.PaymentSucceededPayload{PaymentID: uuid.New(), OrderID: f.order.ID, AmountCents: f.order.TotalCents, Currency: "TRY"}

	wrongAmount := good
	wrongAmount.AmountCents++
	unknownOrder := good
	unknownOrder.OrderID = uuid.New()

	cases := map[string]messaging.Envelope{
		"unknown event type":    envelope(t, "SomethingElse", 1, good),
		"future version":        envelope(t, events.PaymentSucceeded, 9, good),
		"future failed version": envelope(t, events.PaymentFailed, 9, good),
		"unreadable payload":    envelope(t, events.PaymentSucceeded, 1, "not an object"),
		"unknown order":         envelope(t, events.PaymentSucceeded, 1, unknownOrder),
		"wrong amount":          envelope(t, events.PaymentSucceeded, 1, wrongAmount),
		"failed for no order":   envelope(t, events.PaymentFailed, 1, events.PaymentFailedPayload{OrderID: uuid.New()}),
		"refund for no order":   envelope(t, events.PaymentRefunded, 1, events.PaymentRefundedPayload{OrderID: uuid.New()}),
		"refund future version": envelope(t, events.PaymentRefunded, 9, events.PaymentRefundedPayload{OrderID: f.order.ID}),
		"refund wrong amount": envelope(t, events.PaymentRefunded, 1, events.PaymentRefundedPayload{
			OrderID: f.order.ID, AmountCents: f.order.TotalCents + 1, Currency: "TRY"}),
	}
	for name, env := range cases {
		err := f.handler.Handle(context.Background(), env)
		if err == nil || !messaging.IsPermanent(err) {
			t.Errorf("%s: err = %v, want a permanent error", name, err)
		}
	}
	if f.status() != domain.StatusAwaitingPayment {
		t.Fatalf("none of those may change the order, status = %s", f.status())
	}
}

// Inventory being down is worth a retry, not a trip to the dead-letter queue.
func TestInventoryOutageIsRetried(t *testing.T) {
	f := newFixture(t, inventory{commitErr: errors.New("connection refused")})
	env := envelope(t, events.PaymentSucceeded, 1, events.PaymentSucceededPayload{
		PaymentID: uuid.New(), OrderID: f.order.ID, AmountCents: f.order.TotalCents, Currency: "TRY",
	})
	err := f.handler.Handle(context.Background(), env)
	if err == nil || messaging.IsPermanent(err) {
		t.Fatalf("err = %v, want a retryable error", err)
	}
	if f.status() != domain.StatusPaid {
		t.Fatalf("status = %s; the payment is recorded even though the commit is pending", f.status())
	}
}

func TestSubscriptionListensToBothResults(t *testing.T) {
	sub := consumer.Subscription(time.Second)
	if sub.Queue != consumer.Queue || len(sub.Types) != 3 || sub.MaxRetries < 1 {
		t.Fatalf("subscription = %+v", sub)
	}
}

func TestRefundedClosesTheRefundOfACancelledOrder(t *testing.T) {
	f := newFixture(t, inventory{})
	ctx := context.Background()
	// the order is cancelled, then the payment succeeds anyway, which requests the refund
	if err := f.handler.Handle(ctx, envelope(t, events.PaymentFailed, 1, events.PaymentFailedPayload{
		PaymentID: uuid.New(), OrderID: f.order.ID, AmountCents: f.order.TotalCents, Currency: "TRY", FailureCode: "payment_timeout"})); err != nil {
		t.Fatal(err)
	}
	ok := events.PaymentSucceededPayload{PaymentID: uuid.New(), OrderID: f.order.ID, AmountCents: f.order.TotalCents, Currency: "TRY", ProviderRef: "r"}
	if err := f.handler.Handle(ctx, envelope(t, events.PaymentSucceeded, 1, ok)); err != nil {
		t.Fatal(err)
	}
	refund := events.PaymentRefundedPayload{PaymentID: uuid.New(), OrderID: f.order.ID, AmountCents: f.order.TotalCents, Currency: "TRY", RefundRef: "rf"}
	if err := f.handler.Handle(ctx, envelope(t, events.PaymentRefunded, 1, refund)); err != nil {
		t.Fatal(err)
	}
	o, _ := f.repo.GetByID(ctx, f.order.ID)
	if o.Status != domain.StatusCancelled || o.RefundState != domain.RefundRefunded {
		t.Fatalf("order = %+v", o)
	}
}
