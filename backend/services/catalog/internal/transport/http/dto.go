package http

import (
	"encoding/json"
	"time"

	"github.com/Furkanturan8/rovena/services/catalog/internal/application"
	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
)

// ---- requests ----

type ImageInput struct {
	URL string `json:"url" validate:"required,http_url,max=2048"`
	Alt string `json:"alt" validate:"max=200"`
}

func toImages(in []ImageInput) []domain.Image {
	out := make([]domain.Image, len(in))
	for i, img := range in {
		out[i] = domain.Image{URL: img.URL, Alt: img.Alt}
	}
	return out
}

type VariantRequest struct {
	SKU        string `json:"sku" validate:"required,min=2,max=64"`
	Size       string `json:"size" validate:"max=32"`
	Color      string `json:"color" validate:"max=32"`
	PriceCents int64  `json:"price_cents" validate:"required,gt=0"`
}

func (r VariantRequest) toInput() application.VariantInput {
	return application.VariantInput{SKU: r.SKU, Size: r.Size, Color: r.Color, PriceCents: r.PriceCents}
}

type CreateProductRequest struct {
	Name         string           `json:"name" validate:"required,min=2,max=200"`
	Slug         string           `json:"slug" validate:"omitempty,max=80"`
	Translations json.RawMessage  `json:"translations"`
	Attributes   json.RawMessage  `json:"attributes"`
	Description  string           `json:"description" validate:"max=10000"`
	Brand        string           `json:"brand" validate:"max=100"`
	CategoryID   string           `json:"category_id" validate:"required,uuid"`
	Status       string           `json:"status" validate:"omitempty,oneof=draft active archived"`
	Images       []ImageInput     `json:"images" validate:"max=10,dive"`
	Variants     []VariantRequest `json:"variants" validate:"max=100,dive"`
}

type UpdateProductRequest struct {
	Translations json.RawMessage `json:"translations"`
	Attributes   json.RawMessage `json:"attributes"`
	Name         *string         `json:"name" validate:"omitempty,min=2,max=200"`
	Description  *string         `json:"description" validate:"omitempty,max=10000"`
	Brand        *string         `json:"brand" validate:"omitempty,max=100"`
	CategoryID   *string         `json:"category_id" validate:"omitempty,uuid"`
	Status       *string         `json:"status" validate:"omitempty,oneof=draft active archived"`
	Images       *[]ImageInput   `json:"images" validate:"omitempty,max=10,dive"`
}

type UpdateVariantRequest struct {
	Size       *string `json:"size" validate:"omitempty,max=32"`
	Color      *string `json:"color" validate:"omitempty,max=32"`
	PriceCents *int64  `json:"price_cents" validate:"omitempty,gt=0"`
	Active     *bool   `json:"active"`
}

type CreateCategoryRequest struct {
	Name         string          `json:"name" validate:"required,min=2,max=100"`
	Slug         string          `json:"slug" validate:"omitempty,max=80"`
	Translations json.RawMessage `json:"translations"`
	Description  string          `json:"description" validate:"max=1000"`
	ParentID     *string         `json:"parent_id" validate:"omitempty,uuid"`
}

type UpdateCategoryRequest struct {
	Translations json.RawMessage `json:"translations"`
	Name         *string         `json:"name" validate:"omitempty,min=2,max=100"`
	Description  *string         `json:"description" validate:"omitempty,max=1000"`
}

// ---- responses ----

type CategoryResponse struct {
	ID           string          `json:"id"`
	ParentID     *string         `json:"parent_id"`
	Name         string          `json:"name"`
	Slug         string          `json:"slug"`
	Description  string          `json:"description"`
	Translations json.RawMessage `json:"translations"`
}

func ToCategoryResponse(c *domain.Category) CategoryResponse {
	r := CategoryResponse{ID: c.ID.String(), Name: c.Name, Slug: c.Slug, Description: c.Description, Translations: orEmptyObject(c.Translations)}
	if c.ParentID != nil {
		s := c.ParentID.String()
		r.ParentID = &s
	}
	return r
}

type ImageResponse struct {
	URL string `json:"url"`
	Alt string `json:"alt"`
}

type VariantResponse struct {
	ID         string `json:"id"`
	SKU        string `json:"sku"`
	Size       string `json:"size"`
	Color      string `json:"color"`
	PriceCents int64  `json:"price_cents"`
	Currency   string `json:"currency"`
	Active     bool   `json:"active"`
}

func ToVariantResponse(v *domain.Variant) VariantResponse {
	return VariantResponse{
		ID: v.ID.String(), SKU: v.SKU, Size: v.Size, Color: v.Color,
		PriceCents: v.PriceCents, Currency: v.Currency, Active: v.Active,
	}
}

type ProductResponse struct {
	ID           string            `json:"id"`
	CategoryID   string            `json:"category_id"`
	Name         string            `json:"name"`
	Slug         string            `json:"slug"`
	Description  string            `json:"description"`
	Brand        string            `json:"brand"`
	Status       string            `json:"status"`
	Translations json.RawMessage   `json:"translations"`
	Attributes   json.RawMessage   `json:"attributes"`
	Images       []ImageResponse   `json:"images"`
	Variants     []VariantResponse `json:"variants"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

func ToProductResponse(p *domain.Product) ProductResponse {
	images := make([]ImageResponse, len(p.Images))
	for i, img := range p.Images {
		images[i] = ImageResponse{URL: img.URL, Alt: img.Alt}
	}
	variants := make([]VariantResponse, len(p.Variants))
	for i := range p.Variants {
		variants[i] = ToVariantResponse(&p.Variants[i])
	}
	return ProductResponse{
		ID: p.ID.String(), CategoryID: p.CategoryID.String(), Name: p.Name, Slug: p.Slug,
		Description: p.Description, Brand: p.Brand, Status: string(p.Status),
		Translations: orEmptyObject(p.Translations), Attributes: orEmptyObject(p.Attributes),
		Images: images, Variants: variants, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

type ProductSummaryResponse struct {
	ID             string `json:"id"`
	CategoryID     string `json:"category_id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Description    string `json:"description"`
	Brand          string `json:"brand"`
	Status         string `json:"status"`
	PriceFromCents *int64 `json:"price_from_cents"`
	Currency       string `json:"currency"`
	ThumbnailURL   string `json:"thumbnail_url"`
	// Everything a listing card needs, so the storefront does not call once per product.
	Translations json.RawMessage   `json:"translations"`
	Attributes   json.RawMessage   `json:"attributes"`
	Images       []ImageResponse   `json:"images"`
	Variants     []VariantResponse `json:"variants"`
}

func ToSummaryResponse(s domain.ProductSummary) ProductSummaryResponse {
	images := make([]ImageResponse, len(s.Images))
	for i, img := range s.Images {
		images[i] = ImageResponse{URL: img.URL, Alt: img.Alt}
	}
	variants := make([]VariantResponse, len(s.Variants))
	for i := range s.Variants {
		variants[i] = ToVariantResponse(&s.Variants[i])
	}
	return ProductSummaryResponse{
		Translations: orEmptyObject(s.Translations), Attributes: orEmptyObject(s.Attributes),
		Images: images, Variants: variants,
		ID: s.ID.String(), CategoryID: s.CategoryID.String(), Name: s.Name, Slug: s.Slug, Description: s.Description,
		Brand: s.Brand, Status: string(s.Status), PriceFromCents: s.PriceFromCents,
		Currency: s.Currency, ThumbnailURL: s.ThumbnailURL,
	}
}

type LookupVariantsRequest struct {
	VariantIDs []string `json:"variant_ids" validate:"required,min=1,max=100,dive,uuid"`
}

// VariantSnapshotResponse is what the Order service stores in a cart view or an order line.
type VariantSnapshotResponse struct {
	VariantID   string `json:"variant_id"`
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	SKU         string `json:"sku"`
	Size        string `json:"size"`
	Color       string `json:"color"`
	PriceCents  int64  `json:"price_cents"`
	Currency    string `json:"currency"`
	Sellable    bool   `json:"sellable"`
}

func ToVariantSnapshotResponse(s domain.VariantSnapshot) VariantSnapshotResponse {
	return VariantSnapshotResponse{
		VariantID: s.ID.String(), ProductID: s.ProductID.String(), ProductName: s.ProductName,
		SKU: s.SKU, Size: s.Size, Color: s.Color, PriceCents: s.PriceCents,
		Currency: s.Currency, Sellable: s.Sellable(),
	}
}

func orEmptyObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
