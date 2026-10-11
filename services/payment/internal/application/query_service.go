package application

import (
	"context"

	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/services/payment/internal/repository"
	"github.com/google/uuid"
)

// QueryService answers read-only questions about payments. Payments are created only by
// PaymentRequested events, never through the API.
type QueryService struct {
	repo repository.PaymentRepository
}

func NewQueryService(repo repository.PaymentRepository) *QueryService {
	return &QueryService{repo: repo}
}

func (s *QueryService) Get(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	return s.repo.GetByID(ctx, id)
}

// LatestByOrder returns the newest payment attempt for an order.
func (s *QueryService) LatestByOrder(ctx context.Context, orderID uuid.UUID) (*domain.Payment, error) {
	return s.repo.LatestByOrder(ctx, orderID)
}

// LatestByOrderForCustomer is LatestByOrder restricted to the customer's own payments.
// Someone else's payment is reported as not found, so the answer does not reveal
// which order IDs exist.
func (s *QueryService) LatestByOrderForCustomer(ctx context.Context, orderID, customerID uuid.UUID) (*domain.Payment, error) {
	p, err := s.repo.LatestByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if p.CustomerID != customerID {
		return nil, domain.ErrPaymentNotFound
	}
	return p, nil
}

func (s *QueryService) List(ctx context.Context, f domain.Filter) ([]domain.Payment, int, error) {
	return s.repo.List(ctx, f)
}
