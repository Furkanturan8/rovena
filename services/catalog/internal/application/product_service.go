package application

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/Furkanturan8/rovena/services/catalog/internal/repository"
	"github.com/google/uuid"
)

var skuPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{1,63}$`)

type ProductService struct {
	products repository.ProductRepository
	variants repository.VariantRepository
	now      func() time.Time
}

func NewProductService(products repository.ProductRepository, variants repository.VariantRepository) *ProductService {
	return &ProductService{products: products, variants: variants, now: time.Now}
}

type VariantInput struct {
	SKU        string
	Size       string
	Color      string
	PriceCents int64
}

type CreateProductInput struct {
	Name        string
	Description string
	Brand       string
	CategoryID  uuid.UUID
	Status      domain.ProductStatus // empty = draft
	Images      []domain.Image
	Variants    []VariantInput
}

func (s *ProductService) Create(ctx context.Context, in CreateProductInput) (*domain.Product, error) {
	status := in.Status
	if status == "" {
		status = domain.StatusDraft
	}
	if !status.Valid() {
		return nil, domain.ErrInvalidStatus
	}
	if status == domain.StatusActive && len(in.Variants) == 0 {
		return nil, domain.ErrNoActiveVariant
	}

	name := strings.TrimSpace(in.Name)
	slug, err := uniqueSlug(ctx, name, s.products.SlugExists)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := s.now()
	p := &domain.Product{
		ID: id, CategoryID: in.CategoryID, Name: name, Slug: slug,
		Description: strings.TrimSpace(in.Description), Brand: strings.TrimSpace(in.Brand),
		Status: status, Images: numberImages(in.Images), CreatedAt: now, UpdatedAt: now,
	}
	for _, vin := range in.Variants {
		v, err := s.newVariant(p.ID, vin, now)
		if err != nil {
			return nil, err
		}
		p.Variants = append(p.Variants, *v)
	}
	if err := s.products.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

type UpdateProductInput struct {
	Name        *string
	Description *string
	Brand       *string
	CategoryID  *uuid.UUID
	Status      *domain.ProductStatus
	Images      *[]domain.Image // non-nil replaces the whole image list
}

func (s *ProductService) Update(ctx context.Context, id uuid.UUID, in UpdateProductInput) (*domain.Product, error) {
	p, err := s.products.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		p.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		p.Description = strings.TrimSpace(*in.Description)
	}
	if in.Brand != nil {
		p.Brand = strings.TrimSpace(*in.Brand)
	}
	if in.CategoryID != nil {
		p.CategoryID = *in.CategoryID
	}
	if in.Status != nil {
		if !in.Status.Valid() {
			return nil, domain.ErrInvalidStatus
		}
		if *in.Status == domain.StatusActive && p.ActiveVariantCount() == 0 {
			return nil, domain.ErrNoActiveVariant
		}
		p.Status = *in.Status
	}
	replaceImages := in.Images != nil
	if replaceImages {
		p.Images = numberImages(*in.Images)
	}
	if err := s.products.Update(ctx, p, replaceImages); err != nil {
		return nil, err
	}
	return p, nil
}

// Archive hides a product from the storefront. Products are never deleted because
// orders and stock records keep referring to their variants.
func (s *ProductService) Archive(ctx context.Context, id uuid.UUID) error {
	archived := domain.StatusArchived
	_, err := s.Update(ctx, id, UpdateProductInput{Status: &archived})
	return err
}

func (s *ProductService) AddVariant(ctx context.Context, productID uuid.UUID, in VariantInput) (*domain.Variant, error) {
	v, err := s.newVariant(productID, in, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.variants.Create(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

type UpdateVariantInput struct {
	Size       *string
	Color      *string
	PriceCents *int64
	Active     *bool
}

// UpdateVariant applies the changes. The repository guarantees an active product keeps
// at least one active variant, even when admins change variants concurrently.
func (s *ProductService) UpdateVariant(ctx context.Context, productID, variantID uuid.UUID, in UpdateVariantInput) (*domain.Variant, error) {
	v, err := s.variants.GetByID(ctx, variantID)
	if err != nil {
		return nil, err
	}
	if v.ProductID != productID {
		return nil, domain.ErrVariantNotFound
	}
	if in.Size != nil {
		v.Size = strings.TrimSpace(*in.Size)
	}
	if in.Color != nil {
		v.Color = strings.TrimSpace(*in.Color)
	}
	if in.PriceCents != nil {
		v.PriceCents = *in.PriceCents
	}
	if in.Active != nil {
		v.Active = *in.Active
	}
	if err := s.variants.Update(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// GetPublic returns an active product by UUID or slug, with only its active variants.
func (s *ProductService) GetPublic(ctx context.Context, idOrSlug string) (*domain.Product, error) {
	p, err := s.get(ctx, idOrSlug)
	if err != nil {
		return nil, err
	}
	if p.Status != domain.StatusActive {
		return nil, domain.ErrProductNotFound
	}
	active := p.Variants[:0:0]
	for _, v := range p.Variants {
		if v.Active {
			active = append(active, v)
		}
	}
	p.Variants = active
	return p, nil
}

func (s *ProductService) GetAdmin(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	return s.products.GetByID(ctx, id)
}

func (s *ProductService) get(ctx context.Context, idOrSlug string) (*domain.Product, error) {
	if id, err := uuid.Parse(idOrSlug); err == nil {
		return s.products.GetByID(ctx, id)
	}
	return s.products.GetBySlug(ctx, idOrSlug)
}

// ListPublic only ever returns active products, whatever the caller asked for.
func (s *ProductService) ListPublic(ctx context.Context, f domain.ProductFilter) ([]domain.ProductSummary, int, error) {
	active := domain.StatusActive
	f.Status = &active
	return s.products.List(ctx, f)
}

func (s *ProductService) ListAdmin(ctx context.Context, f domain.ProductFilter) ([]domain.ProductSummary, int, error) {
	return s.products.List(ctx, f)
}

func (s *ProductService) newVariant(productID uuid.UUID, in VariantInput, now time.Time) (*domain.Variant, error) {
	sku := strings.ToUpper(strings.TrimSpace(in.SKU))
	if !skuPattern.MatchString(sku) {
		return nil, domain.ErrInvalidSKU
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	return &domain.Variant{
		ID: id, ProductID: productID, SKU: sku,
		Size: strings.TrimSpace(in.Size), Color: strings.TrimSpace(in.Color),
		PriceCents: in.PriceCents, Currency: domain.DefaultCurrency, Active: true,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func numberImages(in []domain.Image) []domain.Image {
	out := make([]domain.Image, len(in))
	for i, img := range in {
		out[i] = domain.Image{URL: strings.TrimSpace(img.URL), Alt: strings.TrimSpace(img.Alt), Position: i}
	}
	return out
}

// MaxSnapshotIDs bounds one lookup so a caller cannot ask for the whole catalog.
const MaxSnapshotIDs = 100

// VariantSnapshots answers the Order service: current price and name of the given
// variants. Unknown ids are left out of the result; the caller decides what that means.
func (s *ProductService) VariantSnapshots(ctx context.Context, ids []uuid.UUID) ([]domain.VariantSnapshot, error) {
	if len(ids) > MaxSnapshotIDs {
		return nil, domain.ErrTooManyVariants
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	unique := ids[:0:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	return s.variants.Snapshots(ctx, unique)
}
