package application_test

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/services/payment/internal/repository"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/google/uuid"
)

// memRepo is an in-memory PaymentRepository that follows the same rules as the
// PostgreSQL one: unique idempotency key, completion only from pending, and one
// succeeded payment per order.
type memRepo struct {
	mu       sync.Mutex
	payments map[uuid.UUID]*domain.Payment
	outbox   []messaging.Envelope
}

var _ repository.PaymentRepository = (*memRepo)(nil)

func newMemRepo() *memRepo { return &memRepo{payments: map[uuid.UUID]*domain.Payment{}} }

func (r *memRepo) byKey(key string) *domain.Payment {
	for _, p := range r.payments {
		if p.IdempotencyKey == key {
			return p
		}
	}
	return nil
}

func (r *memRepo) Begin(_ context.Context, p *domain.Payment) (*domain.Payment, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing := r.byKey(p.IdempotencyKey); existing != nil {
		c := *existing
		return &c, false, nil
	}
	c := *p
	r.payments[p.ID] = &c
	return p, true, nil
}

func (r *memRepo) StartAttempt(_ context.Context, id uuid.UUID) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.payments[id]
	if !ok {
		return nil, domain.ErrPaymentNotFound
	}
	if p.Status == domain.StatusPending {
		p.Attempts++
	}
	c := *p
	return &c, nil
}

func (r *memRepo) Complete(_ context.Context, id uuid.UUID, o domain.Outcome, ev messaging.Envelope) (*domain.Payment, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.payments[id]
	if !ok {
		return nil, false, domain.ErrPaymentNotFound
	}
	if p.Status != domain.StatusPending {
		c := *p
		return &c, false, nil
	}
	if o.Status == domain.StatusSucceeded {
		for _, other := range r.payments {
			if other.OrderID == p.OrderID && (other.Status == domain.StatusSucceeded || other.Status == domain.StatusRefunded) {
				return nil, false, domain.ErrOrderAlreadyPaid
			}
		}
	}
	now := time.Now()
	p.Status, p.ProviderRef, p.FailureCode, p.FailureMessage = o.Status, o.ProviderRef, o.FailureCode, o.FailureMessage
	p.CompletedAt, p.UpdatedAt = &now, now
	r.outbox = append(r.outbox, ev)
	c := *p
	return &c, true, nil
}

func (r *memRepo) Enqueue(_ context.Context, ev messaging.Envelope) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outbox = append(r.outbox, ev)
	return nil
}

func (r *memRepo) HasSucceeded(_ context.Context, orderID uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.payments {
		if p.OrderID == orderID && (p.Status == domain.StatusSucceeded || p.Status == domain.StatusRefunded) {
			return true, nil
		}
	}
	return false, nil
}

func (r *memRepo) PaidByOrder(_ context.Context, orderID uuid.UUID) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.payments {
		if p.OrderID == orderID && (p.Status == domain.StatusSucceeded || p.Status == domain.StatusRefunded) {
			c := *p
			return &c, nil
		}
	}
	return nil, domain.ErrPaymentNotFound
}

func (r *memRepo) Refund(_ context.Context, id uuid.UUID, refundRef string, ev messaging.Envelope) (*domain.Payment, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.payments[id]
	if !ok {
		return nil, false, domain.ErrPaymentNotFound
	}
	if p.Status != domain.StatusSucceeded {
		c := *p
		return &c, false, nil
	}
	now := time.Now()
	p.Status, p.RefundRef, p.RefundedAt, p.UpdatedAt = domain.StatusRefunded, refundRef, &now, now
	r.outbox = append(r.outbox, ev)
	c := *p
	return &c, true, nil
}

func (r *memRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.payments[id]
	if !ok {
		return nil, domain.ErrPaymentNotFound
	}
	c := *p
	return &c, nil
}

func (r *memRepo) LatestByOrder(_ context.Context, orderID uuid.UUID) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var latest *domain.Payment
	for _, p := range r.payments {
		if p.OrderID == orderID && (latest == nil || p.CreatedAt.After(latest.CreatedAt)) {
			latest = p
		}
	}
	if latest == nil {
		return nil, domain.ErrPaymentNotFound
	}
	c := *latest
	return &c, nil
}

func (r *memRepo) List(_ context.Context, f domain.Filter) ([]domain.Payment, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []domain.Payment
	for _, p := range r.payments {
		if f.Status != "" && p.Status != f.Status {
			continue
		}
		if f.OrderID != nil && p.OrderID != *f.OrderID {
			continue
		}
		all = append(all, *p)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	total := len(all)
	start := min((f.Page-1)*f.PageSize, total)
	end := min(start+f.PageSize, total)
	return all[start:end], total, nil
}

func (r *memRepo) byKeyCopy(key string) *domain.Payment {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *r.byKey(key)
	return &c
}

func (r *memRepo) byKeyID(key string) uuid.UUID { return r.byKeyCopy(key).ID }

func (r *memRepo) events() []messaging.Envelope {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]messaging.Envelope(nil), r.outbox...)
}

func (r *memRepo) only() *domain.Payment {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.payments {
		c := *p
		return &c
	}
	return nil
}

func (r *memRepo) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.payments)
}

// scriptedProvider answers from a function and counts how it is called.
type scriptedProvider struct {
	mu      sync.Mutex
	calls   int
	charged map[string]bool // idempotency keys that were really charged

	refundCalls int
	refundErr   error
	refunded    map[string]bool // idempotency keys that were really refunded
	answer      func(call int, req domain.ChargeRequest) (domain.ChargeResult, error)
}

func newProvider(answer func(call int, req domain.ChargeRequest) (domain.ChargeResult, error)) *scriptedProvider {
	return &scriptedProvider{charged: map[string]bool{}, refunded: map[string]bool{}, answer: answer}
}

func approve(call int, req domain.ChargeRequest) (domain.ChargeResult, error) {
	return domain.ChargeResult{Approved: true, Reference: "ref_" + req.IdempotencyKey}, nil
}

func (p *scriptedProvider) Charge(ctx context.Context, req domain.ChargeRequest) (domain.ChargeResult, error) {
	p.mu.Lock()
	p.calls++
	call := p.calls
	p.mu.Unlock()
	res, err := p.answer(call, req)
	if err == nil && res.Approved {
		p.mu.Lock()
		p.charged[req.IdempotencyKey] = true // like a real provider: repeated keys are not charged twice
		p.mu.Unlock()
	}
	return res, err
}

// Refund answers like a provider that remembers keys, unless refundErr says it is down.
func (p *scriptedProvider) Refund(_ context.Context, req domain.RefundRequest) (domain.RefundResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.refundCalls++
	if p.refundErr != nil {
		return domain.RefundResult{}, p.refundErr
	}
	p.refunded[req.IdempotencyKey] = true
	return domain.RefundResult{Reference: "rf_" + req.ChargeRef}, nil
}

func (p *scriptedProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}
