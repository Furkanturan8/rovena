package application

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/services/order/internal/repository/memory"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/google/uuid"
)

type harness struct {
	t        *testing.T
	clk      *clock
	repo     *memory.Repo
	catalog  *fakeCatalog
	inv      *fakeInventory
	svc      *OrderService
	carts    *CartService
	logs     *bytes.Buffer
	customer uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, clk: newClock(), catalog: newFakeCatalog(), inv: newFakeInventory(), logs: &bytes.Buffer{}, customer: uuid.New()}
	h.repo = memory.New(h.clk.Now)
	log := slog.New(slog.NewTextHandler(h.logs, nil))
	h.svc = NewOrderService(h.repo, h.catalog, h.inv, log, Config{
		StuckAfter: 30 * time.Second, PendingMaxAge: 5 * time.Minute, PaymentTimeout: 2 * time.Minute, MaxPaymentRequests: 3,
	})
	h.svc.now = h.clk.Now
	h.carts = NewCartService(h.repo, h.catalog)
	return h
}

func (h *harness) put(variant uuid.UUID, qty int) {
	h.t.Helper()
	if err := h.repo.SetCartItem(context.Background(), h.customer, domain.CartItem{VariantID: variant, Quantity: qty}); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) checkout(key string) (*domain.Order, bool, error) {
	return h.svc.Checkout(context.Background(), CheckoutInput{CustomerID: h.customer, IdempotencyKey: key, PaymentMethod: "tok_visa"})
}

// placed returns an order that reached awaiting_payment.
func (h *harness) placed() *domain.Order {
	h.t.Helper()
	v := h.catalog.add("Linen Shirt", 1500)
	h.put(v, 2)
	o, _, err := h.checkout("key-" + uuid.NewString())
	if err != nil {
		h.t.Fatal(err)
	}
	if o.Status != domain.StatusAwaitingPayment {
		h.t.Fatalf("order is %s, want awaiting_payment", o.Status)
	}
	return o
}

func (h *harness) order(id uuid.UUID) *domain.Order {
	h.t.Helper()
	o, err := h.repo.GetByID(context.Background(), id)
	if err != nil {
		h.t.Fatal(err)
	}
	return o
}

func succeeded(o *domain.Order) events.PaymentSucceededPayload {
	return events.PaymentSucceededPayload{PaymentID: uuid.New(), OrderID: o.ID, AmountCents: o.TotalCents, Currency: o.Currency, ProviderRef: "ref"}
}

func failed(o *domain.Order, code string) events.PaymentFailedPayload {
	return events.PaymentFailedPayload{PaymentID: uuid.New(), OrderID: o.ID, AmountCents: o.TotalCents, Currency: o.Currency, FailureCode: code, FailureMessage: "no"}
}

// ---------------------------------------------------------------- cart

func TestCart_PricesComeFromCatalogNotFromTheClient(t *testing.T) {
	h := newHarness(t)
	v := h.catalog.add("Linen Shirt", 1500)
	view, err := h.carts.SetItem(context.Background(), h.customer, v, 3)
	if err != nil {
		t.Fatal(err)
	}
	if view.TotalCents != 4500 || view.Lines[0].UnitPriceCents != 1500 || view.Lines[0].ProductName != "Linen Shirt" {
		t.Fatalf("view = %+v", view)
	}

	h.catalog.update(v, func(i *VariantInfo) { i.PriceCents = 2000 })
	view, _ = h.carts.Get(context.Background(), h.customer)
	if view.TotalCents != 6000 {
		t.Fatalf("a price change must show in the cart, total = %d", view.TotalCents)
	}
}

func TestCart_RejectsBadQuantityAndUnsellableVariants(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	v := h.catalog.add("Shirt", 100)
	for _, q := range []int{0, -1, domain.MaxLineQuantity + 1} {
		if _, err := h.carts.SetItem(ctx, h.customer, v, q); !errors.Is(err, domain.ErrInvalidQuantity) {
			t.Errorf("quantity %d: err = %v", q, err)
		}
	}
	if _, err := h.carts.SetItem(ctx, h.customer, uuid.New(), 1); !errors.Is(err, domain.ErrItemUnavailable) {
		t.Errorf("unknown variant: err = %v", err)
	}
	h.catalog.update(v, func(i *VariantInfo) { i.Sellable = false })
	var ie *domain.ItemError
	if _, err := h.carts.SetItem(ctx, h.customer, v, 1); !errors.As(err, &ie) || ie.VariantID != v {
		t.Errorf("unsellable variant: err = %v", err)
	}
}

func TestCart_UnavailableLinesAreShownButNotCounted(t *testing.T) {
	h := newHarness(t)
	a, b := h.catalog.add("A", 1000), h.catalog.add("B", 500)
	h.put(a, 1)
	h.put(b, 2)
	h.catalog.update(b, func(i *VariantInfo) { i.Sellable = false })
	view, err := h.carts.Get(context.Background(), h.customer)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Lines) != 2 || view.TotalCents != 1000 {
		t.Fatalf("view = %+v", view)
	}
}

func TestCart_CatalogOutageIsReported(t *testing.T) {
	h := newHarness(t)
	h.put(h.catalog.add("A", 1), 1)
	h.catalog.err = domain.ErrDependencyUnavailable
	if _, err := h.carts.Get(context.Background(), h.customer); !errors.Is(err, domain.ErrDependencyUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

// ---------------------------------------------------------------- checkout

func TestCheckout_HappyPathStartsTheSaga(t *testing.T) {
	h := newHarness(t)
	shirt, hat := h.catalog.add("Linen Shirt", 1500), h.catalog.add("Hat", 700)
	h.put(shirt, 2)
	h.put(hat, 1)

	o, created, err := h.checkout("key-1")
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if o.Status != domain.StatusAwaitingPayment || o.ReservationState != domain.ReservationHeld || o.ReservationID == nil {
		t.Fatalf("order = %+v", o)
	}
	if o.TotalCents != 3700 || o.Currency != "TRY" || len(o.Items) != 2 {
		t.Fatalf("totals = %d %s, items %d", o.TotalCents, o.Currency, len(o.Items))
	}
	if got := h.repo.Events(); len(got) != 1 || got[0] != events.PaymentRequested {
		t.Fatalf("events = %v", got)
	}

	var p events.PaymentRequestedPayload
	if err := h.repo.Outbox[0].DecodePayload(&p); err != nil {
		t.Fatal(err)
	}
	if p.OrderID != o.ID || p.CustomerID != h.customer || p.AmountCents != 3700 || p.PaymentMethod != "tok_visa" ||
		p.IdempotencyKey != "order:"+o.ID.String() {
		t.Fatalf("payload = %+v", p)
	}
	if h.repo.Outbox[0].CorrelationID != o.ID.String() {
		t.Fatalf("correlation id = %q, want the order id", h.repo.Outbox[0].CorrelationID)
	}
	if cart, _ := h.repo.GetCart(context.Background(), h.customer); len(cart) != 0 {
		t.Fatal("the cart must be emptied by checkout")
	}
}

func TestCheckout_TheOrderKeepsThePricesOfTheMoment(t *testing.T) {
	h := newHarness(t)
	v := h.catalog.add("Shirt", 1000)
	h.put(v, 1)
	o, _, err := h.checkout("k")
	if err != nil {
		t.Fatal(err)
	}
	h.catalog.update(v, func(i *VariantInfo) { i.PriceCents = 9999; i.ProductName = "Renamed" })

	got := h.order(o.ID)
	if got.TotalCents != 1000 || got.Items[0].UnitPriceCents != 1000 || got.Items[0].ProductName != "Shirt" {
		t.Fatalf("snapshot changed: %+v", got)
	}
}

func TestCheckout_RepeatedKeyReturnsTheSameOrder(t *testing.T) {
	h := newHarness(t)
	h.put(h.catalog.add("Shirt", 1000), 1)
	first, created, err := h.checkout("same-key")
	if err != nil || !created {
		t.Fatal(created, err)
	}
	second, created, err := h.checkout("same-key")
	if err != nil || created {
		t.Fatalf("second: created=%v err=%v", created, err)
	}
	if second.ID != first.ID {
		t.Fatal("a repeated key must return the first order")
	}
	if h.inv.reserveCalls != 1 || h.repo.CountEvents(events.PaymentRequested) != 1 || len(h.repo.Orders) != 1 {
		t.Fatalf("reserve calls %d, payment requests %d, orders %d",
			h.inv.reserveCalls, h.repo.CountEvents(events.PaymentRequested), len(h.repo.Orders))
	}
}

func TestCheckout_SameKeyWithAnotherPaymentMethodIsRefused(t *testing.T) {
	h := newHarness(t)
	h.put(h.catalog.add("Shirt", 1000), 1)
	if _, _, err := h.checkout("k"); err != nil {
		t.Fatal(err)
	}
	_, _, err := h.svc.Checkout(context.Background(), CheckoutInput{CustomerID: h.customer, IdempotencyKey: "k", PaymentMethod: "tok_other"})
	if !errors.Is(err, domain.ErrIdempotencyKeyReused) {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckout_KeysAreScopedPerCustomer(t *testing.T) {
	h := newHarness(t)
	h.put(h.catalog.add("Shirt", 1000), 1)
	mine, _, err := h.checkout("shared-key")
	if err != nil {
		t.Fatal(err)
	}

	other := newHarness(t)
	other.repo, other.svc.repo = h.repo, h.repo
	other.svc.catalog, other.svc.inventory = h.catalog, h.inv
	other.put(h.catalog.add("Hat", 300), 1)
	theirs, created, err := other.checkout("shared-key")
	if err != nil || !created || theirs.ID == mine.ID {
		t.Fatalf("another customer's identical key must make a new order: created=%v err=%v", created, err)
	}
}

func TestCheckout_ValidatesKeyAndPaymentMethod(t *testing.T) {
	h := newHarness(t)
	h.put(h.catalog.add("Shirt", 1000), 1)
	for name, in := range map[string]CheckoutInput{
		"empty key":      {IdempotencyKey: "", PaymentMethod: "tok_visa"},
		"long key":       {IdempotencyKey: strings.Repeat("k", 65), PaymentMethod: "tok_visa"},
		"key with space": {IdempotencyKey: "a b", PaymentMethod: "tok_visa"},
		"no method":      {IdempotencyKey: "k", PaymentMethod: ""},
		"long method":    {IdempotencyKey: "k", PaymentMethod: strings.Repeat("m", 129)},
	} {
		in.CustomerID = h.customer
		if _, _, err := h.svc.Checkout(context.Background(), in); !errors.Is(err, domain.ErrInvalidRequest) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if len(h.repo.Orders) != 0 {
		t.Fatal("a rejected request must not create an order")
	}
}

func TestCheckout_EmptyCart(t *testing.T) {
	h := newHarness(t)
	if _, _, err := h.checkout("k"); !errors.Is(err, domain.ErrCartEmpty) {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckout_UnavailableItemIsNamedAndNothingIsCreated(t *testing.T) {
	h := newHarness(t)
	ok, gone := h.catalog.add("A", 100), h.catalog.add("B", 200)
	h.put(ok, 1)
	h.put(gone, 1)
	h.catalog.update(gone, func(i *VariantInfo) { i.Sellable = false })

	_, _, err := h.checkout("k")
	var ie *domain.ItemError
	if !errors.As(err, &ie) || ie.VariantID != gone {
		t.Fatalf("err = %v", err)
	}
	if len(h.repo.Orders) != 0 || h.inv.reserveCalls != 0 {
		t.Fatal("no order and no reservation may exist")
	}
	if cart, _ := h.repo.GetCart(context.Background(), h.customer); len(cart) != 2 {
		t.Fatal("the cart must survive a refused checkout")
	}
}

func TestCheckout_MixedCurrenciesAreRefused(t *testing.T) {
	h := newHarness(t)
	a, b := h.catalog.add("A", 100), h.catalog.add("B", 200)
	h.catalog.update(b, func(i *VariantInfo) { i.Currency = "USD" })
	h.put(a, 1)
	h.put(b, 1)
	if _, _, err := h.checkout("k"); !errors.Is(err, domain.ErrMixedCurrencies) {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckout_OutOfStockCancelsTheOrderAndSendsNoPayment(t *testing.T) {
	h := newHarness(t)
	v := h.catalog.add("Shirt", 1000)
	h.put(v, 5)
	h.inv.shortage = &StockShortage{VariantID: v, Requested: 5, Available: 1}

	o, created, err := h.checkout("k")
	if err != nil || !created {
		t.Fatal(created, err)
	}
	if o.Status != domain.StatusCancelled || o.FailureCode != domain.CodeOutOfStock || !strings.Contains(o.FailureMessage, v.String()) {
		t.Fatalf("order = %+v", o)
	}
	if h.repo.CountEvents(events.PaymentRequested) != 0 || h.repo.CountEvents(events.OrderCancelled) != 1 {
		t.Fatalf("events = %v", h.repo.Events())
	}
	if o.ReservationState != domain.ReservationNone || h.inv.releaseCalls != 0 {
		t.Fatal("nothing was reserved, so nothing may be released")
	}
}

func TestCheckout_UntrackedVariantIsReportedAsUnavailable(t *testing.T) {
	h := newHarness(t)
	v := h.catalog.add("Shirt", 1000)
	h.put(v, 1)
	h.inv.shortage = &StockShortage{VariantID: v, NotTracked: true}
	o, _, err := h.checkout("k")
	if err != nil || o.Status != domain.StatusCancelled || o.FailureCode != domain.CodeItemUnavailable {
		t.Fatalf("order = %+v, err = %v", o, err)
	}
}

func TestCheckout_InventoryOutageLeavesThePendingOrderToRecovery(t *testing.T) {
	h := newHarness(t)
	h.put(h.catalog.add("Shirt", 1000), 1)
	h.inv.reserveErr = errBoom

	o, created, err := h.checkout("k")
	if err != nil || !created {
		t.Fatalf("the customer's request must not fail: created=%v err=%v", created, err)
	}
	if o.Status != domain.StatusPending || len(h.repo.Events()) != 0 {
		t.Fatalf("status %s, events %v", o.Status, h.repo.Events())
	}

	// Inventory comes back; the recovery job picks the order up.
	h.inv.reserveErr = nil
	h.clk.Advance(time.Minute)
	if n, err := h.svc.Recover(context.Background()); err != nil || n != 1 {
		t.Fatalf("recover: %d, %v", n, err)
	}
	if got := h.order(o.ID); got.Status != domain.StatusAwaitingPayment {
		t.Fatalf("status = %s", got.Status)
	}
	if h.repo.CountEvents(events.PaymentRequested) != 1 {
		t.Fatalf("events = %v", h.repo.Events())
	}
}

func TestCheckout_RetryOfAPendingOrderFinishesIt(t *testing.T) {
	h := newHarness(t)
	h.put(h.catalog.add("Shirt", 1000), 1)
	h.inv.reserveErr = errBoom
	first, _, _ := h.checkout("k")
	if first.Status != domain.StatusPending {
		t.Fatal("setup")
	}
	h.inv.reserveErr = nil

	again, created, err := h.checkout("k")
	if err != nil || created || again.ID != first.ID || again.Status != domain.StatusAwaitingPayment {
		t.Fatalf("again = %+v created=%v err=%v", again, created, err)
	}
}

func TestCheckout_ReservationRecordingFailureIsRepairedByReserveBeingIdempotent(t *testing.T) {
	h := newHarness(t)
	h.put(h.catalog.add("Shirt", 1000), 1)
	h.repo.FailNext["MarkReserved"] = errBoom

	o, _, err := h.checkout("k")
	if err != nil || o.Status != domain.StatusPending {
		t.Fatalf("o = %+v err = %v", o, err)
	}
	h.clk.Advance(time.Minute)
	if _, err := h.svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := h.order(o.ID); got.Status != domain.StatusAwaitingPayment || h.inv.reserveCalls != 2 || len(h.inv.reservations) != 1 {
		t.Fatalf("status %s, reserve calls %d, reservations %d", got.Status, h.inv.reserveCalls, len(h.inv.reservations))
	}
}

func TestCheckout_CartChangedDuringCheckoutIsRefused(t *testing.T) {
	h := newHarness(t)
	v := h.catalog.add("Shirt", 1000)
	h.put(v, 1)
	h.catalog.onLookup = func() { // the customer edits the cart in another tab
		h.catalog.onLookup = nil
		h.put(v, 4)
	}
	if _, _, err := h.checkout("k"); !errors.Is(err, domain.ErrCartChanged) {
		t.Fatalf("err = %v", err)
	}
	if len(h.repo.Orders) != 0 {
		t.Fatal("an order priced from a stale cart must not be created")
	}
}

func TestCheckout_RaceOnTheSameKeyGivesOneOrder(t *testing.T) {
	h := newHarness(t)
	v := h.catalog.add("Shirt", 1000)
	h.put(v, 1)
	// The first request creates the order while the second one is still pricing the cart.
	first := (*domain.Order)(nil)
	h.catalog.onLookup = func() {
		h.catalog.onLookup = nil
		first, _, _ = h.checkout("k")
	}
	second, created, err := h.checkout("k")
	if err != nil {
		t.Fatal(err)
	}
	if created || first == nil || second.ID != first.ID || len(h.repo.Orders) != 1 {
		t.Fatalf("created=%v orders=%d", created, len(h.repo.Orders))
	}
}

func TestCheckout_ReservationMadeForAnOrderCancelledInTheMeantimeIsReleased(t *testing.T) {
	h := newHarness(t)
	h.put(h.catalog.add("Shirt", 1000), 1)
	h.inv.onReserve = func(orderID uuid.UUID) { // recovery gives up on the order during the call
		o := h.order(orderID)
		h.svc.cancel(context.Background(), o, []domain.Status{domain.StatusPending}, domain.CodeCheckoutTimeout, "late")
	}
	o, _, err := h.checkout("k")
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != domain.StatusCancelled {
		t.Fatalf("status = %s", o.Status)
	}
	if h.inv.releaseCalls != 1 {
		t.Fatalf("the orphaned reservation must be released, release calls = %d", h.inv.releaseCalls)
	}
	if h.repo.CountEvents(events.PaymentRequested) != 0 {
		t.Fatal("no payment may be requested for a cancelled order")
	}
}

// ---------------------------------------------------------------- saga

func TestPaymentSucceeded_CommitsStockAndConfirms(t *testing.T) {
	h := newHarness(t)
	o := h.placed()

	if err := h.svc.OnPaymentSucceeded(context.Background(), succeeded(o)); err != nil {
		t.Fatal(err)
	}
	got := h.order(o.ID)
	if got.Status != domain.StatusConfirmed || got.ReservationState != domain.ReservationCommitted {
		t.Fatalf("order = %+v", got)
	}
	if h.inv.commitCalls != 1 || h.repo.CountEvents(events.OrderConfirmed) != 1 {
		t.Fatalf("commits %d, events %v", h.inv.commitCalls, h.repo.Events())
	}
}

func TestPaymentSucceeded_DeliveredTwiceChangesNothingTheSecondTime(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	p := succeeded(o)
	for i := 0; i < 3; i++ {
		if err := h.svc.OnPaymentSucceeded(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	if h.inv.commitCalls != 1 || h.repo.CountEvents(events.OrderConfirmed) != 1 {
		t.Fatalf("commits %d, confirmed events %d", h.inv.commitCalls, h.repo.CountEvents(events.OrderConfirmed))
	}
}

func TestPaymentSucceeded_CommitOutageKeepsTheOrderPaidUntilRetried(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	h.inv.commitErr = errBoom

	if err := h.svc.OnPaymentSucceeded(context.Background(), succeeded(o)); !errors.Is(err, errBoom) {
		t.Fatalf("the handler must report the failure so the message is retried, got %v", err)
	}
	if got := h.order(o.ID); got.Status != domain.StatusPaid || got.ReservationState != domain.ReservationHeld {
		t.Fatalf("order = %+v", got)
	}

	// The redelivery finds the order already paid and only repeats the commit.
	h.inv.commitErr = nil
	if err := h.svc.OnPaymentSucceeded(context.Background(), succeeded(o)); err != nil {
		t.Fatal(err)
	}
	if got := h.order(o.ID); got.Status != domain.StatusConfirmed {
		t.Fatalf("status = %s", got.Status)
	}
}

func TestPaymentSucceeded_ReservationExpiredBeforePaymentCancelsAndFlagsRefund(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	h.inv.commitErr = ErrReservationGone

	if err := h.svc.OnPaymentSucceeded(context.Background(), succeeded(o)); err != nil {
		t.Fatal(err)
	}
	got := h.order(o.ID)
	if got.Status != domain.StatusCancelled || got.FailureCode != domain.CodeReservationLost || got.ReservationState != domain.ReservationReleased {
		t.Fatalf("order = %+v", got)
	}
	if !strings.Contains(h.logs.String(), "REFUND NEEDED") {
		t.Fatal("money was taken for an order that can not be fulfilled; that must be logged for follow-up")
	}
}

func TestPaymentSucceeded_WrongAmountIsNotConfirmed(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	for name, mutate := range map[string]func(*events.PaymentSucceededPayload){
		"less":     func(p *events.PaymentSucceededPayload) { p.AmountCents-- },
		"more":     func(p *events.PaymentSucceededPayload) { p.AmountCents++ },
		"currency": func(p *events.PaymentSucceededPayload) { p.Currency = "USD" },
	} {
		p := succeeded(o)
		mutate(&p)
		if err := h.svc.OnPaymentSucceeded(context.Background(), p); !errors.Is(err, ErrUnexpectedEvent) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if got := h.order(o.ID); got.Status != domain.StatusAwaitingPayment || h.inv.commitCalls != 0 {
		t.Fatalf("status = %s, commits = %d", got.Status, h.inv.commitCalls)
	}
	if !strings.Contains(h.logs.String(), "PAYMENT AMOUNT MISMATCH") {
		t.Fatal("a mismatch must be logged")
	}
}

func TestPaymentSucceeded_UnknownOrderIsPermanent(t *testing.T) {
	h := newHarness(t)
	err := h.svc.OnPaymentSucceeded(context.Background(), events.PaymentSucceededPayload{OrderID: uuid.New(), AmountCents: 1, Currency: "TRY"})
	if !errors.Is(err, ErrUnexpectedEvent) {
		t.Fatalf("err = %v", err)
	}
}

func TestPaymentSucceeded_ForACancelledOrderFlagsRefundAndChangesNothing(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	if err := h.svc.OnPaymentFailed(context.Background(), failed(o, "payment_timeout")); err != nil {
		t.Fatal(err)
	}

	if err := h.svc.OnPaymentSucceeded(context.Background(), succeeded(o)); err != nil {
		t.Fatal(err)
	}
	if got := h.order(o.ID); got.Status != domain.StatusCancelled {
		t.Fatalf("a cancelled order must stay cancelled, status = %s", got.Status)
	}
	if !strings.Contains(h.logs.String(), "REFUND NEEDED") || h.inv.commitCalls != 0 {
		t.Fatal("a late success for a cancelled order needs a refund and must not commit stock")
	}
}

func TestPaymentFailed_CancelsAndReleasesStock(t *testing.T) {
	h := newHarness(t)
	o := h.placed()

	if err := h.svc.OnPaymentFailed(context.Background(), failed(o, "card_declined")); err != nil {
		t.Fatal(err)
	}
	got := h.order(o.ID)
	if got.Status != domain.StatusCancelled || got.FailureCode != "card_declined" || got.ReservationState != domain.ReservationReleased {
		t.Fatalf("order = %+v", got)
	}
	if h.inv.releaseCalls != 1 || h.repo.CountEvents(events.OrderCancelled) != 1 {
		t.Fatalf("releases %d, events %v", h.inv.releaseCalls, h.repo.Events())
	}
}

func TestPaymentFailed_DeliveredTwiceReleasesOnce(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	p := failed(o, "card_declined")
	for i := 0; i < 3; i++ {
		if err := h.svc.OnPaymentFailed(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	if h.inv.releaseCalls != 1 || h.repo.CountEvents(events.OrderCancelled) != 1 {
		t.Fatalf("releases %d, cancelled events %d", h.inv.releaseCalls, h.repo.CountEvents(events.OrderCancelled))
	}
}

func TestPaymentFailed_ReleaseOutageIsRetriedFromTheCancelledState(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	h.inv.releaseErr = errBoom

	if err := h.svc.OnPaymentFailed(context.Background(), failed(o, "card_declined")); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if got := h.order(o.ID); got.Status != domain.StatusCancelled || got.ReservationState != domain.ReservationHeld {
		t.Fatalf("order = %+v", got)
	}

	h.inv.releaseErr = nil
	if err := h.svc.OnPaymentFailed(context.Background(), failed(o, "card_declined")); err != nil {
		t.Fatal(err)
	}
	if got := h.order(o.ID); got.ReservationState != domain.ReservationReleased {
		t.Fatalf("reservation = %s", got.ReservationState)
	}
	if h.repo.CountEvents(events.OrderCancelled) != 1 {
		t.Fatal("the order is cancelled once, however often the release is retried")
	}
}

func TestPaymentFailed_NeverCancelsAnOrderThatIsPaidOrConfirmed(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	if err := h.svc.OnPaymentSucceeded(context.Background(), succeeded(o)); err != nil {
		t.Fatal(err)
	}
	// A second checkout attempt for the same order is refused by Payment with this code.
	for _, code := range []string{"order_already_paid", "card_declined"} {
		if err := h.svc.OnPaymentFailed(context.Background(), failed(o, code)); err != nil {
			t.Fatal(err)
		}
	}
	if got := h.order(o.ID); got.Status != domain.StatusConfirmed || h.inv.releaseCalls != 0 {
		t.Fatalf("status = %s, releases = %d", got.Status, h.inv.releaseCalls)
	}
}

func TestPaymentFailed_OrderAlreadyPaidDoesNotCancelAnAwaitingOrder(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	if err := h.svc.OnPaymentFailed(context.Background(), failed(o, "order_already_paid")); err != nil {
		t.Fatal(err)
	}
	if got := h.order(o.ID); got.Status != domain.StatusAwaitingPayment {
		t.Fatalf("status = %s; the real outcome is still on its way", got.Status)
	}
}

// ---------------------------------------------------------------- recovery

func TestRecovery_LeavesFreshAndHealthyOrdersAlone(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	h.clk.Advance(time.Minute) // stuck for the generic threshold, but still inside the payment timeout
	if n, _ := h.svc.Recover(context.Background()); n != 0 {
		t.Fatalf("handled %d orders", n)
	}
	if got := h.order(o.ID); got.PaymentRequests != 1 || h.repo.CountEvents(events.PaymentRequested) != 1 {
		t.Fatal("no new payment request may be sent before the timeout")
	}
}

func TestRecovery_AsksForPaymentAgainThenGivesUp(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	ctx := context.Background()

	for want := 2; want <= 3; want++ {
		h.clk.Advance(3 * time.Minute)
		if _, err := h.svc.Recover(ctx); err != nil {
			t.Fatal(err)
		}
		if got := h.order(o.ID); got.Status != domain.StatusAwaitingPayment || got.PaymentRequests != want {
			t.Fatalf("round %d: status %s, requests %d", want, got.Status, got.PaymentRequests)
		}
	}
	if h.repo.CountEvents(events.PaymentRequested) != 3 {
		t.Fatalf("events = %v", h.repo.Events())
	}

	h.clk.Advance(3 * time.Minute)
	if _, err := h.svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	got := h.order(o.ID)
	if got.Status != domain.StatusCancelled || got.FailureCode != domain.CodePaymentTimeout || got.ReservationState != domain.ReservationReleased {
		t.Fatalf("order = %+v", got)
	}
	if h.inv.releaseCalls != 1 {
		t.Fatalf("release calls = %d", h.inv.releaseCalls)
	}
}

func TestRecovery_RepeatedPaymentRequestsReuseTheSameIdempotencyKey(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	h.clk.Advance(3 * time.Minute)
	if _, err := h.svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	var a, b events.PaymentRequestedPayload
	_ = h.repo.Outbox[0].DecodePayload(&a)
	_ = h.repo.Outbox[1].DecodePayload(&b)
	if a.IdempotencyKey != b.IdempotencyKey || a.IdempotencyKey != "order:"+o.ID.String() {
		t.Fatalf("keys differ: %q vs %q; Payment would charge twice", a.IdempotencyKey, b.IdempotencyKey)
	}
	if h.repo.Outbox[0].ID == h.repo.Outbox[1].ID {
		t.Fatal("each request is its own event")
	}
}

func TestRecovery_FinishesAPaidOrderWhoseCommitFailed(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	h.inv.commitErr = errBoom
	_ = h.svc.OnPaymentSucceeded(context.Background(), succeeded(o))
	h.inv.commitErr = nil

	h.clk.Advance(time.Minute)
	if _, err := h.svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := h.order(o.ID); got.Status != domain.StatusConfirmed {
		t.Fatalf("status = %s", got.Status)
	}
}

func TestRecovery_FinishesACancelledOrderWhoseReleaseFailed(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	h.inv.releaseErr = errBoom
	_ = h.svc.OnPaymentFailed(context.Background(), failed(o, "card_declined"))
	h.inv.releaseErr = nil

	h.clk.Advance(time.Minute)
	if _, err := h.svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := h.order(o.ID); got.ReservationState != domain.ReservationReleased {
		t.Fatalf("reservation = %s", got.ReservationState)
	}
}

func TestRecovery_BacksOffAfterAFailedAttempt(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	h.inv.commitErr = errBoom
	_ = h.svc.OnPaymentSucceeded(context.Background(), succeeded(o))

	h.clk.Advance(time.Minute)
	_, _ = h.svc.Recover(context.Background()) // fails again, moves updated_at
	calls := h.inv.commitCalls
	_, _ = h.svc.Recover(context.Background()) // immediately after: must not hammer Inventory
	if h.inv.commitCalls != calls {
		t.Fatalf("commit retried right away: %d -> %d", calls, h.inv.commitCalls)
	}
}

func TestRecovery_CancelsAPendingOrderThatNeverGotItsStock(t *testing.T) {
	h := newHarness(t)
	h.put(h.catalog.add("Shirt", 1000), 1)
	h.inv.reserveErr = errBoom
	o, _, _ := h.checkout("k")

	h.clk.Advance(6 * time.Minute)
	if _, err := h.svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := h.order(o.ID)
	if got.Status != domain.StatusCancelled || got.FailureCode != domain.CodeCheckoutTimeout {
		t.Fatalf("order = %+v", got)
	}
}

// ---------------------------------------------------------------- queries

func TestQuery_AnotherCustomersOrderIsNotFound(t *testing.T) {
	h := newHarness(t)
	o := h.placed()
	q := NewQueryService(h.repo)

	if _, err := q.GetForCustomer(context.Background(), o.ID, uuid.New()); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("err = %v", err)
	}
	d, err := q.GetForCustomer(context.Background(), o.ID, h.customer)
	if err != nil || d.Order.ID != o.ID || len(d.History) < 2 {
		t.Fatalf("detail = %+v, err = %v", d, err)
	}
}
