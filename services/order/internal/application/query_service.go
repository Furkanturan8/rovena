package application

import (
	"context"

	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/services/order/internal/repository"
	"github.com/google/uuid"
)

type OrderDetail struct {
	Order   *domain.Order
	History []domain.StatusChange
}

// QueryService answers read-only questions about orders.
type QueryService struct {
	repo repository.OrderRepository
}

func NewQueryService(repo repository.OrderRepository) *QueryService {
	return &QueryService{repo: repo}
}

func (s *QueryService) Get(ctx context.Context, id uuid.UUID) (*OrderDetail, error) {
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	h, err := s.repo.History(ctx, id)
	if err != nil {
		return nil, err
	}
	return &OrderDetail{Order: o, History: h}, nil
}

// GetForCustomer reports someone else's order as not found, so the answer does not
// reveal which order ids exist.
func (s *QueryService) GetForCustomer(ctx context.Context, id, customerID uuid.UUID) (*OrderDetail, error) {
	d, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if d.Order.CustomerID != customerID {
		return nil, domain.ErrOrderNotFound
	}
	return d, nil
}

func (s *QueryService) List(ctx context.Context, f domain.Filter) ([]domain.Order, int, error) {
	return s.repo.List(ctx, f)
}
