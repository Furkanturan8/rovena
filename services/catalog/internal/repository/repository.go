package repository

import (
	"context"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/google/uuid"
)

type CategoryRepository interface {
	// Create returns ErrSlugTaken on slug conflict and ErrCategoryNotFound when the parent does not exist.
	Create(ctx context.Context, c *domain.Category) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Category, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Category, error)
	List(ctx context.Context) ([]domain.Category, error)
	Update(ctx context.Context, c *domain.Category) error
	// Delete returns ErrCategoryInUse when products or subcategories reference it.
	Delete(ctx context.Context, id uuid.UUID) error
	SlugExists(ctx context.Context, slug string) (bool, error)
}

type ProductRepository interface {
	// Create stores the product with its images and variants in one transaction.
	// Returns ErrSlugTaken, ErrSKUTaken, ErrVariantExists, ErrCategoryNotFound.
	Create(ctx context.Context, p *domain.Product) error
	// GetByID/GetBySlug return the product with all images and variants (active or not).
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Product, error)
	List(ctx context.Context, f domain.ProductFilter) ([]domain.ProductSummary, int, error)
	// Update persists the scalar fields; images are replaced only when replaceImages is true.
	// It locks the product row and returns ErrNoActiveVariant when an active product
	// would be left without an active variant.
	Update(ctx context.Context, p *domain.Product, replaceImages bool) error
	SlugExists(ctx context.Context, slug string) (bool, error)
}

type VariantRepository interface {
	// Create returns ErrSKUTaken, ErrVariantExists, or ErrProductNotFound (unknown product).
	Create(ctx context.Context, v *domain.Variant) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Variant, error)
	// Update locks the parent product row, so concurrent changes to one product are
	// serialized, and returns ErrNoActiveVariant when it would deactivate the last
	// active variant of an active product.
	Update(ctx context.Context, v *domain.Variant) error
}
