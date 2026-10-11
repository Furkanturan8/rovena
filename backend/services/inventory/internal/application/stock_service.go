package application

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/Furkanturan8/rovena/services/inventory/internal/repository"
	"github.com/google/uuid"
)

// skuPattern matches the SKUs the catalog service produces.
var skuPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{1,63}$`)

const defaultOpeningReason = "opening stock"

type StockService struct {
	stock repository.StockRepository
	now   func() time.Time
}

func NewStockService(stock repository.StockRepository) *StockService {
	return &StockService{stock: stock, now: time.Now}
}

type CreateStockInput struct {
	VariantID uuid.UUID
	SKU       string
	OnHand    int
	Reason    string
}

// Create starts tracking a catalog variant. VariantID and SKU are copies of catalog
// data: Inventory never reads the catalog database, so it cannot verify them.
func (s *StockService) Create(ctx context.Context, in CreateStockInput) (*domain.StockItem, error) {
	sku := strings.ToUpper(strings.TrimSpace(in.SKU))
	if !skuPattern.MatchString(sku) {
		return nil, domain.ErrInvalidSKU
	}
	if in.OnHand < 0 {
		return nil, domain.ErrInvalidQuantity
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = defaultOpeningReason
	}
	now := s.now()
	item := &domain.StockItem{
		ID: id, VariantID: in.VariantID, SKU: sku, OnHand: in.OnHand,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.stock.Create(ctx, item, reason); err != nil {
		return nil, err
	}
	return item, nil
}

// Receive books stock that arrived at the warehouse.
func (s *StockService) Receive(ctx context.Context, variantID uuid.UUID, quantity int, reason string) (*domain.StockItem, error) {
	if quantity <= 0 {
		return nil, domain.ErrInvalidQuantity
	}
	return s.stock.ApplyDelta(ctx, variantID, quantity, domain.MovementReceipt, strings.TrimSpace(reason))
}

// Adjust corrects on-hand stock in either direction, for example after a stock
// count or breakage. Units already reserved for an order cannot be adjusted away.
func (s *StockService) Adjust(ctx context.Context, variantID uuid.UUID, delta int, reason string) (*domain.StockItem, error) {
	if delta == 0 {
		return nil, domain.ErrInvalidQuantity
	}
	return s.stock.ApplyDelta(ctx, variantID, delta, domain.MovementAdjustment, strings.TrimSpace(reason))
}

func (s *StockService) Get(ctx context.Context, variantID uuid.UUID) (*domain.StockItem, error) {
	return s.stock.GetByVariantID(ctx, variantID)
}

func (s *StockService) List(ctx context.Context, f domain.StockFilter) ([]domain.StockItem, int, error) {
	f.SKU = strings.ToUpper(strings.TrimSpace(f.SKU))
	return s.stock.List(ctx, f)
}

func (s *StockService) Movements(ctx context.Context, f domain.MovementFilter) ([]domain.Movement, int, error) {
	// Confirms the variant is tracked, so an unknown id is a 404 instead of an empty page.
	if _, err := s.stock.GetByVariantID(ctx, f.VariantID); err != nil {
		return nil, 0, err
	}
	return s.stock.ListMovements(ctx, f)
}

// Availability answers one entry per requested variant, in the requested order.
// A variant Inventory does not track counts as unavailable rather than missing:
// the storefront asks about whatever the catalog shows and wants an answer for all of it.
func (s *StockService) Availability(ctx context.Context, variantIDs []uuid.UUID) ([]domain.Availability, error) {
	known, err := s.stock.Availability(ctx, variantIDs)
	if err != nil {
		return nil, err
	}
	byVariant := make(map[uuid.UUID]domain.Availability, len(known))
	for _, a := range known {
		byVariant[a.VariantID] = a
	}
	out := make([]domain.Availability, 0, len(variantIDs))
	seen := make(map[uuid.UUID]bool, len(variantIDs))
	for _, id := range variantIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if a, ok := byVariant[id]; ok {
			out = append(out, a)
			continue
		}
		out = append(out, domain.Availability{VariantID: id})
	}
	return out, nil
}
