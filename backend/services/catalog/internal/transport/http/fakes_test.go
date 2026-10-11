package http_test

import (
	"context"
	"sync"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/google/uuid"
)

// In-memory repositories. Error fields let a test force a repository failure.

type store struct {
	mu         sync.Mutex
	products   map[uuid.UUID]*domain.Product
	categories map[uuid.UUID]*domain.Category

	lastFilter domain.ProductFilter
	listCalls  int

	createProductErr  error
	deleteCategoryErr error
	addVariantErr     error
}

func newStore() *store {
	return &store{products: map[uuid.UUID]*domain.Product{}, categories: map[uuid.UUID]*domain.Category{}}
}

func cloneProduct(p *domain.Product) *domain.Product {
	cp := *p
	cp.Images = append([]domain.Image(nil), p.Images...)
	cp.Variants = append([]domain.Variant(nil), p.Variants...)
	return &cp
}

// ---- categories ----

type categoryRepo struct{ s *store }

func (r categoryRepo) Create(_ context.Context, c *domain.Category) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if c.ParentID != nil {
		if _, ok := r.s.categories[*c.ParentID]; !ok {
			return domain.ErrCategoryNotFound
		}
	}
	cp := *c
	r.s.categories[c.ID] = &cp
	return nil
}

func (r categoryRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Category, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	c, ok := r.s.categories[id]
	if !ok {
		return nil, domain.ErrCategoryNotFound
	}
	cp := *c
	return &cp, nil
}

func (r categoryRepo) GetBySlug(_ context.Context, slug string) (*domain.Category, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, c := range r.s.categories {
		if c.Slug == slug {
			cp := *c
			return &cp, nil
		}
	}
	return nil, domain.ErrCategoryNotFound
}

func (r categoryRepo) List(context.Context) ([]domain.Category, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	out := make([]domain.Category, 0, len(r.s.categories))
	for _, c := range r.s.categories {
		out = append(out, *c)
	}
	return out, nil
}

func (r categoryRepo) Update(_ context.Context, c *domain.Category) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	cur, ok := r.s.categories[c.ID]
	if !ok {
		return domain.ErrCategoryNotFound
	}
	cur.Name, cur.Description = c.Name, c.Description
	return nil
}

func (r categoryRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if r.s.deleteCategoryErr != nil {
		return r.s.deleteCategoryErr
	}
	if _, ok := r.s.categories[id]; !ok {
		return domain.ErrCategoryNotFound
	}
	delete(r.s.categories, id)
	return nil
}

func (r categoryRepo) SlugExists(_ context.Context, slug string) (bool, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, c := range r.s.categories {
		if c.Slug == slug {
			return true, nil
		}
	}
	return false, nil
}

// ---- products ----

type productRepo struct{ s *store }

func (r productRepo) Create(_ context.Context, p *domain.Product) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if r.s.createProductErr != nil {
		return r.s.createProductErr
	}
	r.s.products[p.ID] = cloneProduct(p)
	return nil
}

func (r productRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Product, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.products[id]
	if !ok {
		return nil, domain.ErrProductNotFound
	}
	return cloneProduct(p), nil
}

func (r productRepo) GetBySlug(_ context.Context, slug string) (*domain.Product, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, p := range r.s.products {
		if p.Slug == slug {
			return cloneProduct(p), nil
		}
	}
	return nil, domain.ErrProductNotFound
}

func (r productRepo) List(_ context.Context, f domain.ProductFilter) ([]domain.ProductSummary, int, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.lastFilter = f
	r.s.listCalls++
	var out []domain.ProductSummary
	for _, p := range r.s.products {
		if f.Status != nil && p.Status != *f.Status {
			continue
		}
		sum := domain.ProductSummary{
			ID: p.ID, CategoryID: p.CategoryID, Name: p.Name, Slug: p.Slug, Brand: p.Brand,
			Status: p.Status, Currency: domain.DefaultCurrency,
		}
		for _, v := range p.Variants {
			if v.Active && (sum.PriceFromCents == nil || v.PriceCents < *sum.PriceFromCents) {
				price := v.PriceCents
				sum.PriceFromCents = &price
			}
		}
		if len(p.Images) > 0 {
			sum.ThumbnailURL = p.Images[0].URL
		}
		out = append(out, sum)
	}
	return out, len(out), nil
}

func (r productRepo) Update(_ context.Context, p *domain.Product, replaceImages bool) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	cur, ok := r.s.products[p.ID]
	if !ok {
		return domain.ErrProductNotFound
	}
	cur.Name, cur.Description, cur.Brand = p.Name, p.Description, p.Brand
	cur.CategoryID, cur.Status = p.CategoryID, p.Status
	if replaceImages {
		cur.Images = p.Images
	}
	return nil
}

func (r productRepo) SlugExists(_ context.Context, slug string) (bool, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, p := range r.s.products {
		if p.Slug == slug {
			return true, nil
		}
	}
	return false, nil
}

// ---- variants ----

type variantRepo struct{ s *store }

func (r variantRepo) Create(_ context.Context, v *domain.Variant) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if r.s.addVariantErr != nil {
		return r.s.addVariantErr
	}
	p, ok := r.s.products[v.ProductID]
	if !ok {
		return domain.ErrProductNotFound
	}
	p.Variants = append(p.Variants, *v)
	return nil
}

func (r variantRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Variant, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, p := range r.s.products {
		for _, v := range p.Variants {
			if v.ID == id {
				cp := v
				return &cp, nil
			}
		}
	}
	return nil, domain.ErrVariantNotFound
}

func (r variantRepo) Snapshots(_ context.Context, ids []uuid.UUID) ([]domain.VariantSnapshot, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []domain.VariantSnapshot
	for _, id := range ids {
		for _, p := range r.s.products {
			for _, v := range p.Variants {
				if v.ID == id {
					out = append(out, domain.VariantSnapshot{Variant: v, ProductName: p.Name, ProductStatus: p.Status})
				}
			}
		}
	}
	return out, nil
}

func (r variantRepo) Update(_ context.Context, v *domain.Variant) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.products[v.ProductID]
	if !ok {
		return domain.ErrProductNotFound
	}
	for i := range p.Variants {
		if p.Variants[i].ID == v.ID {
			p.Variants[i] = *v
			return nil
		}
	}
	return domain.ErrVariantNotFound
}
