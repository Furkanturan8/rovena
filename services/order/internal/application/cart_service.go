package application

import (
	"context"

	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/services/order/internal/repository"
	"github.com/google/uuid"
)

type CartService struct {
	repo    repository.OrderRepository
	catalog Catalog
}

func NewCartService(repo repository.OrderRepository, catalog Catalog) *CartService {
	return &CartService{repo: repo, catalog: catalog}
}

type CartLineView struct {
	VariantID      uuid.UUID
	ProductName    string
	SKU            string
	Size           string
	Color          string
	UnitPriceCents int64
	Quantity       int
	LineTotalCents int64
	// Available is false when the variant was deactivated or removed since it was added.
	Available bool
}

// CartView is the cart priced with Catalog's current prices. The totals count only
// lines that can still be bought.
type CartView struct {
	Lines      []CartLineView
	TotalCents int64
	Currency   string
}

func (s *CartService) Get(ctx context.Context, customerID uuid.UUID) (*CartView, error) {
	items, err := s.repo.GetCart(ctx, customerID)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, items)
}

// SetItem sets the quantity of a variant, adding the line if needed. The variant must
// exist and be on sale; its price is never taken from the client.
func (s *CartService) SetItem(ctx context.Context, customerID, variantID uuid.UUID, quantity int) (*CartView, error) {
	if quantity < 1 || quantity > domain.MaxLineQuantity {
		return nil, domain.ErrInvalidQuantity
	}
	infos, err := s.catalog.Lookup(ctx, []uuid.UUID{variantID})
	if err != nil {
		return nil, err
	}
	if len(infos) != 1 || !infos[0].Sellable {
		return nil, &domain.ItemError{VariantID: variantID, Err: domain.ErrItemUnavailable}
	}
	if err := s.repo.SetCartItem(ctx, customerID, domain.CartItem{VariantID: variantID, Quantity: quantity}); err != nil {
		return nil, err
	}
	return s.Get(ctx, customerID)
}

func (s *CartService) RemoveItem(ctx context.Context, customerID, variantID uuid.UUID) (*CartView, error) {
	if err := s.repo.RemoveCartItem(ctx, customerID, variantID); err != nil {
		return nil, err
	}
	return s.Get(ctx, customerID)
}

func (s *CartService) Clear(ctx context.Context, customerID uuid.UUID) error {
	return s.repo.ClearCart(ctx, customerID)
}

func (s *CartService) view(ctx context.Context, items []domain.CartItem) (*CartView, error) {
	view := &CartView{Lines: []CartLineView{}}
	if len(items) == 0 {
		return view, nil
	}
	infos, err := lookupAll(ctx, s.catalog, items)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		line := CartLineView{VariantID: it.VariantID, Quantity: it.Quantity}
		if info, ok := infos[it.VariantID]; ok {
			line.ProductName, line.SKU, line.Size, line.Color = info.ProductName, info.SKU, info.Size, info.Color
			line.UnitPriceCents = info.PriceCents
			line.LineTotalCents = info.PriceCents * int64(it.Quantity)
			line.Available = info.Sellable
			if info.Sellable {
				if view.Currency == "" {
					view.Currency = info.Currency
				}
				view.TotalCents += line.LineTotalCents
			}
		}
		view.Lines = append(view.Lines, line)
	}
	return view, nil
}

func lookupAll(ctx context.Context, catalog Catalog, items []domain.CartItem) (map[uuid.UUID]VariantInfo, error) {
	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.VariantID
	}
	infos, err := catalog.Lookup(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]VariantInfo, len(infos))
	for _, info := range infos {
		out[info.VariantID] = info
	}
	return out, nil
}
