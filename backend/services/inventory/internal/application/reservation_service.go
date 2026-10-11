package application

import (
	"bytes"
	"context"
	"sort"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/Furkanturan8/rovena/services/inventory/internal/repository"
	"github.com/google/uuid"
)

type ReservationService struct {
	reservations repository.ReservationRepository
	ttl          time.Duration
	now          func() time.Time
}

// NewReservationService takes the hold duration from configuration; clients do not
// get to choose how long they keep stock out of other customers' reach.
func NewReservationService(reservations repository.ReservationRepository, ttl time.Duration) *ReservationService {
	return &ReservationService{reservations: reservations, ttl: ttl, now: time.Now}
}

type ReserveInput struct {
	OrderID uuid.UUID
	Items   []domain.ReservationItem
}

// Reserve holds stock for an order. created is false when the order already had a
// reservation, which is what makes a retried or redelivered checkout request safe.
func (s *ReservationService) Reserve(ctx context.Context, in ReserveInput) (res *domain.Reservation, created bool, err error) {
	items, err := normalizeItems(in.Items)
	if err != nil {
		return nil, false, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, false, err
	}
	now := s.now()
	return s.reservations.Reserve(ctx, &domain.Reservation{
		ID: id, OrderID: in.OrderID, Status: domain.ReservationPending, Items: items,
		ExpiresAt: now.Add(s.ttl), CreatedAt: now, UpdatedAt: now,
	})
}

func (s *ReservationService) Commit(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	return s.reservations.Commit(ctx, id)
}

func (s *ReservationService) Release(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	return s.reservations.Release(ctx, id)
}

func (s *ReservationService) Get(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	return s.reservations.GetByID(ctx, id)
}

func (s *ReservationService) GetByOrder(ctx context.Context, orderID uuid.UUID) (*domain.Reservation, error) {
	return s.reservations.GetByOrderID(ctx, orderID)
}

// normalizeItems rejects malformed lines and sorts by variant, the order every
// transaction takes stock row locks in.
func normalizeItems(in []domain.ReservationItem) ([]domain.ReservationItem, error) {
	if len(in) == 0 {
		return nil, domain.ErrNoItems
	}
	if len(in) > domain.MaxReservationItems {
		return nil, domain.ErrTooManyItems
	}
	seen := make(map[uuid.UUID]bool, len(in))
	for _, it := range in {
		if it.Quantity <= 0 {
			return nil, domain.ErrInvalidQuantity
		}
		if seen[it.VariantID] {
			return nil, domain.ErrDuplicateVariant
		}
		seen[it.VariantID] = true
	}
	items := append([]domain.ReservationItem(nil), in...)
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i].VariantID, items[j].VariantID
		return bytes.Compare(a[:], b[:]) < 0
	})
	return items, nil
}
