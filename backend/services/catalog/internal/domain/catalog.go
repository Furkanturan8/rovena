package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// DefaultCurrency is the only currency supported for now. Prices are stored in
// minor units (kuruş) to avoid floating point rounding.
const DefaultCurrency = "TRY"

type ProductStatus string

const (
	StatusDraft    ProductStatus = "draft"
	StatusActive   ProductStatus = "active"
	StatusArchived ProductStatus = "archived"
)

func (s ProductStatus) Valid() bool {
	return s == StatusDraft || s == StatusActive || s == StatusArchived
}

type Category struct {
	ID          uuid.UUID
	ParentID    *uuid.UUID
	Name        string
	Slug        string
	Description string
	// Translations holds localized name/description by language code ({"en": {"name": "Coats"}}).
	Translations json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Image struct {
	URL      string
	Alt      string
	Position int
}

type Variant struct {
	ID         uuid.UUID
	ProductID  uuid.UUID
	SKU        string
	Size       string
	Color      string
	PriceCents int64
	Currency   string
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// VariantSnapshot is what a buyer's order needs to remember about a variant: the
// variant itself plus the product facts that decide whether it can be sold.
type VariantSnapshot struct {
	Variant
	ProductName   string
	ProductStatus ProductStatus
}

// Sellable reports whether a new order may contain the variant.
func (s VariantSnapshot) Sellable() bool {
	return s.Active && s.ProductStatus == StatusActive
}

type Product struct {
	ID          uuid.UUID
	CategoryID  uuid.UUID
	Name        string
	Slug        string
	Description string
	Brand       string
	Status      ProductStatus
	// Translations: localized texts by language code. Attributes: language-independent storefront facts.
	// Both are JSON objects the storefront interprets; the catalog only stores them.
	Translations json.RawMessage
	Attributes   json.RawMessage
	Images       []Image
	Variants     []Variant
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (p *Product) ActiveVariantCount() int {
	n := 0
	for _, v := range p.Variants {
		if v.Active {
			n++
		}
	}
	return n
}

// ProductSummary is the lightweight shape used in listings.
type ProductSummary struct {
	ID             uuid.UUID
	CategoryID     uuid.UUID
	Name           string
	Slug           string
	Description    string
	Brand          string
	Status         ProductStatus
	PriceFromCents *int64
	Currency       string
	ThumbnailURL   string
	Translations   json.RawMessage
	Attributes     json.RawMessage
	Images         []Image
	Variants       []Variant // active variants only
}

type ProductSort string

const (
	SortNewest    ProductSort = "newest"
	SortPriceAsc  ProductSort = "price_asc"
	SortPriceDesc ProductSort = "price_desc"
	SortName      ProductSort = "name"
)

func (s ProductSort) Valid() bool {
	switch s {
	case SortNewest, SortPriceAsc, SortPriceDesc, SortName:
		return true
	}
	return false
}

type ProductFilter struct {
	Query        string
	CategorySlug string // includes all descendant categories
	Size         string
	Color        string
	MinPrice     *int64
	MaxPrice     *int64
	Status       *ProductStatus // nil = any status (admin only)
	Sort         ProductSort
	Page         int
	PageSize     int
}

var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrCategoryInUse    = errors.New("category has products or subcategories")
	ErrProductNotFound  = errors.New("product not found")
	ErrVariantNotFound  = errors.New("variant not found")
	ErrTooManyVariants  = errors.New("too many variants in one lookup")
	ErrSlugTaken        = errors.New("slug already in use")
	ErrSKUTaken         = errors.New("sku already in use")
	ErrVariantExists    = errors.New("variant with this size and color already exists")
	ErrInvalidSKU       = errors.New("invalid sku")
	ErrInvalidSlug      = errors.New("invalid slug")
	ErrInvalidJSON      = errors.New("translations and attributes must be JSON objects")
	ErrInvalidStatus    = errors.New("invalid status")
	ErrNoActiveVariant  = errors.New("product needs at least one active variant")
)
