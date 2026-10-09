package application

import (
	"context"
	"sync"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/google/uuid"
)

type fakeCategories struct {
	mu    sync.Mutex
	slugs map[string]bool
}

func newFakeCategories() *fakeCategories { return &fakeCategories{slugs: map[string]bool{}} }

func (f *fakeCategories) Create(_ context.Context, c *domain.Category) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.slugs[c.Slug] = true
	return nil
}
func (f *fakeCategories) GetByID(context.Context, uuid.UUID) (*domain.Category, error) {
	return nil, domain.ErrCategoryNotFound
}
func (f *fakeCategories) GetBySlug(context.Context, string) (*domain.Category, error) {
	return nil, domain.ErrCategoryNotFound
}
func (f *fakeCategories) List(context.Context) ([]domain.Category, error) { return nil, nil }
func (f *fakeCategories) Update(context.Context, *domain.Category) error  { return nil }
func (f *fakeCategories) Delete(context.Context, uuid.UUID) error         { return nil }
func (f *fakeCategories) SlugExists(_ context.Context, s string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.slugs[s], nil
}

// fakeStore backs both the product and variant fakes so they share state.
type fakeStore struct {
	mu       sync.Mutex
	products map[uuid.UUID]*domain.Product
}

func newFakeStore() *fakeStore { return &fakeStore{products: map[uuid.UUID]*domain.Product{}} }

func clone(p *domain.Product) *domain.Product {
	cp := *p
	cp.Images = append([]domain.Image(nil), p.Images...)
	cp.Variants = append([]domain.Variant(nil), p.Variants...)
	return &cp
}

type fakeProducts struct{ s *fakeStore }

func (f fakeProducts) Create(_ context.Context, p *domain.Product) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	f.s.products[p.ID] = clone(p)
	return nil
}
func (f fakeProducts) GetByID(_ context.Context, id uuid.UUID) (*domain.Product, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	p, ok := f.s.products[id]
	if !ok {
		return nil, domain.ErrProductNotFound
	}
	return clone(p), nil
}
func (f fakeProducts) GetBySlug(_ context.Context, slug string) (*domain.Product, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	for _, p := range f.s.products {
		if p.Slug == slug {
			return clone(p), nil
		}
	}
	return nil, domain.ErrProductNotFound
}
func (f fakeProducts) List(_ context.Context, flt domain.ProductFilter) ([]domain.ProductSummary, int, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	var out []domain.ProductSummary
	for _, p := range f.s.products {
		if flt.Status != nil && p.Status != *flt.Status {
			continue
		}
		out = append(out, domain.ProductSummary{ID: p.ID, Name: p.Name, Status: p.Status})
	}
	return out, len(out), nil
}
func (f fakeProducts) Update(_ context.Context, p *domain.Product, _ bool) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	cur := f.s.products[p.ID]
	cur.Name, cur.Status, cur.CategoryID, cur.Images = p.Name, p.Status, p.CategoryID, p.Images
	return nil
}
func (f fakeProducts) SlugExists(_ context.Context, slug string) (bool, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	for _, p := range f.s.products {
		if p.Slug == slug {
			return true, nil
		}
	}
	return false, nil
}

type fakeVariants struct{ s *fakeStore }

func (f fakeVariants) Create(_ context.Context, v *domain.Variant) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	p := f.s.products[v.ProductID]
	for _, e := range p.Variants {
		if e.SKU == v.SKU {
			return domain.ErrSKUTaken
		}
	}
	p.Variants = append(p.Variants, *v)
	return nil
}
func (f fakeVariants) GetByID(_ context.Context, id uuid.UUID) (*domain.Variant, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	for _, p := range f.s.products {
		for _, v := range p.Variants {
			if v.ID == id {
				cp := v
				return &cp, nil
			}
		}
	}
	return nil, domain.ErrVariantNotFound
}
func (f fakeVariants) Update(_ context.Context, v *domain.Variant) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	p := f.s.products[v.ProductID]
	for i := range p.Variants {
		if p.Variants[i].ID == v.ID {
			p.Variants[i] = *v
		}
	}
	return nil
}
func (f fakeVariants) CountActive(_ context.Context, productID uuid.UUID) (int, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	return f.s.products[productID].ActiveVariantCount(), nil
}
