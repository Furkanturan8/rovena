package postgres

import (
	"encoding/json"
	"time"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type categoryRow struct {
	bun.BaseModel `bun:"table:categories,alias:c"`

	ID           uuid.UUID       `bun:",pk,type:uuid"`
	ParentID     *uuid.UUID      `bun:",type:uuid,nullzero"`
	Name         string          `bun:",notnull"`
	Slug         string          `bun:",notnull"`
	Description  string          `bun:",notnull"`
	Translations json.RawMessage `bun:"translations,type:jsonb,notnull"`
	CreatedAt    time.Time       `bun:",notnull"`
	UpdatedAt    time.Time       `bun:",notnull"`
}

func (r *categoryRow) toDomain() *domain.Category {
	return &domain.Category{
		ID: r.ID, ParentID: r.ParentID, Name: r.Name, Slug: r.Slug,
		Description: r.Description, Translations: r.Translations, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func categoryRowFrom(c *domain.Category) *categoryRow {
	return &categoryRow{
		ID: c.ID, ParentID: c.ParentID, Name: c.Name, Slug: c.Slug,
		Description: c.Description, Translations: objectOrEmpty(c.Translations), CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

// objectOrEmpty keeps the NOT NULL jsonb columns valid for domain values built without translations.
func objectOrEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

func searchTextOf(p *domain.Product) string {
	parts := append([]string{p.Name, p.Brand}, domain.TranslationTerms(p.Translations)...)
	return domain.SearchText(parts...)
}

type productRow struct {
	bun.BaseModel `bun:"table:products,alias:p"`

	ID           uuid.UUID       `bun:",pk,type:uuid"`
	CategoryID   uuid.UUID       `bun:",type:uuid,notnull"`
	Name         string          `bun:",notnull"`
	Slug         string          `bun:",notnull"`
	Description  string          `bun:",notnull"`
	Brand        string          `bun:",notnull"`
	Status       string          `bun:",notnull"`
	SearchText   string          `bun:",notnull"`
	Translations json.RawMessage `bun:"translations,type:jsonb,notnull"`
	Attributes   json.RawMessage `bun:"attributes,type:jsonb,notnull"`
	CreatedAt    time.Time       `bun:",notnull"`
	UpdatedAt    time.Time       `bun:",notnull"`
}

func productRowFrom(p *domain.Product) *productRow {
	return &productRow{
		ID: p.ID, CategoryID: p.CategoryID, Name: p.Name, Slug: p.Slug,
		Description: p.Description, Brand: p.Brand, Status: string(p.Status),
		SearchText:   searchTextOf(p),
		Translations: objectOrEmpty(p.Translations), Attributes: objectOrEmpty(p.Attributes),
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func (r *productRow) toDomain() *domain.Product {
	return &domain.Product{
		ID: r.ID, CategoryID: r.CategoryID, Name: r.Name, Slug: r.Slug,
		Description: r.Description, Brand: r.Brand, Status: domain.ProductStatus(r.Status),
		Translations: r.Translations, Attributes: r.Attributes,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type imageRow struct {
	bun.BaseModel `bun:"table:product_images,alias:i"`

	ID        uuid.UUID `bun:",pk,type:uuid"`
	ProductID uuid.UUID `bun:",type:uuid,notnull"`
	URL       string    `bun:"url,notnull"`
	Alt       string    `bun:",notnull"`
	Position  int       `bun:",notnull"`
}

type variantRow struct {
	bun.BaseModel `bun:"table:variants,alias:v"`

	ID         uuid.UUID `bun:",pk,type:uuid"`
	ProductID  uuid.UUID `bun:",type:uuid,notnull"`
	SKU        string    `bun:"sku,notnull"`
	Size       string    `bun:",notnull"`
	Color      string    `bun:",notnull"`
	PriceCents int64     `bun:",notnull"`
	Currency   string    `bun:",notnull"`
	Active     bool      `bun:",notnull"`
	CreatedAt  time.Time `bun:",notnull"`
	UpdatedAt  time.Time `bun:",notnull"`
}

func variantRowFrom(v *domain.Variant) *variantRow {
	return &variantRow{
		ID: v.ID, ProductID: v.ProductID, SKU: v.SKU, Size: v.Size, Color: v.Color,
		PriceCents: v.PriceCents, Currency: v.Currency, Active: v.Active,
		CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}

func (r *variantRow) toDomain() *domain.Variant {
	return &domain.Variant{
		ID: r.ID, ProductID: r.ProductID, SKU: r.SKU, Size: r.Size, Color: r.Color,
		PriceCents: r.PriceCents, Currency: r.Currency, Active: r.Active,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// summaryRow is scanned from the listing query; price_from is a computed column.
type summaryRow struct {
	bun.BaseModel `bun:"table:products,alias:p"`

	ID           uuid.UUID `bun:",pk,type:uuid"`
	CategoryID   uuid.UUID `bun:",type:uuid"`
	Name         string
	Slug         string
	Description  string
	Brand        string
	Status       string
	Translations json.RawMessage `bun:"translations,type:jsonb"`
	Attributes   json.RawMessage `bun:"attributes,type:jsonb"`
	PriceFrom    *int64          `bun:"price_from,scanonly"`
}
