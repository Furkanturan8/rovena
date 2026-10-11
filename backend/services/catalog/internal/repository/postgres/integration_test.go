//go:build integration

package postgres_test

// Runs against a real PostgreSQL with the catalog schema applied:
//   CATALOG_TEST_DATABASE_URL=postgres://... go test -tags integration ./internal/repository/postgres
// Every test creates its own uniquely named rows and removes them afterwards.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/Furkanturan8/rovena/services/catalog/internal/application"
	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/Furkanturan8/rovena/services/catalog/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/catalog/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type harness struct {
	ctx        context.Context
	db         *bun.DB
	categories *application.CategoryService
	products   *application.ProductService
	variants   *postgres.VariantRepository
	categoryID uuid.UUID
	root       *domain.Category
}

func setup(t *testing.T) *harness {
	t.Helper()
	dsn := os.Getenv("CATALOG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CATALOG_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	h := &harness{
		ctx:        ctx,
		db:         db,
		categories: application.NewCategoryService(postgres.NewCategoryRepository(db)),
		variants:   postgres.NewVariantRepository(db),
	}
	h.products = application.NewProductService(postgres.NewProductRepository(db), h.variants)

	h.root = h.newCategory(t, nil)
	h.categoryID = h.root.ID
	return h
}

// newCategory creates a uniquely named category and removes it (and its products)
// when the test ends. Cleanups run LIFO, so children are removed before parents.
func (h *harness) newCategory(t *testing.T, parent *domain.Category) *domain.Category {
	t.Helper()
	in := application.CreateCategoryInput{Name: "itest-" + uuid.NewString()}
	if parent != nil {
		in.ParentID = &parent.ID
	}
	cat, err := h.categories.Create(h.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = h.db.ExecContext(h.ctx, "DELETE FROM products WHERE category_id = ?", cat.ID)
		_ = h.categories.Delete(h.ctx, cat.ID)
	})
	return cat
}

func (h *harness) activeProduct(t *testing.T, skuPrefix string, variants int) *domain.Product {
	t.Helper()
	in := application.CreateProductInput{
		Name: "itest " + skuPrefix, CategoryID: h.categoryID, Status: domain.StatusActive,
	}
	for i := 0; i < variants; i++ {
		in.Variants = append(in.Variants, application.VariantInput{
			SKU: fmt.Sprintf("%s-%d", skuPrefix, i), Size: fmt.Sprintf("V%d", i), PriceCents: 1000,
		})
	}
	p, err := h.products.Create(h.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func (h *harness) activeVariants(t *testing.T, productID uuid.UUID) int {
	t.Helper()
	n, err := h.db.NewSelect().TableExpr("variants").Where("product_id = ? AND active", productID).Count(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	return int(n)
}

// Two admins deactivate the two different variants of a published product at the
// same moment. Exactly one of them may succeed; the product must keep a sellable variant.
func TestConcurrentDeactivation_KeepsOneActiveVariant(t *testing.T) {
	h := setup(t)
	off := false

	for round := 0; round < 25; round++ {
		p := h.activeProduct(t, fmt.Sprintf("RACE%d-%s", round, uuid.NewString()[:8]), 2)

		var wg sync.WaitGroup
		errs := make([]error, 2)
		start := make(chan struct{})
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[i] = h.products.UpdateVariant(h.ctx, p.ID, p.Variants[i].ID, application.UpdateVariantInput{Active: &off})
			}()
		}
		close(start)
		wg.Wait()

		if got := h.activeVariants(t, p.ID); got != 1 {
			t.Fatalf("round %d: %d active variants left, want 1 (errs: %v, %v)", round, got, errs[0], errs[1])
		}
		failures := 0
		for _, err := range errs {
			if errors.Is(err, domain.ErrNoActiveVariant) {
				failures++
			} else if err != nil {
				t.Fatalf("round %d: unexpected error %v", round, err)
			}
		}
		if failures != 1 {
			t.Fatalf("round %d: %d requests rejected, want exactly 1", round, failures)
		}
	}
}

// The repository enforces the invariant itself, whatever the service checked before.
func TestProductUpdate_CannotPublishWithoutActiveVariant(t *testing.T) {
	h := setup(t)
	p := h.activeProduct(t, "PUB-"+uuid.NewString()[:8], 1)

	// Bypass the service: archive, then turn the only variant off directly.
	if _, err := h.db.ExecContext(h.ctx, "UPDATE products SET status = 'archived' WHERE id = ?", p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.ExecContext(h.ctx, "UPDATE variants SET active = false WHERE product_id = ?", p.ID); err != nil {
		t.Fatal(err)
	}

	repo := postgres.NewProductRepository(h.db)
	p.Status = domain.StatusActive
	if err := repo.Update(h.ctx, p, false); !errors.Is(err, domain.ErrNoActiveVariant) {
		t.Fatalf("err = %v, want ErrNoActiveVariant", err)
	}
}

func TestAddVariant_UnknownProduct(t *testing.T) {
	h := setup(t)
	_, err := h.products.AddVariant(h.ctx, uuid.New(), application.VariantInput{SKU: "GHOST-1", PriceCents: 100})
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("err = %v, want ErrProductNotFound", err)
	}
}

func TestVariantSnapshots_JoinsProductAndSkipsUnknown(t *testing.T) {
	h := setup(t)
	p := h.activeProduct(t, "SNAP"+uuid.NewString()[:6], 2)

	got, err := h.variants.Snapshots(h.ctx, []uuid.UUID{p.Variants[0].ID, p.Variants[1].ID, uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d snapshots, want 2 (unknown id skipped)", len(got))
	}
	for _, s := range got {
		if s.ProductName != p.Name || s.ProductStatus != domain.StatusActive || !s.Sellable() || s.PriceCents != 1000 {
			t.Errorf("snapshot %+v", s)
		}
	}
	if none, err := h.variants.Snapshots(h.ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("empty input: %v, %v", none, err)
	}
}
