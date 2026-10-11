// Package memory is an in-memory OrderRepository for tests. It enforces the same
// guarded transitions as the Postgres repository, so service and HTTP tests exercise
// the real state-machine rules without a database.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/services/order/internal/repository"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/google/uuid"
)

var _ repository.OrderRepository = (*Repo)(nil)

// Repo is an in-memory OrderRepository that enforces the same guarded transitions
// as the Postgres one, so the service tests exercise real state-machine rules.
type Repo struct {
	mu      sync.Mutex
	now     func() time.Time
	carts   map[uuid.UUID]map[uuid.UUID]int
	Orders  map[uuid.UUID]*domain.Order
	history map[uuid.UUID][]domain.StatusChange
	Outbox  []messaging.Envelope

	// FailNext makes the named method return an error once.
	FailNext map[string]error
	// OnGetCart runs once, before the next cart is read, for tests that need another request to
	// slip in between two steps of a checkout.
	OnGetCart func()
}

func New(now func() time.Time) *Repo {
	return &Repo{
		now: now, carts: map[uuid.UUID]map[uuid.UUID]int{}, Orders: map[uuid.UUID]*domain.Order{},
		history: map[uuid.UUID][]domain.StatusChange{}, FailNext: map[string]error{},
	}
}

func (r *Repo) fail(method string) error {
	if err, ok := r.FailNext[method]; ok {
		delete(r.FailNext, method)
		return err
	}
	return nil
}

func (r *Repo) GetCart(_ context.Context, c uuid.UUID) ([]domain.CartItem, error) {
	if r.OnGetCart != nil {
		hook := r.OnGetCart
		r.OnGetCart = nil
		hook()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cartLocked(c), nil
}

func (r *Repo) cartLocked(c uuid.UUID) []domain.CartItem {
	var out []domain.CartItem
	for v, q := range r.carts[c] {
		out = append(out, domain.CartItem{VariantID: v, Quantity: q})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VariantID.String() < out[j].VariantID.String() })
	return out
}

func (r *Repo) SetCartItem(_ context.Context, c uuid.UUID, it domain.CartItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.carts[c] == nil {
		r.carts[c] = map[uuid.UUID]int{}
	}
	if _, ok := r.carts[c][it.VariantID]; !ok && len(r.carts[c]) >= domain.MaxCartLines {
		return domain.ErrCartFull
	}
	r.carts[c][it.VariantID] = it.Quantity
	return nil
}

func (r *Repo) RemoveCartItem(_ context.Context, c, v uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.carts[c], v)
	return nil
}

func (r *Repo) ClearCart(_ context.Context, c uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.carts, c)
	return nil
}

func (r *Repo) Create(_ context.Context, o *domain.Order, cart []domain.CartItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur := r.cartLocked(o.CustomerID)
	if len(cur) != len(cart) {
		return domain.ErrCartChanged
	}
	for i := range cur {
		if cur[i] != cart[i] {
			return domain.ErrCartChanged
		}
	}
	for _, e := range r.Orders {
		if e.CustomerID == o.CustomerID && e.IdempotencyKey == o.IdempotencyKey {
			return domain.ErrDuplicateCheckout
		}
	}
	cp := *o
	cp.CreatedAt, cp.UpdatedAt = r.now(), r.now()
	r.Orders[o.ID] = &cp
	delete(r.carts, o.CustomerID)
	r.noteLocked(o.ID, nil, o.Status, "order placed")
	return nil
}

func (r *Repo) noteLocked(id uuid.UUID, from *domain.Status, to domain.Status, reason string) {
	r.history[id] = append(r.history[id], domain.StatusChange{FromStatus: from, ToStatus: to, Reason: reason, CreatedAt: r.now()})
}

func (r *Repo) GetByID(_ context.Context, id uuid.UUID) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.Orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	cp := *o
	return &cp, nil
}

func (r *Repo) GetByIdempotencyKey(_ context.Context, c uuid.UUID, key string) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, o := range r.Orders {
		if o.CustomerID == c && o.IdempotencyKey == key {
			cp := *o
			return &cp, nil
		}
	}
	return nil, domain.ErrOrderNotFound
}

func (r *Repo) History(_ context.Context, id uuid.UUID) ([]domain.StatusChange, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.StatusChange(nil), r.history[id]...), nil
}

func (r *Repo) List(_ context.Context, f domain.Filter) ([]domain.Order, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Order
	for _, o := range r.Orders {
		if f.CustomerID != nil && o.CustomerID != *f.CustomerID {
			continue
		}
		if f.Status != "" && o.Status != f.Status {
			continue
		}
		out = append(out, *o)
	}
	return out, len(out), nil
}

// transition mirrors OrderRepository.apply.
func (r *Repo) transition(id uuid.UUID, from []domain.Status, resv domain.ReservationState, to domain.Status, reason string,
	ev *messaging.Envelope, set func(*domain.Order)) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.Orders[id]
	if !ok {
		return false, domain.ErrOrderNotFound
	}
	allowed := false
	for _, s := range from {
		allowed = allowed || s == o.Status
	}
	if !allowed || (resv != "" && o.ReservationState != resv) {
		return false, nil
	}
	prev := o.Status
	o.Status = to
	o.UpdatedAt = r.now()
	if set != nil {
		set(o)
	}
	r.noteLocked(id, &prev, to, reason)
	if ev != nil {
		r.Outbox = append(r.Outbox, *ev)
	}
	return true, nil
}

func (r *Repo) MarkReserved(_ context.Context, id, res uuid.UUID, ev messaging.Envelope) (bool, error) {
	if err := r.fail("MarkReserved"); err != nil {
		return false, err
	}
	return r.transition(id, []domain.Status{domain.StatusPending}, "", domain.StatusAwaitingPayment, "reserved", &ev, func(o *domain.Order) {
		o.ReservationID = &res
		o.ReservationState = domain.ReservationHeld
		o.PaymentRequests++
	})
}

func (r *Repo) MarkPaid(_ context.Context, id uuid.UUID) (bool, error) {
	return r.transition(id, []domain.Status{domain.StatusAwaitingPayment}, "", domain.StatusPaid, "paid", nil, nil)
}

func (r *Repo) Confirm(_ context.Context, id uuid.UUID, ev messaging.Envelope) (bool, error) {
	return r.transition(id, []domain.Status{domain.StatusPaid}, domain.ReservationHeld, domain.StatusConfirmed, "committed", &ev, func(o *domain.Order) {
		o.ReservationState = domain.ReservationCommitted
	})
}

func (r *Repo) Cancel(_ context.Context, id uuid.UUID, from []domain.Status, code, msg string, ev messaging.Envelope) (bool, error) {
	return r.transition(id, from, "", domain.StatusCancelled, code, &ev, func(o *domain.Order) {
		o.FailureCode, o.FailureMessage = code, msg
	})
}

func (r *Repo) MarkReleased(_ context.Context, id uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.Orders[id]
	if !ok || o.Status != domain.StatusCancelled || o.ReservationState != domain.ReservationHeld {
		return false, nil
	}
	o.ReservationState = domain.ReservationReleased
	o.UpdatedAt = r.now()
	return true, nil
}

func (r *Repo) RequestPaymentAgain(_ context.Context, id uuid.UUID, ev messaging.Envelope) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.Orders[id]
	if !ok || o.Status != domain.StatusAwaitingPayment {
		return false, nil
	}
	o.PaymentRequests++
	o.UpdatedAt = r.now()
	r.Outbox = append(r.Outbox, ev)
	return true, nil
}

func (r *Repo) refundStep(id uuid.UUID, ev messaging.Envelope, from domain.RefundState) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.Orders[id]
	if !ok || o.Status != domain.StatusCancelled || o.RefundState != from {
		return false, nil
	}
	o.RefundState = domain.RefundRequested
	o.RefundRequests++
	o.UpdatedAt = r.now()
	r.Outbox = append(r.Outbox, ev)
	return true, nil
}

func (r *Repo) RequestRefund(_ context.Context, id uuid.UUID, ev messaging.Envelope) (bool, error) {
	if err := r.fail("RequestRefund"); err != nil {
		return false, err
	}
	return r.refundStep(id, ev, domain.RefundNone)
}

func (r *Repo) RequestRefundAgain(_ context.Context, id uuid.UUID, ev messaging.Envelope) (bool, error) {
	return r.refundStep(id, ev, domain.RefundRequested)
}

func (r *Repo) MarkRefunded(_ context.Context, id uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.Orders[id]
	if !ok || o.Status != domain.StatusCancelled || o.RefundState != domain.RefundRequested {
		return false, nil
	}
	o.RefundState = domain.RefundRefunded
	o.UpdatedAt = r.now()
	return true, nil
}

func (r *Repo) RestoreCart(_ context.Context, customerID uuid.UUID, items []domain.CartItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.carts[customerID] == nil {
		r.carts[customerID] = map[uuid.UUID]int{}
	}
	for _, it := range items {
		if len(r.carts[customerID]) >= domain.MaxCartLines {
			break
		}
		if _, have := r.carts[customerID][it.VariantID]; !have {
			r.carts[customerID][it.VariantID] = it.Quantity
		}
	}
	return nil
}

func (r *Repo) Touch(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if o, ok := r.Orders[id]; ok {
		o.UpdatedAt = r.now()
	}
	return nil
}

func (r *Repo) Unfinished(_ context.Context, olderThan time.Time, limit int) ([]domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Order
	for _, o := range r.Orders {
		unfinished := o.Status == domain.StatusPending || o.Status == domain.StatusAwaitingPayment || o.Status == domain.StatusPaid ||
			(o.Status == domain.StatusCancelled && (o.ReservationState == domain.ReservationHeld ||
				(o.RefundState == domain.RefundRequested && o.RefundRequests < domain.MaxRefundRequests)))
		if unfinished && o.UpdatedAt.Before(olderThan) {
			out = append(out, *o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.Before(out[j].UpdatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Events returns the outbox payload types in order.
func (r *Repo) Events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.Outbox))
	for i, e := range r.Outbox {
		out[i] = e.Type
	}
	return out
}

func (r *Repo) CountEvents(t string) int {
	n := 0
	for _, e := range r.Events() {
		if e == t {
			n++
		}
	}
	return n
}
