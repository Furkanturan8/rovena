//go:build integration

package postgres_test

// Runs against a real PostgreSQL with the inventory schema applied:
//   INVENTORY_TEST_DATABASE_URL=postgres://... go test -tags integration ./internal/repository/postgres
// Every test creates its own uniquely named rows and removes them afterwards.
//
// These tests exist for the rules that only a database can really prove: the
// conditional updates and row locks that keep stock from being oversold when
// several orders arrive at the same moment.

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/application"
	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/Furkanturan8/rovena/services/inventory/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/inventory/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type harness struct {
	ctx             context.Context
	db              *bun.DB
	stock           *application.StockService
	reservations    *application.ReservationService
	reservationRepo *postgres.ReservationRepository
}

func setup(t *testing.T) *harness {
	t.Helper()
	dsn := os.Getenv("INVENTORY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("INVENTORY_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	reservationRepo := postgres.NewReservationRepository(db)
	return &harness{
		ctx:             ctx,
		db:              db,
		stock:           application.NewStockService(postgres.NewStockRepository(db)),
		reservations:    application.NewReservationService(reservationRepo, 15*time.Minute),
		reservationRepo: reservationRepo,
	}
}

// newStock creates a tracked variant with its own SKU and removes it, together
// with everything that references it, when the test ends.
func (h *harness) newStock(t *testing.T, onHand int) uuid.UUID {
	t.Helper()
	variantID := uuid.New()
	sku := "ITEST-" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))[:24]
	if _, err := h.stock.Create(h.ctx, application.CreateStockInput{
		VariantID: variantID, SKU: sku, OnHand: onHand, Reason: "integration test",
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = h.db.ExecContext(h.ctx,
			"DELETE FROM reservations WHERE id IN (SELECT reservation_id FROM reservation_items WHERE variant_id = ?)", variantID)
		_, _ = h.db.ExecContext(h.ctx, "DELETE FROM stock_movements WHERE variant_id = ?", variantID)
		_, _ = h.db.ExecContext(h.ctx, "DELETE FROM stock_items WHERE variant_id = ?", variantID)
	})
	return variantID
}

func (h *harness) assertStock(t *testing.T, variantID uuid.UUID, onHand, reserved int) {
	t.Helper()
	item, err := h.stock.Get(h.ctx, variantID)
	if err != nil {
		t.Fatal(err)
	}
	if item.OnHand != onHand || item.Reserved != reserved {
		t.Fatalf("stock = on_hand %d / reserved %d, want %d / %d", item.OnHand, item.Reserved, onHand, reserved)
	}
}

func (h *harness) reserve(orderID uuid.UUID, items ...domain.ReservationItem) (*domain.Reservation, bool, error) {
	return h.reservations.Reserve(h.ctx, application.ReserveInput{OrderID: orderID, Items: items})
}

func item(variantID uuid.UUID, quantity int) domain.ReservationItem {
	return domain.ReservationItem{VariantID: variantID, Quantity: quantity}
}

func TestReserveCommitFlow(t *testing.T) {
	h := setup(t)
	variantID := h.newStock(t, 10)

	res, created, err := h.reserve(uuid.New(), item(variantID, 3))
	if err != nil || !created {
		t.Fatalf("reserve = %v, created %v", err, created)
	}
	h.assertStock(t, variantID, 10, 3)

	committed, err := h.reservations.Commit(h.ctx, res.ID)
	if err != nil {
		t.Fatal(err)
	}
	if committed.Status != domain.ReservationCommitted {
		t.Fatalf("status = %q, want committed", committed.Status)
	}
	h.assertStock(t, variantID, 7, 0)

	movements, _, err := h.stock.Movements(h.ctx, domain.MovementFilter{VariantID: variantID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	sale := movements[0]
	if sale.Type != domain.MovementSale || sale.QuantityDelta != -3 || sale.ReservationID == nil || *sale.ReservationID != res.ID {
		t.Fatalf("latest movement = %+v, want a sale of -3 linked to the reservation", sale)
	}
}

// The test the whole service exists for: many customers, one unit left.
func TestConcurrentReservationsDoNotOversell(t *testing.T) {
	h := setup(t)
	variantID := h.newStock(t, 1)

	const racers = 8
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		wins     int
		rejected int
	)
	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, created, err := h.reserve(uuid.New(), item(variantID, 1))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && created:
				wins++
			case errors.Is(err, domain.ErrInsufficientStock):
				rejected++
			default:
				t.Errorf("unexpected reserve result: created=%v err=%v", created, err)
			}
		}()
	}
	wg.Wait()

	if wins != 1 || rejected != racers-1 {
		t.Fatalf("wins = %d, rejected = %d; want exactly 1 winner out of %d", wins, rejected, racers)
	}
	h.assertStock(t, variantID, 1, 1)
}

// Duplicate delivery of the same checkout message must hold the stock once.
func TestConcurrentReserveOfSameOrderHoldsStockOnce(t *testing.T) {
	h := setup(t)
	variantID := h.newStock(t, 10)
	orderID := uuid.New()

	const attempts = 6
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		creates int
		ids     = map[uuid.UUID]bool{}
	)
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, created, err := h.reserve(orderID, item(variantID, 2))
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				t.Errorf("reserve: %v", err)
				return
			}
			ids[res.ID] = true
			if created {
				creates++
			}
		}()
	}
	wg.Wait()

	if creates != 1 || len(ids) != 1 {
		t.Fatalf("creates = %d, distinct reservations = %d; want 1 and 1", creates, len(ids))
	}
	h.assertStock(t, variantID, 10, 2)
}

// A reserve spanning several variants is all-or-nothing, including its own row.
func TestMultiItemReserveRollsBackCompletely(t *testing.T) {
	h := setup(t)
	plenty := h.newStock(t, 10)
	scarce := h.newStock(t, 1)
	orderID := uuid.New()

	_, _, err := h.reserve(orderID, item(plenty, 2), item(scarce, 5))
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("err = %v, want insufficient stock", err)
	}
	h.assertStock(t, plenty, 10, 0)
	h.assertStock(t, scarce, 1, 0)

	if _, err := h.reservations.GetByOrder(h.ctx, orderID); !errors.Is(err, domain.ErrReservationNotFound) {
		t.Fatalf("err = %v, want no reservation row left behind", err)
	}
}

func TestReserveRejectsUntrackedVariant(t *testing.T) {
	h := setup(t)
	orderID := uuid.New()

	_, _, err := h.reserve(orderID, item(uuid.New(), 1))
	if !errors.Is(err, domain.ErrStockItemNotFound) {
		t.Fatalf("err = %v, want stock item not found", err)
	}
	if _, err := h.reservations.GetByOrder(h.ctx, orderID); !errors.Is(err, domain.ErrReservationNotFound) {
		t.Fatalf("err = %v, want no reservation row left behind", err)
	}
}

// Items are always locked in variant order, so two orders holding the same two
// variants in opposite request order cannot deadlock each other.
func TestOppositeItemOrderDoesNotDeadlock(t *testing.T) {
	h := setup(t)
	first := h.newStock(t, 100)
	second := h.newStock(t, 100)

	const pairs = 15
	var wg sync.WaitGroup
	for range pairs {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, _, err := h.reserve(uuid.New(), item(first, 1), item(second, 1)); err != nil {
				t.Errorf("reserve in request order: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, _, err := h.reserve(uuid.New(), item(second, 1), item(first, 1)); err != nil {
				t.Errorf("reserve in reverse order: %v", err)
			}
		}()
	}
	wg.Wait()

	h.assertStock(t, first, 100, 2*pairs)
	h.assertStock(t, second, 100, 2*pairs)
}

func TestConcurrentCommitAndReleaseResolveToOneOutcome(t *testing.T) {
	h := setup(t)
	variantID := h.newStock(t, 10)
	res, _, err := h.reserve(uuid.New(), item(variantID, 4))
	if err != nil {
		t.Fatal(err)
	}

	// The payment result and a timeout cancellation arrive at the same instant.
	// Both goroutines wait on the same barrier so they really do overlap.
	var (
		wg                sync.WaitGroup
		mu                sync.Mutex
		commitErr, relErr error
		start             = make(chan struct{})
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, err := h.reservations.Commit(h.ctx, res.ID)
		mu.Lock()
		commitErr = err
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		<-start
		_, err := h.reservations.Release(h.ctx, res.ID)
		mu.Lock()
		relErr = err
		mu.Unlock()
	}()
	close(start)
	wg.Wait()

	// The row lock serializes them: whoever runs second sees the first one's result
	// and is turned away with a domain error, not with a constraint violation that
	// would reach the client as an internal error.
	if (commitErr == nil) == (relErr == nil) {
		t.Fatalf("commit err = %v, release err = %v; want exactly one to succeed", commitErr, relErr)
	}
	loser := commitErr
	if loser == nil {
		loser = relErr
	}
	if !errors.Is(loser, domain.ErrReservationCommitted) && !errors.Is(loser, domain.ErrReservationClosed) {
		t.Fatalf("loser err = %v, want a reservation state error", loser)
	}

	final, err := h.reservations.Get(h.ctx, res.ID)
	if err != nil {
		t.Fatal(err)
	}
	if commitErr == nil {
		if final.Status != domain.ReservationCommitted {
			t.Fatalf("status = %q, want committed", final.Status)
		}
		h.assertStock(t, variantID, 6, 0)
		return
	}
	if final.Status != domain.ReservationReleased {
		t.Fatalf("status = %q, want released", final.Status)
	}
	h.assertStock(t, variantID, 10, 0)
}

func TestReleaseIsIdempotent(t *testing.T) {
	h := setup(t)
	variantID := h.newStock(t, 5)
	res, _, err := h.reserve(uuid.New(), item(variantID, 5))
	if err != nil {
		t.Fatal(err)
	}

	for i := range 3 {
		if _, err := h.reservations.Release(h.ctx, res.ID); err != nil {
			t.Fatalf("release %d: %v", i+1, err)
		}
	}
	// Three releases must not hand back fifteen units.
	h.assertStock(t, variantID, 5, 0)
}

func TestAdjustCannotEatIntoReservedStock(t *testing.T) {
	h := setup(t)
	variantID := h.newStock(t, 10)
	if _, _, err := h.reserve(uuid.New(), item(variantID, 8)); err != nil {
		t.Fatal(err)
	}

	if _, err := h.stock.Adjust(h.ctx, variantID, -5, "stock count"); !errors.Is(err, domain.ErrReservedExceedsOnHand) {
		t.Fatalf("err = %v, want reserved exceeds on hand", err)
	}
	h.assertStock(t, variantID, 10, 8)

	if _, err := h.stock.Adjust(h.ctx, variantID, -2, "stock count"); err != nil {
		t.Fatalf("adjust down to the reserved quantity: %v", err)
	}
	h.assertStock(t, variantID, 8, 8)

	movements, total, err := h.stock.Movements(h.ctx, domain.MovementFilter{VariantID: variantID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	// Only the opening receipt and the adjustment that went through are recorded.
	if total != 2 || movements[0].Type != domain.MovementAdjustment || movements[0].QuantityDelta != -2 {
		t.Fatalf("ledger = %+v (total %d)", movements, total)
	}
}

func TestExpireDueReleasesAbandonedHolds(t *testing.T) {
	h := setup(t)
	variantID := h.newStock(t, 6)

	// A service whose holds are already expired stands in for an abandoned checkout.
	expired := application.NewReservationService(h.reservationRepo, -time.Minute)
	res, _, err := expired.Reserve(h.ctx, application.ReserveInput{
		OrderID: uuid.New(), Items: []domain.ReservationItem{item(variantID, 6)},
	})
	if err != nil {
		t.Fatal(err)
	}
	h.assertStock(t, variantID, 6, 6)

	released, err := h.reservationRepo.ExpireDue(h.ctx, time.Now(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if released < 1 {
		t.Fatalf("released = %d, want at least 1", released)
	}
	h.assertStock(t, variantID, 6, 0)

	after, err := h.reservations.Get(h.ctx, res.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != domain.ReservationExpired {
		t.Fatalf("status = %q, want expired", after.Status)
	}
	if _, err := h.reservations.Commit(h.ctx, res.ID); !errors.Is(err, domain.ErrReservationClosed) {
		t.Fatalf("err = %v, want reservation closed", err)
	}
}

func TestExpireDueLeavesLiveHolds(t *testing.T) {
	h := setup(t)
	variantID := h.newStock(t, 4)
	if _, _, err := h.reserve(uuid.New(), item(variantID, 2)); err != nil {
		t.Fatal(err)
	}

	// Sweeping with a cutoff before the hold was taken must change nothing.
	released, err := h.reservationRepo.ExpireDue(h.ctx, time.Now().Add(-time.Hour), 100)
	if err != nil {
		t.Fatal(err)
	}
	if released != 0 {
		t.Fatalf("released = %d, want 0", released)
	}
	h.assertStock(t, variantID, 4, 2)
}

func TestDuplicateStockItemAndSKU(t *testing.T) {
	h := setup(t)
	variantID := h.newStock(t, 1)
	existing, err := h.stock.Get(h.ctx, variantID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = h.stock.Create(h.ctx, application.CreateStockInput{VariantID: variantID, SKU: "ITEST-OTHER-" + uuid.NewString()[:8]})
	if !errors.Is(err, domain.ErrStockItemExists) {
		t.Fatalf("err = %v, want stock item exists", err)
	}
	_, err = h.stock.Create(h.ctx, application.CreateStockInput{VariantID: uuid.New(), SKU: existing.SKU})
	if !errors.Is(err, domain.ErrSKUTaken) {
		t.Fatalf("err = %v, want sku taken", err)
	}
}

func TestListAndAvailability(t *testing.T) {
	h := setup(t)
	low := h.newStock(t, 2)
	if _, _, err := h.reserve(uuid.New(), item(low, 1)); err != nil {
		t.Fatal(err)
	}

	avail, err := h.stock.Availability(h.ctx, []uuid.UUID{low, uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if len(avail) != 2 || avail[0].Available != 1 || avail[1].Available != 0 {
		t.Fatalf("availability = %+v, want 1 for the tracked variant and 0 for the unknown one", avail)
	}

	item, err := h.stock.Get(h.ctx, low)
	if err != nil {
		t.Fatal(err)
	}
	items, total, err := h.stock.List(h.ctx, domain.StockFilter{SKU: item.SKU, Sort: domain.SortSKU, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || items[0].VariantID != low {
		t.Fatalf("list = %+v (total %d), want the one item matching the sku prefix", items, total)
	}

	below := 2
	items, _, err = h.stock.List(h.ctx, domain.StockFilter{
		SKU: item.SKU, Below: &below, Sort: domain.SortAvailableAsc, Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("low-stock list = %+v, want the item with 1 available", items)
	}
}

// A paid order cannot be cancelled by the compensating action, and a cancelled one cannot be
// paid for later. Both must leave the stock exactly as it was.
func TestCommitAndReleaseRefuseEachOthersReservations(t *testing.T) {
	h := setup(t)
	variantID := h.newStock(t, 10)

	paid, _, err := h.reserve(uuid.New(), item(variantID, 3))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.reservations.Commit(h.ctx, paid.ID); err != nil {
		t.Fatal(err)
	}
	h.assertStock(t, variantID, 7, 0)

	if _, err := h.reservations.Release(h.ctx, paid.ID); !errors.Is(err, domain.ErrReservationCommitted) {
		t.Fatalf("release of a committed reservation: err = %v, want ErrReservationCommitted", err)
	}
	h.assertStock(t, variantID, 7, 0)
	if got, _ := h.reservations.Get(h.ctx, paid.ID); got.Status != domain.ReservationCommitted {
		t.Fatalf("status = %q, want it to stay committed", got.Status)
	}

	// Committing again is a no-op, not a second sale.
	for i := range 2 {
		if _, err := h.reservations.Commit(h.ctx, paid.ID); err != nil {
			t.Fatalf("repeat commit %d: %v", i+1, err)
		}
	}
	h.assertStock(t, variantID, 7, 0)
	movements, total, err := h.stock.Movements(h.ctx, domain.MovementFilter{VariantID: variantID, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	sales := 0
	for _, m := range movements {
		if m.Type == domain.MovementSale {
			sales++
		}
	}
	if sales != 1 {
		t.Fatalf("%d sale movements out of %d, want exactly 1", sales, total)
	}

	cancelled, _, err := h.reserve(uuid.New(), item(variantID, 2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.reservations.Release(h.ctx, cancelled.ID); err != nil {
		t.Fatal(err)
	}
	h.assertStock(t, variantID, 7, 0)
	if _, err := h.reservations.Commit(h.ctx, cancelled.ID); !errors.Is(err, domain.ErrReservationClosed) {
		t.Fatalf("commit of a released reservation: err = %v, want ErrReservationClosed", err)
	}
	h.assertStock(t, variantID, 7, 0)
}

// The sweep takes its work in batches ordered by expiry. Reservations that are already closed
// must never occupy a batch slot, or one old committed order could block every later expiry.
func TestExpireDueBatchOnlyContainsPendingReservations(t *testing.T) {
	h := setup(t)
	variantID := h.newStock(t, 10)

	longGone := application.NewReservationService(h.reservationRepo, -2*time.Hour)
	closed, _, err := longGone.Reserve(h.ctx, application.ReserveInput{
		OrderID: uuid.New(), Items: []domain.ReservationItem{item(variantID, 2)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.reservations.Commit(h.ctx, closed.ID); err != nil { // paid, but its expires_at is the oldest
		t.Fatal(err)
	}

	recentlyDue := application.NewReservationService(h.reservationRepo, -time.Hour)
	abandoned, _, err := recentlyDue.Reserve(h.ctx, application.ReserveInput{
		OrderID: uuid.New(), Items: []domain.ReservationItem{item(variantID, 3)},
	})
	if err != nil {
		t.Fatal(err)
	}
	h.assertStock(t, variantID, 8, 3)

	// One slot per sweep. Other due holds in the database may go first, so keep sweeping;
	// the committed reservation, if it were selected, would be re-selected forever.
	for range 100 {
		if _, err := h.reservationRepo.ExpireDue(h.ctx, time.Now(), 1); err != nil {
			t.Fatal(err)
		}
		if got, _ := h.reservations.Get(h.ctx, abandoned.ID); got.Status == domain.ReservationExpired {
			break
		}
	}
	got, err := h.reservations.Get(h.ctx, abandoned.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.ReservationExpired {
		t.Fatalf("status = %q: the abandoned hold was never reached", got.Status)
	}
	h.assertStock(t, variantID, 8, 0)
	if c, _ := h.reservations.Get(h.ctx, closed.ID); c.Status != domain.ReservationCommitted {
		t.Fatalf("committed reservation changed to %q", c.Status)
	}
}
