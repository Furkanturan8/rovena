package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/google/uuid"
)

const testTTL = 15 * time.Minute

type env struct {
	store        *store
	stock        *StockService
	reservations *ReservationService
}

func newEnv(ttl time.Duration) *env {
	s := newStore()
	return &env{
		store:        s,
		stock:        NewStockService(stockRepo{s}),
		reservations: NewReservationService(reservationRepo{s}, ttl),
	}
}

func items(pairs ...any) []domain.ReservationItem {
	out := make([]domain.ReservationItem, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, domain.ReservationItem{
			VariantID: pairs[i].(uuid.UUID), Quantity: pairs[i+1].(int),
		})
	}
	return out
}

func (e *env) reserve(t *testing.T, orderID uuid.UUID, in []domain.ReservationItem) (*domain.Reservation, bool) {
	t.Helper()
	res, created, err := e.reservations.Reserve(context.Background(), ReserveInput{OrderID: orderID, Items: in})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	return res, created
}

func (e *env) assertStock(t *testing.T, variantID uuid.UUID, onHand, reserved int) {
	t.Helper()
	item := e.store.item(variantID)
	if item.OnHand != onHand || item.Reserved != reserved {
		t.Fatalf("stock = on_hand %d / reserved %d, want %d / %d", item.OnHand, item.Reserved, onHand, reserved)
	}
}

func TestReserveHoldsStockWithoutRemovingIt(t *testing.T) {
	e := newEnv(testTTL)
	variantID := e.store.seed(10)

	res, created := e.reserve(t, uuid.New(), items(variantID, 3))

	if !created {
		t.Fatal("expected a new reservation")
	}
	if res.Status != domain.ReservationPending {
		t.Fatalf("status = %q, want pending", res.Status)
	}
	if res.ExpiresAt.Before(time.Now()) {
		t.Fatal("reservation should expire in the future")
	}
	// Reserving promises stock; the units are still in the warehouse.
	e.assertStock(t, variantID, 10, 3)
}

func TestReserveIsIdempotentPerOrder(t *testing.T) {
	e := newEnv(testTTL)
	variantID := e.store.seed(10)
	orderID := uuid.New()

	first, created := e.reserve(t, orderID, items(variantID, 3))
	if !created {
		t.Fatal("first reserve should create")
	}

	second, created := e.reserve(t, orderID, items(variantID, 3))
	if created {
		t.Fatal("a redelivered reserve must not create a second reservation")
	}
	if second.ID != first.ID {
		t.Fatalf("reservation id = %s, want %s", second.ID, first.ID)
	}
	// The decisive assertion: the stock was held once, not twice.
	e.assertStock(t, variantID, 10, 3)
}

func TestReserveRejectsMoreThanAvailable(t *testing.T) {
	e := newEnv(testTTL)
	variantID := e.store.seed(2)
	e.reserve(t, uuid.New(), items(variantID, 1))

	_, _, err := e.reservations.Reserve(context.Background(), ReserveInput{
		OrderID: uuid.New(), Items: items(variantID, 2),
	})
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("err = %v, want insufficient stock", err)
	}
	var stockErr *domain.StockError
	if !errors.As(err, &stockErr) {
		t.Fatal("error should name the variant that failed")
	}
	if stockErr.VariantID != variantID || stockErr.Requested != 2 || stockErr.Available != 1 {
		t.Fatalf("stock error = %+v", stockErr)
	}
	e.assertStock(t, variantID, 2, 1)
}

func TestReserveRejectsUntrackedVariant(t *testing.T) {
	e := newEnv(testTTL)
	_, _, err := e.reservations.Reserve(context.Background(), ReserveInput{
		OrderID: uuid.New(), Items: items(uuid.New(), 1),
	})
	if !errors.Is(err, domain.ErrStockItemNotFound) {
		t.Fatalf("err = %v, want stock item not found", err)
	}
}

// A multi-item reserve is all-or-nothing: a customer must not end up holding half
// an order while the rest is out of stock.
func TestReserveHoldsNothingWhenOneItemFails(t *testing.T) {
	e := newEnv(testTTL)
	plenty := e.store.seed(10)
	scarce := e.store.seed(1)

	_, _, err := e.reservations.Reserve(context.Background(), ReserveInput{
		OrderID: uuid.New(), Items: items(plenty, 2, scarce, 5),
	})
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("err = %v, want insufficient stock", err)
	}
	e.assertStock(t, plenty, 10, 0)
	e.assertStock(t, scarce, 1, 0)
}

func TestReserveValidation(t *testing.T) {
	e := newEnv(testTTL)
	variantID := e.store.seed(10)

	tests := []struct {
		name  string
		items []domain.ReservationItem
		want  error
	}{
		{"no items", nil, domain.ErrNoItems},
		{"zero quantity", items(variantID, 0), domain.ErrInvalidQuantity},
		{"negative quantity", items(variantID, -1), domain.ErrInvalidQuantity},
		{"duplicate variant", items(variantID, 1, variantID, 1), domain.ErrDuplicateVariant},
		{"too many items", tooManyItems(), domain.ErrTooManyItems},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := e.reservations.Reserve(context.Background(), ReserveInput{
				OrderID: uuid.New(), Items: tc.items,
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func tooManyItems() []domain.ReservationItem {
	out := make([]domain.ReservationItem, domain.MaxReservationItems+1)
	for i := range out {
		out[i] = domain.ReservationItem{VariantID: uuid.New(), Quantity: 1}
	}
	return out
}

func TestCommitTakesStockOutAndIsIdempotent(t *testing.T) {
	e := newEnv(testTTL)
	variantID := e.store.seed(10)
	res, _ := e.reserve(t, uuid.New(), items(variantID, 3))

	committed, err := e.reservations.Commit(context.Background(), res.ID)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if committed.Status != domain.ReservationCommitted {
		t.Fatalf("status = %q, want committed", committed.Status)
	}
	e.assertStock(t, variantID, 7, 0)

	// A redelivered payment-succeeded event must not sell the units twice.
	if _, err := e.reservations.Commit(context.Background(), res.ID); err != nil {
		t.Fatalf("second commit: %v", err)
	}
	e.assertStock(t, variantID, 7, 0)

	movements, total, err := e.stock.Movements(context.Background(), domain.MovementFilter{
		VariantID: variantID, Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("movements: %v", err)
	}
	if total != 1 || movements[0].Type != domain.MovementSale || movements[0].QuantityDelta != -3 {
		t.Fatalf("ledger = %+v (total %d), want one sale of -3", movements, total)
	}
}

func TestReleaseGivesStockBackAndIsIdempotent(t *testing.T) {
	e := newEnv(testTTL)
	variantID := e.store.seed(10)
	res, _ := e.reserve(t, uuid.New(), items(variantID, 4))

	released, err := e.reservations.Release(context.Background(), res.ID)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if released.Status != domain.ReservationReleased {
		t.Fatalf("status = %q, want released", released.Status)
	}
	e.assertStock(t, variantID, 10, 0)

	if _, err := e.reservations.Release(context.Background(), res.ID); err != nil {
		t.Fatalf("second release: %v", err)
	}
	e.assertStock(t, variantID, 10, 0)
}

func TestCommittedReservationCannotBeReleased(t *testing.T) {
	e := newEnv(testTTL)
	variantID := e.store.seed(5)
	res, _ := e.reserve(t, uuid.New(), items(variantID, 1))
	if _, err := e.reservations.Commit(context.Background(), res.ID); err != nil {
		t.Fatalf("commit: %v", err)
	}

	_, err := e.reservations.Release(context.Background(), res.ID)
	if !errors.Is(err, domain.ErrReservationCommitted) {
		t.Fatalf("err = %v, want reservation committed", err)
	}
	e.assertStock(t, variantID, 4, 0)
}

func TestReleasedReservationCannotBeCommitted(t *testing.T) {
	e := newEnv(testTTL)
	variantID := e.store.seed(5)
	res, _ := e.reserve(t, uuid.New(), items(variantID, 1))
	if _, err := e.reservations.Release(context.Background(), res.ID); err != nil {
		t.Fatalf("release: %v", err)
	}

	_, err := e.reservations.Commit(context.Background(), res.ID)
	if !errors.Is(err, domain.ErrReservationClosed) {
		t.Fatalf("err = %v, want reservation closed", err)
	}
	e.assertStock(t, variantID, 5, 0)
}

func TestUnknownReservation(t *testing.T) {
	e := newEnv(testTTL)
	if _, err := e.reservations.Get(context.Background(), uuid.New()); !errors.Is(err, domain.ErrReservationNotFound) {
		t.Fatalf("err = %v, want reservation not found", err)
	}
	if _, err := e.reservations.GetByOrder(context.Background(), uuid.New()); !errors.Is(err, domain.ErrReservationNotFound) {
		t.Fatalf("err = %v, want reservation not found", err)
	}
}

// An abandoned checkout must not hold stock forever.
func TestExpirerReleasesDueReservations(t *testing.T) {
	e := newEnv(-time.Minute) // already expired when created
	variantID := e.store.seed(6)
	res, _ := e.reserve(t, uuid.New(), items(variantID, 6))
	e.assertStock(t, variantID, 6, 6)

	expirer := NewExpirer(reservationRepo{e.store}, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Minute)
	released, err := expirer.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if released != 1 {
		t.Fatalf("released = %d, want 1", released)
	}
	e.assertStock(t, variantID, 6, 0)

	after, err := e.reservations.Get(context.Background(), res.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if after.Status != domain.ReservationExpired {
		t.Fatalf("status = %q, want expired", after.Status)
	}

	// Nothing is due anymore, so a second sweep releases nothing.
	released, err = expirer.RunOnce(context.Background())
	if err != nil || released != 0 {
		t.Fatalf("second sweep = %d, %v; want 0, nil", released, err)
	}
}

func TestExpirerLeavesLiveReservationsAlone(t *testing.T) {
	e := newEnv(testTTL)
	variantID := e.store.seed(6)
	e.reserve(t, uuid.New(), items(variantID, 2))

	expirer := NewExpirer(reservationRepo{e.store}, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Minute)
	released, err := expirer.RunOnce(context.Background())
	if err != nil || released != 0 {
		t.Fatalf("sweep = %d, %v; want 0, nil", released, err)
	}
	e.assertStock(t, variantID, 6, 2)
}

func TestCreateStockNormalizesSKU(t *testing.T) {
	e := newEnv(testTTL)
	item, err := e.stock.Create(context.Background(), CreateStockInput{
		VariantID: uuid.New(), SKU: "  tsh-001-m-blk ", OnHand: 5,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if item.SKU != "TSH-001-M-BLK" {
		t.Fatalf("sku = %q, want TSH-001-M-BLK", item.SKU)
	}
	if item.Available() != 5 {
		t.Fatalf("available = %d, want 5", item.Available())
	}

	movements, total, err := e.stock.Movements(context.Background(), domain.MovementFilter{
		VariantID: item.VariantID, Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("movements: %v", err)
	}
	if total != 1 || movements[0].Type != domain.MovementReceipt || movements[0].QuantityDelta != 5 {
		t.Fatalf("ledger = %+v, want an opening receipt of 5", movements)
	}
}

func TestCreateStockRejectsInvalidInput(t *testing.T) {
	e := newEnv(testTTL)
	variantID := uuid.New()

	if _, err := e.stock.Create(context.Background(), CreateStockInput{VariantID: variantID, SKU: "a b c"}); !errors.Is(err, domain.ErrInvalidSKU) {
		t.Fatalf("err = %v, want invalid sku", err)
	}
	if _, err := e.stock.Create(context.Background(), CreateStockInput{VariantID: variantID, SKU: "SKU-1", OnHand: -1}); !errors.Is(err, domain.ErrInvalidQuantity) {
		t.Fatalf("err = %v, want invalid quantity", err)
	}
	if _, err := e.stock.Create(context.Background(), CreateStockInput{VariantID: variantID, SKU: "SKU-1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := e.stock.Create(context.Background(), CreateStockInput{VariantID: variantID, SKU: "SKU-2"}); !errors.Is(err, domain.ErrStockItemExists) {
		t.Fatalf("err = %v, want stock item exists", err)
	}
	if _, err := e.stock.Create(context.Background(), CreateStockInput{VariantID: uuid.New(), SKU: "SKU-1"}); !errors.Is(err, domain.ErrSKUTaken) {
		t.Fatalf("err = %v, want sku taken", err)
	}
}

func TestReceiveAndAdjust(t *testing.T) {
	e := newEnv(testTTL)
	variantID := e.store.seed(4)

	if _, err := e.stock.Receive(context.Background(), variantID, 6, "delivery"); err != nil {
		t.Fatalf("receive: %v", err)
	}
	e.assertStock(t, variantID, 10, 0)

	if _, err := e.stock.Adjust(context.Background(), variantID, -2, "damaged"); err != nil {
		t.Fatalf("adjust: %v", err)
	}
	e.assertStock(t, variantID, 8, 0)

	if _, err := e.stock.Receive(context.Background(), variantID, 0, ""); !errors.Is(err, domain.ErrInvalidQuantity) {
		t.Fatalf("err = %v, want invalid quantity", err)
	}
	if _, err := e.stock.Adjust(context.Background(), variantID, 0, "noop"); !errors.Is(err, domain.ErrInvalidQuantity) {
		t.Fatalf("err = %v, want invalid quantity", err)
	}
	if _, err := e.stock.Receive(context.Background(), uuid.New(), 1, ""); !errors.Is(err, domain.ErrStockItemNotFound) {
		t.Fatalf("err = %v, want stock item not found", err)
	}
}

// Stock that is already promised to an order cannot be adjusted away.
func TestAdjustCannotEatIntoReservedStock(t *testing.T) {
	e := newEnv(testTTL)
	variantID := e.store.seed(10)
	e.reserve(t, uuid.New(), items(variantID, 8))

	if _, err := e.stock.Adjust(context.Background(), variantID, -5, "stock count"); !errors.Is(err, domain.ErrReservedExceedsOnHand) {
		t.Fatalf("err = %v, want reserved exceeds on hand", err)
	}
	e.assertStock(t, variantID, 10, 8)

	// Reducing down to the reserved quantity is still allowed.
	if _, err := e.stock.Adjust(context.Background(), variantID, -2, "stock count"); err != nil {
		t.Fatalf("adjust: %v", err)
	}
	e.assertStock(t, variantID, 8, 8)
}

func TestAvailabilityAnswersForEveryRequestedVariant(t *testing.T) {
	e := newEnv(testTTL)
	tracked := e.store.seed(5)
	untracked := uuid.New()
	e.reserve(t, uuid.New(), items(tracked, 2))

	out, err := e.stock.Availability(context.Background(), []uuid.UUID{untracked, tracked, tracked})
	if err != nil {
		t.Fatalf("availability: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2 (duplicates collapsed)", len(out))
	}
	if out[0].VariantID != untracked || out[0].Available != 0 {
		t.Fatalf("untracked variant = %+v, want available 0", out[0])
	}
	// Reserved units are not available to anyone else.
	if out[1].VariantID != tracked || out[1].Available != 3 {
		t.Fatalf("tracked variant = %+v, want available 3", out[1])
	}
}

func TestListFiltersLowStock(t *testing.T) {
	e := newEnv(testTTL)
	low := e.store.seed(1)
	e.store.seed(50)

	below := 5
	out, total, err := e.stock.List(context.Background(), domain.StockFilter{Below: &below, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || out[0].VariantID != low {
		t.Fatalf("list = %+v (total %d), want only the low-stock item", out, total)
	}

	if _, total, err = e.stock.List(context.Background(), domain.StockFilter{Page: 1, PageSize: 10}); err != nil {
		t.Fatalf("list: %v", err)
	} else if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
}

func TestMovementsRejectsUnknownVariant(t *testing.T) {
	e := newEnv(testTTL)
	_, _, err := e.stock.Movements(context.Background(), domain.MovementFilter{
		VariantID: uuid.New(), Page: 1, PageSize: 10,
	})
	if !errors.Is(err, domain.ErrStockItemNotFound) {
		t.Fatalf("err = %v, want stock item not found", err)
	}
}
