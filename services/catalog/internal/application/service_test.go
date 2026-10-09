package application

import (
	"context"
	"errors"
	"testing"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/google/uuid"
)

func newProductService() *ProductService {
	s := newFakeStore()
	return NewProductService(fakeProducts{s}, fakeVariants{s})
}

func tee(variants ...VariantInput) CreateProductInput {
	return CreateProductInput{Name: "Basic Tişört", CategoryID: uuid.New(), Variants: variants}
}

func TestCategoryCreate_UniqueSlugs(t *testing.T) {
	svc := NewCategoryService(newFakeCategories())
	ctx := context.Background()

	a, err := svc.Create(ctx, CreateCategoryInput{Name: "Çanta"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Create(ctx, CreateCategoryInput{Name: "Çanta"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Slug != "canta" || b.Slug != "canta-2" {
		t.Errorf("slugs = %q, %q; want canta, canta-2", a.Slug, b.Slug)
	}
}

func TestProductCreate_DefaultsToDraftAndNormalizesSKU(t *testing.T) {
	svc := newProductService()
	p, err := svc.Create(context.Background(), tee(VariantInput{SKU: " tee-blk-m ", Size: "M", PriceCents: 1000}))
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.StatusDraft {
		t.Errorf("status = %q, want draft", p.Status)
	}
	if p.Slug != "basic-tisort" {
		t.Errorf("slug = %q", p.Slug)
	}
	if p.Variants[0].SKU != "TEE-BLK-M" || !p.Variants[0].Active {
		t.Errorf("variant = %+v", p.Variants[0])
	}
}

func TestProductCreate_Validation(t *testing.T) {
	svc := newProductService()
	ctx := context.Background()

	in := tee()
	in.Status = domain.StatusActive
	if _, err := svc.Create(ctx, in); !errors.Is(err, domain.ErrNoActiveVariant) {
		t.Errorf("active without variants err = %v", err)
	}
	if _, err := svc.Create(ctx, tee(VariantInput{SKU: "bad sku!", PriceCents: 1})); !errors.Is(err, domain.ErrInvalidSKU) {
		t.Errorf("bad sku err = %v", err)
	}
	in = tee()
	in.Status = "published"
	if _, err := svc.Create(ctx, in); !errors.Is(err, domain.ErrInvalidStatus) {
		t.Errorf("bad status err = %v", err)
	}
}

func TestPublish_RequiresActiveVariant(t *testing.T) {
	svc := newProductService()
	ctx := context.Background()
	p, _ := svc.Create(ctx, tee())

	active := domain.StatusActive
	if _, err := svc.Update(ctx, p.ID, UpdateProductInput{Status: &active}); !errors.Is(err, domain.ErrNoActiveVariant) {
		t.Fatalf("publish without variant err = %v", err)
	}
	if _, err := svc.AddVariant(ctx, p.ID, VariantInput{SKU: "TEE-1", PriceCents: 1000}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, p.ID, UpdateProductInput{Status: &active}); err != nil {
		t.Fatalf("publish with variant: %v", err)
	}
}

func TestUpdateVariant_WrongProduct(t *testing.T) {
	svc := newProductService()
	ctx := context.Background()
	a, _ := svc.Create(ctx, tee(VariantInput{SKU: "A-1", PriceCents: 1}))
	b, _ := svc.Create(ctx, tee(VariantInput{SKU: "B-1", PriceCents: 1}))

	price := int64(5)
	_, err := svc.UpdateVariant(ctx, b.ID, a.Variants[0].ID, UpdateVariantInput{PriceCents: &price})
	if !errors.Is(err, domain.ErrVariantNotFound) {
		t.Fatalf("err = %v, want ErrVariantNotFound", err)
	}
}

func TestPublicVisibility(t *testing.T) {
	svc := newProductService()
	ctx := context.Background()

	draft, _ := svc.Create(ctx, tee(VariantInput{SKU: "D-1", PriceCents: 1}))
	if _, err := svc.GetPublic(ctx, draft.ID.String()); !errors.Is(err, domain.ErrProductNotFound) {
		t.Errorf("draft visible publicly, err = %v", err)
	}

	in := tee(VariantInput{SKU: "P-1", PriceCents: 1}, VariantInput{SKU: "P-2", Size: "L", PriceCents: 1})
	in.Name = "Pantolon"
	in.Status = domain.StatusActive
	live, err := svc.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	off := false
	if _, err := svc.UpdateVariant(ctx, live.ID, live.Variants[1].ID, UpdateVariantInput{Active: &off}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetPublic(ctx, "pantolon") // by slug
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Variants) != 1 || got.Variants[0].SKU != "P-1" {
		t.Errorf("public variants = %+v, want only the active one", got.Variants)
	}

	items, _, _ := svc.ListPublic(ctx, domain.ProductFilter{})
	if len(items) != 1 {
		t.Errorf("public list has %d items, want 1 (draft excluded)", len(items))
	}
}

func TestArchive(t *testing.T) {
	svc := newProductService()
	ctx := context.Background()
	in := tee(VariantInput{SKU: "A-1", PriceCents: 1})
	in.Status = domain.StatusActive
	p, _ := svc.Create(ctx, in)

	if err := svc.Archive(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetPublic(ctx, p.ID.String()); !errors.Is(err, domain.ErrProductNotFound) {
		t.Errorf("archived product still public, err = %v", err)
	}
	admin, err := svc.GetAdmin(ctx, p.ID)
	if err != nil || admin.Status != domain.StatusArchived {
		t.Errorf("admin view = %+v, err = %v", admin, err)
	}
}
