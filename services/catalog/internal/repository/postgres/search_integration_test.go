//go:build integration

package postgres_test

import (
	"slices"
	"testing"

	"github.com/Furkanturan8/rovena/services/catalog/internal/application"
	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/Furkanturan8/rovena/services/catalog/internal/repository/postgres"
)

type searchFixture struct {
	h        *harness
	repo     *postgres.ProductRepository
	root     *domain.Category
	child    *domain.Category
	canta    *domain.Product // child category, brand Aurea
	tisort   *domain.Product
	ceket    *domain.Product
	taslak   *domain.Product // draft
	pasif    *domain.Product // one inactive + one active variant
	rootSlug string
}

func v(sku, size, color string, price int64) application.VariantInput {
	return application.VariantInput{SKU: sku, Size: size, Color: color, PriceCents: price}
}

// newSearchFixture creates five products, in this order (relevant for "newest").
//
//	canta  "Deri Çanta"      Aurea   child   Taba 200000, Siyah 250000
//	tisort "Basic Tişört"    Rovena  root    M/Beyaz 30000, M/Siyah 35000, L/Beyaz 32000
//	ceket  "Deri Ceket"      Rovena  root    S/Siyah 450000, M/Kahverengi 480000
//	taslak "Gizli Taslak"    Rovena  root    draft, 10000
//	pasif  "Pasif Varyantli" Rovena  root    active: B 90000; inactive: A 10000
func newSearchFixture(t *testing.T) *searchFixture {
	t.Helper()
	h := setup(t)
	f := &searchFixture{h: h, repo: postgres.NewProductRepository(h.db), root: h.root, rootSlug: h.root.Slug}
	f.child = h.newCategory(t, h.root)

	uniq := f.root.Slug[len("itest-"):][:8]
	create := func(name, brand string, cat *domain.Category, status domain.ProductStatus, images []domain.Image, vs ...application.VariantInput) *domain.Product {
		for i := range vs {
			vs[i].SKU += "-" + uniq
		}
		p, err := h.products.Create(h.ctx, application.CreateProductInput{
			Name: name, Brand: brand, CategoryID: cat.ID, Status: status, Images: images, Variants: vs,
		})
		if err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
		return p
	}
	active := domain.StatusActive

	f.canta = create("Deri Çanta", "Aurea", f.child, active,
		[]domain.Image{{URL: "https://img.test/canta-1.jpg"}, {URL: "https://img.test/canta-2.jpg"}},
		v("CNT-TAN", "", "Taba", 200000), v("CNT-BLK", "", "Siyah", 250000))
	f.tisort = create("Basic Tişört", "Rovena", f.root, active, nil,
		v("TEE-MW", "M", "Beyaz", 30000), v("TEE-MB", "M", "Siyah", 35000), v("TEE-LW", "L", "Beyaz", 32000))
	f.ceket = create("Deri Ceket", "Rovena", f.root, active, nil,
		v("JKT-SB", "S", "Siyah", 450000), v("JKT-MK", "M", "Kahverengi", 480000))
	f.taslak = create("Gizli Taslak", "Rovena", f.root, domain.StatusDraft, nil, v("DRF-1", "M", "Mavi", 10000))
	f.pasif = create("Pasif Varyantli", "Rovena", f.root, active, nil,
		v("PSF-A", "A", "", 10000), v("PSF-B", "B", "", 90000))

	off := false
	if _, err := h.products.UpdateVariant(h.ctx, f.pasif.ID, f.pasif.Variants[0].ID, application.UpdateVariantInput{Active: &off}); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *searchFixture) list(t *testing.T, flt domain.ProductFilter) ([]domain.ProductSummary, int) {
	t.Helper()
	if flt.CategorySlug == "" {
		flt.CategorySlug = f.rootSlug
	}
	if flt.Status == nil {
		a := domain.StatusActive
		flt.Status = &a
	}
	if flt.Page == 0 {
		flt.Page = 1
	}
	if flt.PageSize == 0 {
		flt.PageSize = 50
	}
	items, total, err := f.repo.List(f.h.ctx, flt)
	if err != nil {
		t.Fatalf("List(%+v): %v", flt, err)
	}
	return items, total
}

func (f *searchFixture) names(items []domain.ProductSummary) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

func int64p(n int64) *int64 { return &n }

func TestSearchAndFilter(t *testing.T) {
	f := newSearchFixture(t)

	want := func(t *testing.T, flt domain.ProductFilter, ordered bool, names ...string) {
		t.Helper()
		items, total := f.list(t, flt)
		got := f.names(items)
		if !ordered {
			got, names = sorted(got), sorted(names)
		}
		if !slices.Equal(got, names) {
			t.Errorf("filter %+v\n got  %v\n want %v", flt, got, names)
		}
		if total != len(names) {
			t.Errorf("filter %+v: total = %d, want %d", flt, total, len(names))
		}
	}

	t.Run("text query", func(t *testing.T) {
		want(t, domain.ProductFilter{Query: "canta"}, false, "Deri Çanta") // accent-insensitive
		want(t, domain.ProductFilter{Query: "ÇANTA"}, false, "Deri Çanta") // upper case + accent
		want(t, domain.ProductFilter{Query: "deri"}, false, "Deri Çanta", "Deri Ceket")
		want(t, domain.ProductFilter{Query: "rovena"}, false, "Basic Tişört", "Deri Ceket", "Pasif Varyantli") // brand is searchable, draft excluded
		want(t, domain.ProductFilter{Query: "deri rovena"}, false, "Deri Ceket")                               // every word must match
		want(t, domain.ProductFilter{Query: "tisort"}, false, "Basic Tişört")
		want(t, domain.ProductFilter{Query: "yokboyleurun"}, false)
	})

	t.Run("query cannot inject wildcards or SQL", func(t *testing.T) {
		// Punctuation is stripped, so these degrade to "no text filter" or harmless words.
		for _, q := range []string{"%", "_", `\`, "'; DROP TABLE products; --", "%%%%deri"} {
			items, _ := f.list(t, domain.ProductFilter{Query: q})
			if len(items) > 4 {
				t.Errorf("q=%q returned %d items", q, len(items))
			}
		}
		want(t, domain.ProductFilter{Query: "%deri%"}, false, "Deri Çanta", "Deri Ceket")
		var n int
		if err := f.h.db.NewSelect().TableExpr("products").ColumnExpr("count(*)").Scan(f.h.ctx, &n); err != nil || n == 0 {
			t.Fatalf("products table damaged: n=%d err=%v", n, err)
		}
	})

	t.Run("category includes descendants", func(t *testing.T) {
		want(t, domain.ProductFilter{CategorySlug: f.child.Slug}, false, "Deri Çanta")
		want(t, domain.ProductFilter{CategorySlug: f.root.Slug}, false, "Deri Çanta", "Basic Tişört", "Deri Ceket", "Pasif Varyantli")
		want(t, domain.ProductFilter{CategorySlug: "no-such-category-" + f.root.Slug}, false)
	})

	t.Run("variant filters must match the same variant", func(t *testing.T) {
		want(t, domain.ProductFilter{Size: "M", Color: "Siyah"}, false, "Basic Tişört") // ceket has S/Siyah and M/Kahverengi, not M/Siyah
		want(t, domain.ProductFilter{Size: "m", Color: "SIYAH"}, false, "Basic Tişört") // case-insensitive
		want(t, domain.ProductFilter{Color: "Siyah"}, false, "Deri Çanta", "Basic Tişört", "Deri Ceket")
		want(t, domain.ProductFilter{Size: "M"}, false, "Basic Tişört", "Deri Ceket")
		want(t, domain.ProductFilter{Size: "XXL"}, false)
	})

	t.Run("price range", func(t *testing.T) {
		want(t, domain.ProductFilter{MinPrice: int64p(400000)}, false, "Deri Ceket")
		want(t, domain.ProductFilter{MaxPrice: int64p(33000)}, false, "Basic Tişört") // pasif's 10000 variant is inactive
		want(t, domain.ProductFilter{MinPrice: int64p(31000), MaxPrice: int64p(33000)}, false, "Basic Tişört")
		want(t, domain.ProductFilter{MinPrice: int64p(35000), MaxPrice: int64p(35000)}, false, "Basic Tişört") // inclusive bounds
		// Price and color on the same variant: the only <=33000 tişört variants are Beyaz.
		want(t, domain.ProductFilter{Color: "Siyah", MaxPrice: int64p(33000)}, false)
		want(t, domain.ProductFilter{Color: "Beyaz", MaxPrice: int64p(33000)}, false, "Basic Tişört")
	})

	t.Run("combined filters", func(t *testing.T) {
		want(t, domain.ProductFilter{Query: "rovena", Size: "M", MaxPrice: int64p(500000)}, false, "Basic Tişört", "Deri Ceket")
		want(t, domain.ProductFilter{Query: "deri", CategorySlug: f.child.Slug, Color: "taba"}, false, "Deri Çanta")
	})

	t.Run("sorting", func(t *testing.T) {
		want(t, domain.ProductFilter{Sort: domain.SortPriceAsc}, true, "Basic Tişört", "Pasif Varyantli", "Deri Çanta", "Deri Ceket")
		want(t, domain.ProductFilter{Sort: domain.SortPriceDesc}, true, "Deri Ceket", "Deri Çanta", "Pasif Varyantli", "Basic Tişört")
		want(t, domain.ProductFilter{Sort: domain.SortNewest}, true, "Pasif Varyantli", "Deri Ceket", "Basic Tişört", "Deri Çanta")
		// ASCII-first-letter names only, so the result does not depend on the database collation.
		want(t, domain.ProductFilter{Sort: domain.SortName, Query: "rovena", Size: "M"}, true, "Basic Tişört", "Deri Ceket")
	})

	t.Run("pagination", func(t *testing.T) {
		page := func(n int) ([]string, int) {
			items, total := f.list(t, domain.ProductFilter{Sort: domain.SortPriceAsc, Page: n, PageSize: 2})
			return f.names(items), total
		}
		if got, total := page(1); !slices.Equal(got, []string{"Basic Tişört", "Pasif Varyantli"}) || total != 4 {
			t.Errorf("page 1 = %v total %d", got, total)
		}
		if got, total := page(2); !slices.Equal(got, []string{"Deri Çanta", "Deri Ceket"}) || total != 4 {
			t.Errorf("page 2 = %v total %d", got, total)
		}
		if got, total := page(3); len(got) != 0 || total != 4 {
			t.Errorf("page 3 = %v total %d, want empty page with total 4", got, total)
		}
	})

	t.Run("status", func(t *testing.T) {
		draft := domain.StatusDraft
		want(t, domain.ProductFilter{Status: &draft}, false, "Gizli Taslak")

		// nil status = every status; List() above defaults it, so call the repo directly.
		items, total, err := f.repo.List(f.h.ctx, domain.ProductFilter{CategorySlug: f.rootSlug, Page: 1, PageSize: 50})
		if err != nil || total != 5 || len(items) != 5 {
			t.Errorf("any-status list: n=%d total=%d err=%v, want 5", len(items), total, err)
		}
	})

	t.Run("summary data", func(t *testing.T) {
		items, _ := f.list(t, domain.ProductFilter{})
		byName := map[string]domain.ProductSummary{}
		for _, it := range items {
			byName[it.Name] = it
		}
		canta := byName["Deri Çanta"]
		if canta.PriceFromCents == nil || *canta.PriceFromCents != 200000 {
			t.Errorf("canta price_from = %v, want 200000", canta.PriceFromCents)
		}
		if canta.ThumbnailURL != "https://img.test/canta-1.jpg" {
			t.Errorf("canta thumbnail = %q, want the first image", canta.ThumbnailURL)
		}
		if canta.Currency != domain.DefaultCurrency || canta.CategoryID != f.child.ID {
			t.Errorf("canta = %+v", canta)
		}
		if pasif := byName["Pasif Varyantli"]; pasif.PriceFromCents == nil || *pasif.PriceFromCents != 90000 {
			t.Errorf("pasif price_from = %v, want 90000 (inactive variant ignored)", pasif.PriceFromCents)
		}
		if tisort := byName["Basic Tişört"]; tisort.ThumbnailURL != "" {
			t.Errorf("tisort has no images but thumbnail = %q", tisort.ThumbnailURL)
		}
	})
}

func TestGetAndUpdateImages(t *testing.T) {
	f := newSearchFixture(t)
	ctx := f.h.ctx

	got, err := f.repo.GetBySlug(ctx, f.canta.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Images) != 2 || got.Images[0].URL != "https://img.test/canta-1.jpg" || got.Images[1].Position != 1 {
		t.Errorf("images = %+v", got.Images)
	}
	if len(got.Variants) != 2 {
		t.Errorf("variants = %d, want 2", len(got.Variants))
	}

	// Replacing images keeps the new order; not replacing leaves them alone.
	reversed := []domain.Image{{URL: "https://img.test/new-a.jpg"}, {URL: "https://img.test/new-b.jpg"}, {URL: "https://img.test/new-c.jpg"}}
	if _, err := f.h.products.Update(ctx, f.canta.ID, application.UpdateProductInput{Images: &reversed}); err != nil {
		t.Fatal(err)
	}
	name := "Deri Çanta Yeni"
	if _, err := f.h.products.Update(ctx, f.canta.ID, application.UpdateProductInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	got, _ = f.repo.GetByID(ctx, f.canta.ID)
	if len(got.Images) != 3 || got.Images[0].URL != "https://img.test/new-a.jpg" || got.Images[2].Position != 2 {
		t.Errorf("images after update = %+v", got.Images)
	}

	// Renaming must refresh the accent-folded search column.
	items, _ := f.list(t, domain.ProductFilter{Query: "yeni"})
	if len(items) != 1 || items[0].ID != f.canta.ID {
		t.Errorf("search after rename = %v", f.names(items))
	}
	if items, _ := f.list(t, domain.ProductFilter{Query: "canta"}); len(items) != 1 {
		t.Errorf("old words must still match, got %v", f.names(items))
	}
}
