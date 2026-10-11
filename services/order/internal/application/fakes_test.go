package application

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)} }

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// ---- catalog ----

type fakeCatalog struct {
	mu       sync.Mutex
	variants map[uuid.UUID]VariantInfo
	err      error
	// onLookup runs before each answer, for tests that change things mid-checkout.
	onLookup func()
}

func newFakeCatalog() *fakeCatalog { return &fakeCatalog{variants: map[uuid.UUID]VariantInfo{}} }

func (c *fakeCatalog) add(name string, price int64) uuid.UUID {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := uuid.New()
	c.variants[id] = VariantInfo{
		VariantID: id, ProductID: uuid.New(), ProductName: name, SKU: "SKU-" + id.String()[:6],
		Size: "M", Color: "Blue", PriceCents: price, Currency: "TRY", Sellable: true,
	}
	return id
}

func (c *fakeCatalog) update(id uuid.UUID, f func(*VariantInfo)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.variants[id]
	f(&v)
	c.variants[id] = v
}

func (c *fakeCatalog) Lookup(_ context.Context, ids []uuid.UUID) ([]VariantInfo, error) {
	if c.onLookup != nil {
		c.onLookup()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	var out []VariantInfo
	for _, id := range ids {
		if v, ok := c.variants[id]; ok {
			out = append(out, v)
		}
	}
	return out, nil
}

// ---- inventory ----

type fakeInventory struct {
	mu           sync.Mutex
	reservations map[uuid.UUID]uuid.UUID // order -> reservation
	shortage     *StockShortage
	reserveErr   error
	commitErr    error
	releaseErr   error
	// onReserve runs after a successful hold, before the answer returns.
	onReserve func(orderID uuid.UUID)

	reserveCalls, commitCalls, releaseCalls int
}

func newFakeInventory() *fakeInventory {
	return &fakeInventory{reservations: map[uuid.UUID]uuid.UUID{}}
}

func (i *fakeInventory) Reserve(_ context.Context, orderID uuid.UUID, _ []ReservationLine) (*Reservation, error) {
	i.mu.Lock()
	i.reserveCalls++
	if i.reserveErr != nil {
		err := i.reserveErr
		i.mu.Unlock()
		return nil, err
	}
	if i.shortage != nil {
		s := *i.shortage
		i.mu.Unlock()
		return nil, &s
	}
	id, ok := i.reservations[orderID]
	if !ok {
		id = uuid.New()
		i.reservations[orderID] = id
	}
	hook := i.onReserve
	i.mu.Unlock()
	if hook != nil {
		hook(orderID)
	}
	return &Reservation{ID: id}, nil
}

func (i *fakeInventory) Commit(_ context.Context, _ uuid.UUID) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.commitCalls++
	return i.commitErr
}

func (i *fakeInventory) Release(_ context.Context, _ uuid.UUID) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.releaseCalls++
	return i.releaseErr
}

var errBoom = errors.New("boom")
