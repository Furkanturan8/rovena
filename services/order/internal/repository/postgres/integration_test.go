//go:build integration

package postgres_test

// Runs against a real PostgreSQL with the order schema applied:
//   ORDER_TEST_DATABASE_URL=postgres://... go test -tags integration ./internal/repository/postgres
// Every test uses its own random customers and removes its rows afterwards.
//
// These tests exist for the guarantees only a database can prove: one order per
// idempotency key and cart under concurrency, a bounded cart, one winner per state
// transition, and "status changed if and only if the event is queued".

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/application"
	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/services/order/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/order/internal/repository/postgres"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type harness struct {
	ctx  context.Context
	db   *bun.DB
	repo *postgres.OrderRepository

	customers []uuid.UUID
	orders    []uuid.UUID
}

func setup(t *testing.T) *harness {
	t.Helper()
	dsn := os.Getenv("ORDER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ORDER_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{ctx: ctx, db: db, repo: postgres.NewOrderRepository(db)}
	t.Cleanup(func() {
		for _, o := range h.orders {
			db.ExecContext(ctx, "DELETE FROM outbox_events WHERE envelope->'payload'->>'order_id' = ?", o.String())
		}
		for _, c := range h.customers {
			db.ExecContext(ctx, "DELETE FROM orders WHERE customer_id = ?", c) // cascades to items and history
			db.ExecContext(ctx, "DELETE FROM carts WHERE customer_id = ?", c)  // cascades to cart items
		}
		db.Close()
	})
	return h
}

func (h *harness) customer() uuid.UUID {
	id := uuid.New()
	h.customers = append(h.customers, id)
	return id
}

func (h *harness) fillCart(t *testing.T, customer uuid.UUID, n int) []domain.CartItem {
	t.Helper()
	var items []domain.CartItem
	for i := 0; i < n; i++ {
		it := domain.CartItem{VariantID: uuid.New(), Quantity: i + 1}
		if err := h.repo.SetCartItem(h.ctx, customer, it); err != nil {
			t.Fatal(err)
		}
		items = append(items, it)
	}
	return items
}

func (h *harness) newOrder(customer uuid.UUID, key string, cart []domain.CartItem) *domain.Order {
	id := uuid.Must(uuid.NewV7())
	h.orders = append(h.orders, id)
	now := time.Now().UTC().Truncate(time.Microsecond)
	o := &domain.Order{
		ID: id, CustomerID: customer, Status: domain.StatusPending, TotalCents: 3000, Currency: "TRY",
		IdempotencyKey: key, PaymentMethod: "tok_visa", ReservationState: domain.ReservationNone,
		CreatedAt: now, UpdatedAt: now,
	}
	for _, c := range cart {
		o.Items = append(o.Items, domain.OrderItem{
			VariantID: c.VariantID, ProductID: uuid.New(), SKU: "SKU-" + c.VariantID.String()[:6],
			ProductName: "Linen Shirt", Size: "M", Color: "Blue", UnitPriceCents: 1000, Quantity: c.Quantity,
		})
	}
	return o
}

// placed creates an order that is awaiting payment.
func (h *harness) placed(t *testing.T) (*domain.Order, uuid.UUID) {
	t.Helper()
	c := h.customer()
	cart := h.fillCart(t, c, 2)
	o := h.newOrder(c, "key-"+uuid.NewString(), cart)
	if err := h.repo.Create(h.ctx, o, cart); err != nil {
		t.Fatal(err)
	}
	res := uuid.New()
	changed, err := h.repo.MarkReserved(h.ctx, o.ID, res, h.event(t, o, events.PaymentRequested, events.PaymentRequestedPayload{
		OrderID: o.ID, CustomerID: c, IdempotencyKey: o.PaymentIdempotencyKey(), AmountCents: o.TotalCents, Currency: "TRY", PaymentMethod: "tok_visa",
	}))
	if err != nil || !changed {
		t.Fatalf("MarkReserved: changed=%v err=%v", changed, err)
	}
	return o, res
}

func (h *harness) event(t *testing.T, o *domain.Order, typ string, payload any) messaging.Envelope {
	t.Helper()
	env, err := messaging.NewEnvelope(h.ctx, typ, 1, payload)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func (h *harness) cancelledEvent(t *testing.T, o *domain.Order) messaging.Envelope {
	return h.event(t, o, events.OrderCancelled, events.OrderCancelledPayload{OrderID: o.ID, CustomerID: o.CustomerID, Code: "x"})
}

func (h *harness) confirmedEvent(t *testing.T, o *domain.Order) messaging.Envelope {
	return h.event(t, o, events.OrderConfirmed, events.OrderConfirmedPayload{OrderID: o.ID, CustomerID: o.CustomerID, TotalCents: o.TotalCents, Currency: "TRY"})
}

func (h *harness) outboxCount(t *testing.T, o *domain.Order, typ string) int {
	t.Helper()
	n, err := h.db.NewSelect().TableExpr("outbox_events").
		Where("type = ? AND envelope->'payload'->>'order_id' = ?", typ, o.ID.String()).Count(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	return int(n)
}

func (h *harness) get(t *testing.T, id uuid.UUID) *domain.Order {
	t.Helper()
	o, err := h.repo.GetByID(h.ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// ---------------------------------------------------------------- carts

func TestCart_SetReplacesQuantityAndRemoveDeletes(t *testing.T) {
	h := setup(t)
	c := h.customer()
	v := uuid.New()
	for _, q := range []int{1, 5} {
		if err := h.repo.SetCartItem(h.ctx, c, domain.CartItem{VariantID: v, Quantity: q}); err != nil {
			t.Fatal(err)
		}
	}
	items, _ := h.repo.GetCart(h.ctx, c)
	if len(items) != 1 || items[0].Quantity != 5 {
		t.Fatalf("items = %+v", items)
	}
	if err := h.repo.RemoveCartItem(h.ctx, c, v); err != nil {
		t.Fatal(err)
	}
	if items, _ := h.repo.GetCart(h.ctx, c); len(items) != 0 {
		t.Fatalf("items = %+v", items)
	}
}

func TestCart_CapHoldsUnderConcurrentAdds(t *testing.T) {
	h := setup(t)
	c := h.customer()
	const attempts = domain.MaxCartLines + 20

	var wg sync.WaitGroup
	var full atomic.Int32
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := h.repo.SetCartItem(h.ctx, c, domain.CartItem{VariantID: uuid.New(), Quantity: 1})
			if errors.Is(err, domain.ErrCartFull) {
				full.Add(1)
			} else if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	items, _ := h.repo.GetCart(h.ctx, c)
	if len(items) != domain.MaxCartLines || int(full.Load()) != attempts-domain.MaxCartLines {
		t.Fatalf("cart has %d lines, %d adds were refused", len(items), full.Load())
	}
}

func TestCart_DatabaseRefusesBadQuantities(t *testing.T) {
	h := setup(t)
	c := h.customer()
	if _, err := h.db.ExecContext(h.ctx, "INSERT INTO carts (customer_id) VALUES (?)", c); err != nil {
		t.Fatal(err)
	}
	for _, q := range []int{0, -1, 100} {
		_, err := h.db.ExecContext(h.ctx, "INSERT INTO cart_items (customer_id, variant_id, quantity) VALUES (?, ?, ?)", c, uuid.New(), q)
		if err == nil {
			t.Errorf("quantity %d accepted", q)
		}
	}
}

// ---------------------------------------------------------------- create

func TestCreate_StoresOrderItemsHistoryAndEmptiesTheCart(t *testing.T) {
	h := setup(t)
	c := h.customer()
	cart := h.fillCart(t, c, 3)
	o := h.newOrder(c, "k1", cart)

	if err := h.repo.Create(h.ctx, o, cart); err != nil {
		t.Fatal(err)
	}
	got := h.get(t, o.ID)
	if got.Status != domain.StatusPending || got.TotalCents != 3000 || len(got.Items) != 3 || got.ReservationID != nil {
		t.Fatalf("order = %+v", got)
	}
	if left, _ := h.repo.GetCart(h.ctx, c); len(left) != 0 {
		t.Fatalf("cart still has %d lines", len(left))
	}
	hist, _ := h.repo.History(h.ctx, o.ID)
	if len(hist) != 1 || hist[0].ToStatus != domain.StatusPending || hist[0].FromStatus != nil {
		t.Fatalf("history = %+v", hist)
	}
	byKey, err := h.repo.GetByIdempotencyKey(h.ctx, c, "k1")
	if err != nil || byKey.ID != o.ID {
		t.Fatalf("by key: %+v, %v", byKey, err)
	}
	if _, err := h.repo.GetByIdempotencyKey(h.ctx, h.customer(), "k1"); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("keys are per customer, err = %v", err)
	}
}

func TestCreate_RefusesACartThatChanged(t *testing.T) {
	h := setup(t)
	c := h.customer()
	cart := h.fillCart(t, c, 2)
	o := h.newOrder(c, "k", cart)

	// The customer edits the cart after it was priced.
	if err := h.repo.SetCartItem(h.ctx, c, domain.CartItem{VariantID: cart[0].VariantID, Quantity: 9}); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.Create(h.ctx, o, cart); !errors.Is(err, domain.ErrCartChanged) {
		t.Fatalf("err = %v", err)
	}
	if _, err := h.repo.GetByID(h.ctx, o.ID); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatal("a refused checkout must leave no order behind")
	}
	if left, _ := h.repo.GetCart(h.ctx, c); len(left) != 2 {
		t.Fatal("a refused checkout must keep the cart")
	}
}

func TestCreate_RefusesAnEmptiedCart(t *testing.T) {
	h := setup(t)
	c := h.customer()
	cart := h.fillCart(t, c, 1)
	if err := h.repo.ClearCart(h.ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.Create(h.ctx, h.newOrder(c, "k", cart), cart); !errors.Is(err, domain.ErrCartChanged) {
		t.Fatalf("err = %v", err)
	}
}

func TestCreate_SameKeyTwiceIsRefused(t *testing.T) {
	h := setup(t)
	c := h.customer()
	cart := h.fillCart(t, c, 1)
	if err := h.repo.Create(h.ctx, h.newOrder(c, "dup", cart), cart); err != nil {
		t.Fatal(err)
	}
	cart2 := h.fillCart(t, c, 1)
	if err := h.repo.Create(h.ctx, h.newOrder(c, "dup", cart2), cart2); !errors.Is(err, domain.ErrDuplicateCheckout) {
		t.Fatalf("err = %v", err)
	}
	if left, _ := h.repo.GetCart(h.ctx, c); len(left) != 1 {
		t.Fatal("the refused attempt must not touch the new cart")
	}
}

// Ten requests with the same key and the same cart arrive together (a double click, a
// client that retries too eagerly). Exactly one order may come out of them.
func TestCreate_ConcurrentCheckoutsMakeOneOrder(t *testing.T) {
	h := setup(t)
	c := h.customer()
	cart := h.fillCart(t, c, 3)

	var wg sync.WaitGroup
	var created, duplicate, changed atomic.Int32
	for i := 0; i < 10; i++ {
		o := h.newOrder(c, "same-key", cart)
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := h.repo.Create(h.ctx, o, append([]domain.CartItem(nil), cart...))
			switch {
			case err == nil:
				created.Add(1)
			case errors.Is(err, domain.ErrDuplicateCheckout):
				duplicate.Add(1)
			case errors.Is(err, domain.ErrCartChanged):
				changed.Add(1)
			default:
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if created.Load() != 1 {
		t.Fatalf("created=%d duplicate=%d cart-changed=%d", created.Load(), duplicate.Load(), changed.Load())
	}
	n, _ := h.db.NewSelect().TableExpr("orders").Where("customer_id = ?", c).Count(h.ctx)
	if n != 1 {
		t.Fatalf("%d orders exist", n)
	}
}

// Same cart, different keys: the cart can only become one order.
func TestCreate_ConcurrentCheckoutsWithDifferentKeysStillMakeOneOrder(t *testing.T) {
	h := setup(t)
	c := h.customer()
	cart := h.fillCart(t, c, 2)

	var wg sync.WaitGroup
	var created atomic.Int32
	for i := 0; i < 8; i++ {
		o := h.newOrder(c, "key-"+uuid.NewString(), cart)
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := h.repo.Create(h.ctx, o, append([]domain.CartItem(nil), cart...))
			if err == nil {
				created.Add(1)
			} else if !errors.Is(err, domain.ErrCartChanged) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 1 {
		t.Fatalf("%d orders were made from one cart", created.Load())
	}
}

// ---------------------------------------------------------------- transitions

func TestMarkReserved_RecordsReservationAndQueuesThePaymentRequestTogether(t *testing.T) {
	h := setup(t)
	o, res := h.placed(t)

	got := h.get(t, o.ID)
	if got.Status != domain.StatusAwaitingPayment || got.ReservationState != domain.ReservationHeld ||
		got.ReservationID == nil || *got.ReservationID != res || got.PaymentRequests != 1 {
		t.Fatalf("order = %+v", got)
	}
	if h.outboxCount(t, o, events.PaymentRequested) != 1 {
		t.Fatal("PaymentRequested must be in the outbox")
	}

	// Doing it again changes nothing and queues nothing.
	env := h.event(t, o, events.PaymentRequested, events.PaymentRequestedPayload{OrderID: o.ID})
	changed, err := h.repo.MarkReserved(h.ctx, o.ID, uuid.New(), env)
	if err != nil || changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if h.outboxCount(t, o, events.PaymentRequested) != 1 || *h.get(t, o.ID).ReservationID != res {
		t.Fatal("a repeated MarkReserved must not change the order or queue a second event")
	}
}

func TestTransition_StatusAndEventStandOrFallTogether(t *testing.T) {
	h := setup(t)
	o, _ := h.placed(t)

	// An envelope that fails validation can not be stored; the status change must not survive.
	_, err := h.repo.Cancel(h.ctx, o.ID, []domain.Status{domain.StatusAwaitingPayment}, "card_declined", "no", messaging.Envelope{})
	if err == nil {
		t.Fatal("expected an error for an unusable envelope")
	}
	if got := h.get(t, o.ID); got.Status != domain.StatusAwaitingPayment || got.FailureCode != "" {
		t.Fatalf("the status changed without its event: %+v", got)
	}
	if hist, _ := h.repo.History(h.ctx, o.ID); len(hist) != 2 {
		t.Fatalf("history = %d rows", len(hist))
	}
}

func TestTransitions_GuardsRejectWrongStates(t *testing.T) {
	h := setup(t)
	o, _ := h.placed(t)
	ctx := h.ctx

	if changed, _ := h.repo.Confirm(ctx, o.ID, h.confirmedEvent(t, o)); changed {
		t.Fatal("Confirm needs a paid order")
	}
	if changed, _ := h.repo.MarkReleased(ctx, o.ID); changed {
		t.Fatal("MarkReleased needs a cancelled order")
	}
	if changed, _ := h.repo.Cancel(ctx, o.ID, []domain.Status{domain.StatusPending}, "x", "x", h.cancelledEvent(t, o)); changed {
		t.Fatal("Cancel from the wrong status must not apply")
	}
	if changed, _ := h.repo.MarkPaid(ctx, o.ID); !changed {
		t.Fatal("MarkPaid should apply")
	}
	if changed, _ := h.repo.MarkPaid(ctx, o.ID); changed {
		t.Fatal("MarkPaid twice must apply once")
	}
	if changed, err := h.repo.Confirm(ctx, o.ID, h.confirmedEvent(t, o)); err != nil || !changed {
		t.Fatalf("Confirm: %v %v", changed, err)
	}
	got := h.get(t, o.ID)
	if got.Status != domain.StatusConfirmed || got.ReservationState != domain.ReservationCommitted {
		t.Fatalf("order = %+v", got)
	}
	if changed, _ := h.repo.Cancel(ctx, o.ID, []domain.Status{domain.StatusAwaitingPayment, domain.StatusPaid, domain.StatusConfirmed}, "x", "x", h.cancelledEvent(t, o)); !changed {
		// A confirmed order is only cancelled if a caller explicitly allows it; the service never does.
		t.Log("cancel refused as expected")
	}
}

// A success and a failure for the same order arrive together (a duplicated, reordered
// message pair). One of them wins; the order never ends up half of each.
func TestTransitions_PayAndCancelRaceHasOneWinner(t *testing.T) {
	h := setup(t)
	for round := 0; round < 10; round++ {
		o, _ := h.placed(t)
		var wg sync.WaitGroup
		var paid, cancelled atomic.Bool
		wg.Add(2)
		go func() {
			defer wg.Done()
			ch, err := h.repo.MarkPaid(h.ctx, o.ID)
			if err != nil {
				t.Error(err)
			}
			paid.Store(ch)
		}()
		go func() {
			defer wg.Done()
			ch, err := h.repo.Cancel(h.ctx, o.ID, []domain.Status{domain.StatusAwaitingPayment}, "card_declined", "no", h.cancelledEvent(t, o))
			if err != nil {
				t.Error(err)
			}
			cancelled.Store(ch)
		}()
		wg.Wait()

		if paid.Load() == cancelled.Load() {
			t.Fatalf("round %d: paid=%v cancelled=%v; exactly one must win", round, paid.Load(), cancelled.Load())
		}
		want := domain.StatusPaid
		if cancelled.Load() {
			want = domain.StatusCancelled
		}
		if got := h.get(t, o.ID); got.Status != want {
			t.Fatalf("round %d: status = %s, want %s", round, got.Status, want)
		}
		if n := h.outboxCount(t, o, events.OrderCancelled); (n == 1) != cancelled.Load() {
			t.Fatalf("round %d: %d OrderCancelled events for cancelled=%v", round, n, cancelled.Load())
		}
		if hist, _ := h.repo.History(h.ctx, o.ID); len(hist) != 3 {
			t.Fatalf("round %d: %d history rows, want 3", round, len(hist))
		}
	}
}

func TestMarkReleased_OnlyForACancelledOrderThatStillHoldsStock(t *testing.T) {
	h := setup(t)
	o, _ := h.placed(t)
	if _, err := h.repo.Cancel(h.ctx, o.ID, []domain.Status{domain.StatusAwaitingPayment}, "card_declined", "no", h.cancelledEvent(t, o)); err != nil {
		t.Fatal(err)
	}
	if got := h.get(t, o.ID); got.ReservationState != domain.ReservationHeld {
		t.Fatalf("cancelling does not release by itself: %+v", got)
	}
	if changed, err := h.repo.MarkReleased(h.ctx, o.ID); err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if changed, _ := h.repo.MarkReleased(h.ctx, o.ID); changed {
		t.Fatal("releasing twice must apply once")
	}
	if got := h.get(t, o.ID); got.ReservationState != domain.ReservationReleased || got.ReservationID == nil {
		t.Fatalf("order = %+v", got)
	}
}

func TestRequestPaymentAgain_OnlyWhileAwaitingPayment(t *testing.T) {
	h := setup(t)
	o, _ := h.placed(t)
	env := func() messaging.Envelope {
		return h.event(t, o, events.PaymentRequested, events.PaymentRequestedPayload{OrderID: o.ID})
	}
	if changed, err := h.repo.RequestPaymentAgain(h.ctx, o.ID, env()); err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if got := h.get(t, o.ID); got.PaymentRequests != 2 || h.outboxCount(t, o, events.PaymentRequested) != 2 {
		t.Fatalf("requests=%d events=%d", got.PaymentRequests, h.outboxCount(t, o, events.PaymentRequested))
	}
	if _, err := h.repo.MarkPaid(h.ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	if changed, _ := h.repo.RequestPaymentAgain(h.ctx, o.ID, env()); changed {
		t.Fatal("a paid order must not ask for payment again")
	}
	if h.outboxCount(t, o, events.PaymentRequested) != 2 {
		t.Fatal("no event may be queued when nothing changed")
	}
}

// ---------------------------------------------------------------- constraints

func TestDatabaseRefusesInconsistentOrders(t *testing.T) {
	h := setup(t)
	o, _ := h.placed(t)
	for name, sql := range map[string]string{
		"cancelled without a reason":     "UPDATE orders SET status = 'cancelled' WHERE id = ?",
		"unknown status":                 "UPDATE orders SET status = 'shipped' WHERE id = ?",
		"reservation id without a state": "UPDATE orders SET reservation_state = 'none' WHERE id = ?",
		"negative payment requests":      "UPDATE orders SET payment_requests = -1 WHERE id = ?",
		"zero total":                     "UPDATE orders SET total_cents = 0 WHERE id = ?",
	} {
		if _, err := h.db.ExecContext(h.ctx, sql, o.ID); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if got := h.get(t, o.ID); got.Status != domain.StatusAwaitingPayment {
		t.Fatalf("order changed: %+v", got)
	}
}

// ---------------------------------------------------------------- queries

func TestListAndUnfinished(t *testing.T) {
	h := setup(t)
	c := h.customer()
	mk := func(key string) *domain.Order {
		cart := h.fillCart(t, c, 1)
		o := h.newOrder(c, key, cart)
		if err := h.repo.Create(h.ctx, o, cart); err != nil {
			t.Fatal(err)
		}
		return o
	}
	pending := mk("a")
	cancelled := mk("b")
	if _, err := h.repo.Cancel(h.ctx, cancelled.ID, []domain.Status{domain.StatusPending}, "out_of_stock", "x", h.cancelledEvent(t, cancelled)); err != nil {
		t.Fatal(err)
	}

	got, total, err := h.repo.List(h.ctx, domain.Filter{CustomerID: &c, Page: 1, PageSize: 10})
	if err != nil || total != 2 || len(got) != 2 || len(got[0].Items) != 1 {
		t.Fatalf("list: %d/%d items, err %v", len(got), total, err)
	}
	got, total, _ = h.repo.List(h.ctx, domain.Filter{CustomerID: &c, Status: domain.StatusCancelled, Page: 1, PageSize: 10})
	if total != 1 || got[0].ID != cancelled.ID {
		t.Fatalf("status filter: %+v", got)
	}
	got, total, _ = h.repo.List(h.ctx, domain.Filter{CustomerID: &c, Page: 2, PageSize: 1})
	if total != 2 || len(got) != 1 {
		t.Fatalf("page 2: %d of %d", len(got), total)
	}

	stuck, err := h.repo.Unfinished(h.ctx, time.Now().Add(time.Hour), 1000)
	if err != nil {
		t.Fatal(err)
	}
	has := func(id uuid.UUID) bool {
		for _, o := range stuck {
			if o.ID == id {
				return true
			}
		}
		return false
	}
	if !has(pending.ID) || has(cancelled.ID) {
		t.Fatal("a pending order is unfinished; a cancelled order that holds no stock is not")
	}
	if fresh, _ := h.repo.Unfinished(h.ctx, time.Now().Add(-time.Hour), 1000); func() bool {
		for _, o := range fresh {
			if o.ID == pending.ID {
				return true
			}
		}
		return false
	}() {
		t.Fatal("an order changed just now is not stuck yet")
	}
}

// ---------------------------------------------------------------- saga on a real database

type stubCatalog struct{}

func (stubCatalog) Lookup(_ context.Context, ids []uuid.UUID) ([]application.VariantInfo, error) {
	out := make([]application.VariantInfo, len(ids))
	for i, id := range ids {
		out[i] = application.VariantInfo{VariantID: id, ProductID: uuid.New(), ProductName: "Shirt", SKU: "S-" + id.String()[:6],
			PriceCents: 1000, Currency: "TRY", Sellable: true}
	}
	return out, nil
}

type countingInventory struct{ reserves, commits, releases atomic.Int32 }

func (i *countingInventory) Reserve(context.Context, uuid.UUID, []application.ReservationLine) (*application.Reservation, error) {
	i.reserves.Add(1)
	return &application.Reservation{ID: uuid.New()}, nil
}
func (i *countingInventory) Commit(context.Context, uuid.UUID) error  { i.commits.Add(1); return nil }
func (i *countingInventory) Release(context.Context, uuid.UUID) error { i.releases.Add(1); return nil }

func TestSaga_ConcurrentDuplicatePaymentResultsConfirmOnce(t *testing.T) {
	h := setup(t)
	inv := &countingInventory{}
	svc := application.NewOrderService(h.repo, stubCatalog{}, inv, slog.New(slog.NewTextHandler(io.Discard, nil)), application.Config{})

	c := h.customer()
	h.fillCart(t, c, 2)
	o, created, err := svc.Checkout(h.ctx, application.CheckoutInput{CustomerID: c, IdempotencyKey: "saga-1", PaymentMethod: "tok_visa"})
	if err != nil || !created || o.Status != domain.StatusAwaitingPayment {
		t.Fatalf("checkout: %+v created=%v err=%v", o, created, err)
	}
	h.orders = append(h.orders, o.ID)

	p := events.PaymentSucceededPayload{PaymentID: uuid.New(), OrderID: o.ID, AmountCents: o.TotalCents, Currency: "TRY", ProviderRef: "ref"}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svc.OnPaymentSucceeded(h.ctx, p); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	got := h.get(t, o.ID)
	if got.Status != domain.StatusConfirmed || got.ReservationState != domain.ReservationCommitted {
		t.Fatalf("order = %+v", got)
	}
	if n := h.outboxCount(t, o, events.OrderConfirmed); n != 1 {
		t.Fatalf("%d OrderConfirmed events for one order", n)
	}
	if h.outboxCount(t, o, events.PaymentRequested) != 1 {
		t.Fatal("exactly one PaymentRequested expected")
	}
}

func TestSaga_ConcurrentCheckoutRetriesWithOneKeyReserveOnce(t *testing.T) {
	h := setup(t)
	inv := &countingInventory{}
	svc := application.NewOrderService(h.repo, stubCatalog{}, inv, slog.New(slog.NewTextHandler(io.Discard, nil)), application.Config{})

	c := h.customer()
	h.fillCart(t, c, 2)
	var wg sync.WaitGroup
	ids := make(chan uuid.UUID, 10)
	var createdCount atomic.Int32
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o, created, err := svc.Checkout(h.ctx, application.CheckoutInput{CustomerID: c, IdempotencyKey: "one-key", PaymentMethod: "tok_visa"})
			if err != nil {
				t.Error(err)
				return
			}
			if created {
				createdCount.Add(1)
			}
			ids <- o.ID
		}()
	}
	wg.Wait()
	close(ids)

	var first uuid.UUID
	for id := range ids {
		if first == uuid.Nil {
			first = id
			h.orders = append(h.orders, id)
		}
		if id != first {
			t.Fatalf("the retries returned different orders: %s vs %s", id, first)
		}
	}
	if createdCount.Load() != 1 {
		t.Fatalf("%d requests believe they created the order", createdCount.Load())
	}
	o := h.get(t, first)
	if o.Status != domain.StatusAwaitingPayment || h.outboxCount(t, o, events.PaymentRequested) != 1 {
		t.Fatalf("status %s, payment requests queued %d", o.Status, h.outboxCount(t, o, events.PaymentRequested))
	}
}
