// The fakes mirror internal/application/fakes_test.go: an in-memory stand-in for
// the inventory tables, so the handler tests run without a database.
package http_test

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/google/uuid"
)

// store is an in-memory stand-in for the inventory tables. It copies the rules the
// database enforces — conditional holds, one reservation per order, all-or-nothing
// reserves — so the service tests exercise real behaviour without a database.
type store struct {
	mu           sync.Mutex
	items        map[uuid.UUID]*domain.StockItem
	movements    []domain.Movement
	reservations map[uuid.UUID]*domain.Reservation
	byOrder      map[uuid.UUID]uuid.UUID
}

func newStore() *store {
	return &store{
		items:        map[uuid.UUID]*domain.StockItem{},
		reservations: map[uuid.UUID]*domain.Reservation{},
		byOrder:      map[uuid.UUID]uuid.UUID{},
	}
}

// seed registers a variant with the given on-hand quantity.
func (s *store) seed(onHand int) uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()
	variantID := uuid.New()
	s.items[variantID] = &domain.StockItem{
		ID: uuid.New(), VariantID: variantID, SKU: "SKU-" + strings.ToUpper(variantID.String()[:8]),
		OnHand: onHand, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	return variantID
}

func (s *store) item(variantID uuid.UUID) domain.StockItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.items[variantID]
}

type stockRepo struct{ s *store }

func (f stockRepo) Create(_ context.Context, item *domain.StockItem, reason string) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	if _, ok := f.s.items[item.VariantID]; ok {
		return domain.ErrStockItemExists
	}
	for _, existing := range f.s.items {
		if existing.SKU == item.SKU {
			return domain.ErrSKUTaken
		}
	}
	cp := *item
	f.s.items[item.VariantID] = &cp
	if item.OnHand > 0 {
		f.s.record(item.VariantID, domain.MovementReceipt, item.OnHand, reason, nil)
	}
	return nil
}

func (f stockRepo) GetByVariantID(_ context.Context, variantID uuid.UUID) (*domain.StockItem, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	item, ok := f.s.items[variantID]
	if !ok {
		return nil, domain.ErrStockItemNotFound
	}
	cp := *item
	return &cp, nil
}

func (f stockRepo) List(_ context.Context, filter domain.StockFilter) ([]domain.StockItem, int, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	var all []domain.StockItem
	for _, item := range f.s.items {
		if filter.SKU != "" && !strings.HasPrefix(item.SKU, filter.SKU) {
			continue
		}
		if filter.Below != nil && item.Available() >= *filter.Below {
			continue
		}
		all = append(all, *item)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].SKU < all[j].SKU })

	total := len(all)
	start := min((filter.Page-1)*filter.PageSize, total)
	end := min(start+filter.PageSize, total)
	return all[start:end], total, nil
}

func (f stockRepo) Availability(_ context.Context, variantIDs []uuid.UUID) ([]domain.Availability, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	var out []domain.Availability
	for _, id := range variantIDs {
		item, ok := f.s.items[id]
		if !ok {
			continue
		}
		out = append(out, domain.Availability{VariantID: id, SKU: item.SKU, Available: item.Available()})
	}
	return out, nil
}

func (f stockRepo) ApplyDelta(_ context.Context, variantID uuid.UUID, delta int, typ domain.MovementType, reason string) (*domain.StockItem, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	item, ok := f.s.items[variantID]
	if !ok {
		return nil, domain.ErrStockItemNotFound
	}
	if item.OnHand+delta < item.Reserved {
		return nil, domain.ErrReservedExceedsOnHand
	}
	item.OnHand += delta
	item.UpdatedAt = time.Now()
	f.s.record(variantID, typ, delta, reason, nil)
	cp := *item
	return &cp, nil
}

func (f stockRepo) ListMovements(_ context.Context, filter domain.MovementFilter) ([]domain.Movement, int, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	var all []domain.Movement
	for _, m := range f.s.movements {
		if m.VariantID == filter.VariantID {
			all = append(all, m)
		}
	}
	total := len(all)
	start := min((filter.Page-1)*filter.PageSize, total)
	end := min(start+filter.PageSize, total)
	return all[start:end], total, nil
}

// record appends to the ledger; the caller already holds the lock.
func (s *store) record(variantID uuid.UUID, typ domain.MovementType, delta int, reason string, reservationID *uuid.UUID) {
	s.movements = append(s.movements, domain.Movement{
		ID: uuid.New(), VariantID: variantID, Type: typ, QuantityDelta: delta,
		Reason: reason, ReservationID: reservationID, CreatedAt: time.Now(),
	})
}

type reservationRepo struct{ s *store }

func (f reservationRepo) Reserve(_ context.Context, res *domain.Reservation) (*domain.Reservation, bool, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	if id, ok := f.s.byOrder[res.OrderID]; ok {
		return f.s.load(id), false, nil
	}
	// Everything is checked before anything is held: a reserve either holds every
	// item or none of them, like the database transaction does.
	for _, it := range res.Items {
		item, ok := f.s.items[it.VariantID]
		if !ok {
			return nil, false, &domain.StockError{VariantID: it.VariantID, Requested: it.Quantity, Err: domain.ErrStockItemNotFound}
		}
		if item.Available() < it.Quantity {
			return nil, false, &domain.StockError{
				VariantID: it.VariantID, Requested: it.Quantity,
				Available: item.Available(), Err: domain.ErrInsufficientStock,
			}
		}
	}
	for _, it := range res.Items {
		f.s.items[it.VariantID].Reserved += it.Quantity
	}
	stored := *res
	f.s.reservations[res.ID] = &stored
	f.s.byOrder[res.OrderID] = res.ID
	return f.s.load(res.ID), true, nil
}

func (f reservationRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Reservation, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	res := f.s.load(id)
	if res == nil {
		return nil, domain.ErrReservationNotFound
	}
	return res, nil
}

func (f reservationRepo) GetByOrderID(_ context.Context, orderID uuid.UUID) (*domain.Reservation, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	id, ok := f.s.byOrder[orderID]
	if !ok {
		return nil, domain.ErrReservationNotFound
	}
	return f.s.load(id), nil
}

func (f reservationRepo) Commit(_ context.Context, id uuid.UUID) (*domain.Reservation, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	res, ok := f.s.reservations[id]
	if !ok {
		return nil, domain.ErrReservationNotFound
	}
	if res.Status == domain.ReservationCommitted {
		return f.s.load(id), nil
	}
	if res.Status != domain.ReservationPending {
		return nil, domain.ErrReservationClosed
	}
	for _, it := range res.Items {
		item := f.s.items[it.VariantID]
		item.OnHand -= it.Quantity
		item.Reserved -= it.Quantity
		f.s.record(it.VariantID, domain.MovementSale, -it.Quantity, "", &res.ID)
	}
	res.Status, res.UpdatedAt = domain.ReservationCommitted, time.Now()
	return f.s.load(id), nil
}

func (f reservationRepo) Release(_ context.Context, id uuid.UUID) (*domain.Reservation, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	res, _, err := f.s.release(id, domain.ReservationReleased)
	return res, err
}

func (f reservationRepo) ExpireDue(_ context.Context, now time.Time, limit int) (int, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	released := 0
	for id, res := range f.s.reservations {
		if released == limit {
			break
		}
		if res.Status != domain.ReservationPending || res.ExpiresAt.After(now) {
			continue
		}
		if _, changed, err := f.s.release(id, domain.ReservationExpired); err != nil {
			return released, err
		} else if changed {
			released++
		}
	}
	return released, nil
}

// release is the shared body of Release and ExpireDue; the caller holds the lock.
func (s *store) release(id uuid.UUID, status domain.ReservationStatus) (*domain.Reservation, bool, error) {
	res, ok := s.reservations[id]
	if !ok {
		return nil, false, domain.ErrReservationNotFound
	}
	if res.Status == domain.ReservationCommitted {
		return nil, false, domain.ErrReservationCommitted
	}
	if res.Status != domain.ReservationPending {
		return s.load(id), false, nil
	}
	for _, it := range res.Items {
		s.items[it.VariantID].Reserved -= it.Quantity
	}
	res.Status, res.UpdatedAt = status, time.Now()
	return s.load(id), true, nil
}

// load returns a copy; the caller holds the lock.
func (s *store) load(id uuid.UUID) *domain.Reservation {
	res, ok := s.reservations[id]
	if !ok {
		return nil
	}
	cp := *res
	cp.Items = append([]domain.ReservationItem(nil), res.Items...)
	return &cp
}
