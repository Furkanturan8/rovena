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

	cat, err := h.categories.Create(ctx, application.CreateCategoryInput{Name: "itest-" + uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	h.categoryID = cat.ID
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM products WHERE category_id = ?", cat.ID)
		_ = h.categories.Delete(ctx, cat.ID)
	})
	return h
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
