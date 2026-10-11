//go:build integration

package postgres_test

// Runs against a real PostgreSQL with the payment schema applied:
//   PAYMENT_TEST_DATABASE_URL=postgres://... go test -tags integration ./internal/repository/postgres
// Every test uses its own random order IDs and removes its rows afterwards.
//
// These tests exist for the guarantees only a database can prove: one payment per
// idempotency key under concurrency, one winner per completion, "status changed if and
// only if the event is queued", and one successful payment per order.

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/services/payment/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/payment/internal/repository/postgres"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type harness struct {
	ctx  context.Context
	db   *bun.DB
	repo *postgres.PaymentRepository

	orders []uuid.UUID
}

func setup(t *testing.T) *harness {
	t.Helper()
	dsn := os.Getenv("PAYMENT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PAYMENT_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{ctx: ctx, db: db, repo: postgres.NewPaymentRepository(db)}
	t.Cleanup(func() {
		for _, o := range h.orders {
			db.ExecContext(ctx, "DELETE FROM outbox_events WHERE envelope->'payload'->>'order_id' = ?", o.String())
			db.ExecContext(ctx, "DELETE FROM payments WHERE order_id = ?", o)
		}
		db.Close()
	})
	return h
}

func (h *harness) newPayment(orderID uuid.UUID, key string) *domain.Payment {
	h.orders = append(h.orders, orderID)
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.Payment{
		ID: uuid.Must(uuid.NewV7()), OrderID: orderID, CustomerID: uuid.New(), IdempotencyKey: key,
		AmountCents: 459900, Currency: "TRY", PaymentMethod: "tok_visa",
		Status: domain.StatusPending, CreatedAt: now, UpdatedAt: now,
	}
}

func (h *harness) event(t *testing.T, p *domain.Payment, o domain.Outcome) messaging.Envelope {
	t.Helper()
	var env messaging.Envelope
	var err error
	if o.Status == domain.StatusSucceeded {
		env, err = messaging.NewEnvelope(h.ctx, events.PaymentSucceeded, 1, events.PaymentSucceededPayload{
			PaymentID: p.ID, OrderID: p.OrderID, AmountCents: p.AmountCents, Currency: p.Currency, ProviderRef: o.ProviderRef,
		})
	} else {
		env, err = messaging.NewEnvelope(h.ctx, events.PaymentFailed, 1, events.PaymentFailedPayload{
			PaymentID: p.ID, OrderID: p.OrderID, AmountCents: p.AmountCents, Currency: p.Currency,
			FailureCode: o.FailureCode, FailureMessage: o.FailureMessage,
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func (h *harness) outboxCount(t *testing.T, orderID uuid.UUID) int {
	t.Helper()
	var n int
	err := h.db.NewRaw("SELECT count(*) FROM outbox_events WHERE envelope->'payload'->>'order_id' = ?", orderID.String()).Scan(h.ctx, &n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func (h *harness) begin(t *testing.T, p *domain.Payment) *domain.Payment {
	t.Helper()
	stored, created, err := h.repo.Begin(h.ctx, p)
	if err != nil || !created {
		t.Fatalf("begin: created=%v err=%v", created, err)
	}
	return stored
}

func TestBeginStoresAllFields(t *testing.T) {
	h := setup(t)
	p := h.newPayment(uuid.New(), "k-"+uuid.NewString())
	h.begin(t, p)

	got, err := h.repo.GetByID(h.ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.OrderID != p.OrderID || got.CustomerID != p.CustomerID || got.IdempotencyKey != p.IdempotencyKey ||
		got.AmountCents != 459900 || got.Currency != "TRY" || got.PaymentMethod != "tok_visa" ||
		got.Status != domain.StatusPending || got.Attempts != 0 || got.CompletedAt != nil {
		t.Fatalf("round trip changed the payment: %+v", got)
	}
}

func TestBeginIsIdempotentPerKey(t *testing.T) {
	h := setup(t)
	key := "k-" + uuid.NewString()
	first := h.newPayment(uuid.New(), key)
	h.begin(t, first)

	again := h.newPayment(first.OrderID, key) // a new candidate ID, same key
	stored, created, err := h.repo.Begin(h.ctx, again)
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if stored.ID != first.ID {
		t.Fatalf("got payment %s, want the first one %s", stored.ID, first.ID)
	}
	list, total, _ := h.repo.List(h.ctx, domain.Filter{OrderID: &first.OrderID, Page: 1, PageSize: 10})
	if total != 1 || len(list) != 1 {
		t.Fatalf("%d payments exist for one key", total)
	}
}

func TestConcurrentBeginsWithOneKeyCreateOnePayment(t *testing.T) {
	h := setup(t)
	order, key := uuid.New(), "k-"+uuid.NewString()

	var created atomic.Int32
	ids := make(chan uuid.UUID, 16)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stored, isNew, err := h.repo.Begin(h.ctx, h.newPaymentLocked(order, key))
			if err != nil {
				t.Error(err)
				return
			}
			if isNew {
				created.Add(1)
			}
			ids <- stored.ID
		}()
	}
	wg.Wait()
	close(ids)

	if created.Load() != 1 {
		t.Fatalf("%d goroutines believe they created the payment", created.Load())
	}
	var first uuid.UUID
	for id := range ids {
		if first == uuid.Nil {
			first = id
		}
		if id != first {
			t.Fatalf("callers got different payments: %s and %s", first, id)
		}
	}
}

var ordersMu sync.Mutex

// newPaymentLocked is newPayment for goroutines: it only appends to the shared slice under a lock.
func (h *harness) newPaymentLocked(orderID uuid.UUID, key string) *domain.Payment {
	ordersMu.Lock()
	defer ordersMu.Unlock()
	return h.newPayment(orderID, key)
}

func TestCompleteChangesStatusAndQueuesEventTogether(t *testing.T) {
	h := setup(t)
	p := h.begin(t, h.newPayment(uuid.New(), "k-"+uuid.NewString()))
	o := domain.Succeeded("ref_1")
	ev := h.event(t, p, o)

	cur, applied, err := h.repo.Complete(h.ctx, p.ID, o, ev)
	if err != nil || !applied {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
	if cur.Status != domain.StatusSucceeded || cur.ProviderRef != "ref_1" || cur.CompletedAt == nil {
		t.Fatalf("unexpected payment: %+v", cur)
	}
	if h.outboxCount(t, p.OrderID) != 1 {
		t.Fatal("the event was not queued")
	}
	var pending int
	h.db.NewRaw("SELECT count(*) FROM outbox_events WHERE id = ? AND published_at IS NULL", ev.ID).Scan(h.ctx, &pending)
	if pending != 1 {
		t.Fatal("the queued event must be waiting for the relay")
	}
}

func TestCompleteTwiceKeepsTheFirstOutcome(t *testing.T) {
	h := setup(t)
	p := h.begin(t, h.newPayment(uuid.New(), "k-"+uuid.NewString()))
	first := domain.Succeeded("ref_first")
	if _, applied, err := h.repo.Complete(h.ctx, p.ID, first, h.event(t, p, first)); err != nil || !applied {
		t.Fatalf("applied=%v err=%v", applied, err)
	}

	other := domain.Failed(domain.CodeCardDeclined, "declined")
	cur, applied, err := h.repo.Complete(h.ctx, p.ID, other, h.event(t, p, other))
	if err != nil {
		t.Fatal(err)
	}
	if applied || cur.Status != domain.StatusSucceeded || cur.ProviderRef != "ref_first" {
		t.Fatalf("a finished payment must not change: applied=%v %+v", applied, cur)
	}
	if h.outboxCount(t, p.OrderID) != 1 {
		t.Fatalf("expected the first event only, found %d", h.outboxCount(t, p.OrderID))
	}
}

func TestConcurrentCompletionsHaveExactlyOneWinner(t *testing.T) {
	h := setup(t)
	p := h.begin(t, h.newPayment(uuid.New(), "k-"+uuid.NewString()))

	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		o := domain.Failed(domain.CodeCardDeclined, "declined")
		if i%2 == 0 {
			o = domain.Succeeded("ref")
		}
		ev := h.event(t, p, o)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, applied, err := h.repo.Complete(h.ctx, p.ID, o, ev)
			if err != nil {
				t.Error(err)
			}
			if applied {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()

	if winners.Load() != 1 {
		t.Fatalf("%d completions were applied", winners.Load())
	}
	if n := h.outboxCount(t, p.OrderID); n != 1 {
		t.Fatalf("%d events were queued for one payment", n)
	}
}

func TestFailedEventInsertRollsBackTheStatusChange(t *testing.T) {
	h := setup(t)
	p := h.begin(t, h.newPayment(uuid.New(), "k-"+uuid.NewString()))
	o := domain.Succeeded("ref_1")
	ev := h.event(t, p, o)
	// Occupy the event ID, so storing the event inside Complete fails.
	if err := h.repo.Enqueue(h.ctx, ev); err != nil {
		t.Fatal(err)
	}

	_, applied, err := h.repo.Complete(h.ctx, p.ID, o, ev)
	if err == nil || applied {
		t.Fatalf("expected the duplicate event to fail the completion, applied=%v err=%v", applied, err)
	}

	got, _ := h.repo.GetByID(h.ctx, p.ID)
	if got.Status != domain.StatusPending || got.ProviderRef != "" || got.CompletedAt != nil {
		t.Fatalf("the payment finished without its event: %+v", got)
	}
}

func TestAnOrderCanSucceedOnlyOnce(t *testing.T) {
	h := setup(t)
	order := uuid.New()
	a := h.begin(t, h.newPayment(order, "k-"+uuid.NewString()))
	b := h.begin(t, h.newPayment(order, "k-"+uuid.NewString()))

	oa := domain.Succeeded("ref_a")
	if _, applied, err := h.repo.Complete(h.ctx, a.ID, oa, h.event(t, a, oa)); err != nil || !applied {
		t.Fatalf("first success: applied=%v err=%v", applied, err)
	}
	if paid, _ := h.repo.HasSucceeded(h.ctx, order); !paid {
		t.Fatal("HasSucceeded should be true")
	}

	ob := domain.Succeeded("ref_b")
	_, applied, err := h.repo.Complete(h.ctx, b.ID, ob, h.event(t, b, ob))
	if !errors.Is(err, domain.ErrOrderAlreadyPaid) || applied {
		t.Fatalf("a second success must be refused: applied=%v err=%v", applied, err)
	}
	got, _ := h.repo.GetByID(h.ctx, b.ID)
	if got.Status != domain.StatusPending {
		t.Fatalf("the refused payment must stay pending, got %s", got.Status)
	}
	if h.outboxCount(t, order) != 1 {
		t.Fatal("the refused completion must not leave an event")
	}

	// Failing the second attempt is fine: only successes are limited.
	fail := domain.Failed(domain.CodeOrderAlreadyPaid, "already paid")
	if _, applied, err := h.repo.Complete(h.ctx, b.ID, fail, h.event(t, b, fail)); err != nil || !applied {
		t.Fatalf("failing it should work: applied=%v err=%v", applied, err)
	}
}

func TestDatabaseRejectsInconsistentOutcomes(t *testing.T) {
	h := setup(t)
	p := h.begin(t, h.newPayment(uuid.New(), "k-"+uuid.NewString()))

	for name, set := range map[string]string{
		"succeeded without a provider reference": "status = 'succeeded', completed_at = now()",
		"failed without a failure code":          "status = 'failed', completed_at = now()",
		"finished without a completion time":     "status = 'failed', failure_code = 'x'",
		"unknown status":                         "status = 'refunded'",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := h.db.ExecContext(h.ctx, "UPDATE payments SET "+set+" WHERE id = ?", p.ID); err == nil {
				t.Fatal("the database accepted an impossible payment state")
			}
		})
	}
	for name, stmt := range map[string]string{
		"zero amount":  "UPDATE payments SET amount_cents = 0 WHERE id = ?",
		"bad currency": "UPDATE payments SET currency = 'TL' WHERE id = ?",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := h.db.ExecContext(h.ctx, stmt, p.ID); err == nil {
				t.Fatal("the database accepted an invalid value")
			}
		})
	}
}

func TestStartAttemptCountsOnlyWhilePending(t *testing.T) {
	h := setup(t)
	p := h.begin(t, h.newPayment(uuid.New(), "k-"+uuid.NewString()))

	for want := 1; want <= 3; want++ {
		got, err := h.repo.StartAttempt(h.ctx, p.ID)
		if err != nil || got.Attempts != want {
			t.Fatalf("attempt %d: %+v, %v", want, got, err)
		}
	}

	o := domain.Failed(domain.CodeProviderUnavailable, "down")
	h.repo.Complete(h.ctx, p.ID, o, h.event(t, p, o))
	got, err := h.repo.StartAttempt(h.ctx, p.ID)
	if err != nil || got.Attempts != 3 || got.Status != domain.StatusFailed {
		t.Fatalf("a finished payment must come back unchanged: %+v, %v", got, err)
	}

	if _, err := h.repo.StartAttempt(h.ctx, uuid.New()); !errors.Is(err, domain.ErrPaymentNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestLatestByOrderAndList(t *testing.T) {
	h := setup(t)
	order := uuid.New()
	older := h.newPayment(order, "k-"+uuid.NewString())
	older.CreatedAt = older.CreatedAt.Add(-time.Hour)
	h.begin(t, older)
	newer := h.begin(t, h.newPayment(order, "k-"+uuid.NewString()))
	o := domain.Failed(domain.CodeCardDeclined, "declined")
	h.repo.Complete(h.ctx, older.ID, o, h.event(t, older, o))

	latest, err := h.repo.LatestByOrder(h.ctx, order)
	if err != nil || latest.ID != newer.ID {
		t.Fatalf("latest = %v, %v; want %s", latest, err, newer.ID)
	}
	if _, err := h.repo.LatestByOrder(h.ctx, uuid.New()); !errors.Is(err, domain.ErrPaymentNotFound) {
		t.Fatalf("got %v", err)
	}

	all, total, err := h.repo.List(h.ctx, domain.Filter{OrderID: &order, Page: 1, PageSize: 10})
	if err != nil || total != 2 || len(all) != 2 || all[0].ID != newer.ID {
		t.Fatalf("list: total=%d err=%v %+v", total, err, all)
	}
	failed, total, _ := h.repo.List(h.ctx, domain.Filter{OrderID: &order, Status: domain.StatusFailed, Page: 1, PageSize: 10})
	if total != 1 || len(failed) != 1 || failed[0].ID != older.ID {
		t.Fatalf("status filter: total=%d %+v", total, failed)
	}
	page, total, _ := h.repo.List(h.ctx, domain.Filter{OrderID: &order, Page: 2, PageSize: 1})
	if total != 2 || len(page) != 1 || page[0].ID != older.ID {
		t.Fatalf("pagination: total=%d %+v", total, page)
	}
}
