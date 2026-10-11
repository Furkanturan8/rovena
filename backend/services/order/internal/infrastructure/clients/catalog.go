package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/application"
	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/google/uuid"
)

type Catalog struct{ base }

func NewCatalog(url, internalKey string, timeout time.Duration) *Catalog {
	return &Catalog{newBase(url, internalKey, timeout)}
}

var _ application.Catalog = (*Catalog)(nil)

type variantDTO struct {
	VariantID   uuid.UUID `json:"variant_id"`
	ProductID   uuid.UUID `json:"product_id"`
	ProductName string    `json:"product_name"`
	SKU         string    `json:"sku"`
	Size        string    `json:"size"`
	Color       string    `json:"color"`
	PriceCents  int64     `json:"price_cents"`
	Currency    string    `json:"currency"`
	Sellable    bool      `json:"sellable"`
}

func (c *Catalog) Lookup(ctx context.Context, variantIDs []uuid.UUID) ([]application.VariantInfo, error) {
	ids := make([]string, len(variantIDs))
	for i, id := range variantIDs {
		ids[i] = id.String()
	}
	status, body, err := c.do(ctx, http.MethodPost, "/api/v1/internal/variants/lookup", map[string]any{"variant_ids": ids})
	if err != nil {
		return nil, fmt.Errorf("%w: catalog: %v", domain.ErrDependencyUnavailable, err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%w: %v", domain.ErrDependencyUnavailable, unexpected("catalog", status, body))
	}
	var env struct {
		Data []variantDTO `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("%w: catalog sent unreadable data: %v", domain.ErrDependencyUnavailable, err)
	}
	out := make([]application.VariantInfo, len(env.Data))
	for i, v := range env.Data {
		out[i] = application.VariantInfo{
			VariantID: v.VariantID, ProductID: v.ProductID, ProductName: v.ProductName, SKU: v.SKU,
			Size: v.Size, Color: v.Color, PriceCents: v.PriceCents, Currency: v.Currency, Sellable: v.Sellable,
		}
	}
	return out, nil
}
