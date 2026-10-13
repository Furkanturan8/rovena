package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Furkanturan8/rovena/services/catalog/internal/application"
	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/Furkanturan8/rovena/services/catalog/internal/infrastructure/config"
	"github.com/Furkanturan8/rovena/services/catalog/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/catalog/internal/repository/postgres"
)

// store.json holds the storefront catalog (categories, products, variants with Turkish and
// English texts). Image paths start with /media/ and are resolved against MEDIA_BASE_URL.
//
//go:embed store.json
var storeJSON []byte

type seedFile struct {
	Categories []struct {
		Slug         string          `json:"slug"`
		Name         string          `json:"name"`
		Description  string          `json:"description"`
		Translations json.RawMessage `json:"translations"`
	} `json:"categories"`
	Products []struct {
		Slug         string          `json:"slug"`
		Category     string          `json:"category"`
		Brand        string          `json:"brand"`
		Status       string          `json:"status"`
		Name         string          `json:"name"`
		Description  string          `json:"description"`
		Translations json.RawMessage `json:"translations"`
		Attributes   json.RawMessage `json:"attributes"`
		Images       []string        `json:"images"`
		Variants     []struct {
			SKU        string `json:"sku"`
			Size       string `json:"size"`
			Color      string `json:"color"`
			PriceCents int64  `json:"price_cents"`
		} `json:"variants"`
	} `json:"products"`
}

// Loads the storefront catalog. Existing categories and products (by slug) are left alone, so
// running it twice is harmless. With -reset every category and product is deleted first; that is
// meant for local development, to replace an older demo catalog.
func main() {
	reset := flag.Bool("reset", false, "delete all categories and products before seeding (development only)")
	flag.Parse()
	if err := run(*reset); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(reset bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var data seedFile
	if err := json.Unmarshal(storeJSON, &data); err != nil {
		return fmt.Errorf("parse store.json: %w", err)
	}
	mediaBase := strings.TrimRight(os.Getenv("MEDIA_BASE_URL"), "/")
	if mediaBase == "" {
		return errors.New("MEDIA_BASE_URL is required: product images are stored as absolute URLs " +
			"(e.g. https://<project>.supabase.co/storage/v1/object/public/media)")
	}

	ctx := context.Background()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if reset {
		if _, err := db.ExecContext(ctx, "DELETE FROM products"); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, "DELETE FROM categories WHERE parent_id IS NOT NULL"); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, "DELETE FROM categories"); err != nil {
			return err
		}
		fmt.Println("catalog cleared")
	}

	catRepo := postgres.NewCategoryRepository(db)
	categories := application.NewCategoryService(catRepo)
	prodRepo := postgres.NewProductRepository(db)
	products := application.NewProductService(prodRepo, postgres.NewVariantRepository(db))

	categoryIDs := map[string]domain.Category{}
	newCategories := 0
	for _, c := range data.Categories {
		existing, err := catRepo.GetBySlug(ctx, c.Slug)
		if err == nil {
			categoryIDs[c.Slug] = *existing
			continue
		}
		if !errors.Is(err, domain.ErrCategoryNotFound) {
			return err
		}
		created, err := categories.Create(ctx, application.CreateCategoryInput{
			Name: c.Name, Slug: c.Slug, Description: c.Description, Translations: c.Translations,
		})
		if err != nil {
			return fmt.Errorf("create category %q: %w", c.Slug, err)
		}
		categoryIDs[c.Slug] = *created
		newCategories++
	}

	newProducts := 0
	for _, p := range data.Products {
		if taken, err := prodRepo.SlugExists(ctx, p.Slug); err != nil {
			return err
		} else if taken {
			continue
		}
		category, ok := categoryIDs[p.Category]
		if !ok {
			return fmt.Errorf("product %q: unknown category %q", p.Slug, p.Category)
		}
		in := application.CreateProductInput{
			Name: p.Name, Slug: p.Slug, Description: p.Description, Brand: p.Brand,
			CategoryID: category.ID, Status: domain.ProductStatus(p.Status),
			Translations: p.Translations, Attributes: p.Attributes,
		}
		for _, img := range p.Images {
			in.Images = append(in.Images, domain.Image{URL: mediaURL(mediaBase, img), Alt: p.Name})
		}
		for _, v := range p.Variants {
			in.Variants = append(in.Variants, application.VariantInput{SKU: v.SKU, Size: v.Size, Color: v.Color, PriceCents: v.PriceCents})
		}
		if _, err := products.Create(ctx, in); err != nil {
			return fmt.Errorf("create product %q: %w", p.Slug, err)
		}
		newProducts++
	}
	fmt.Printf("catalog seeded: %d new categories, %d new products\n", newCategories, newProducts)
	return nil
}

// mediaURL turns "/media/products/x/1.jpg" into "<MEDIA_BASE_URL>/products/x/1.jpg".
func mediaURL(base, path string) string {
	return base + "/" + strings.TrimPrefix(strings.TrimPrefix(path, "/media/"), "/")
}
