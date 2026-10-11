package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Furkanturan8/rovena/services/catalog/internal/application"
	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/Furkanturan8/rovena/services/catalog/internal/infrastructure/config"
	"github.com/Furkanturan8/rovena/services/catalog/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/catalog/internal/repository/postgres"
)

// Loads demo categories and products. Does nothing when categories already exist.
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	catRepo := postgres.NewCategoryRepository(db)
	categories := application.NewCategoryService(catRepo)
	products := application.NewProductService(postgres.NewProductRepository(db), postgres.NewVariantRepository(db))

	existing, err := categories.List(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		fmt.Println("catalog already seeded")
		return nil
	}

	clothing, err := categories.Create(ctx, application.CreateCategoryInput{Name: "Giyim"})
	if err != nil {
		return err
	}
	outer, err := categories.Create(ctx, application.CreateCategoryInput{Name: "Dış Giyim", ParentID: &clothing.ID})
	if err != nil {
		return err
	}
	tshirts, err := categories.Create(ctx, application.CreateCategoryInput{Name: "Tişört", ParentID: &clothing.ID})
	if err != nil {
		return err
	}
	accessories, err := categories.Create(ctx, application.CreateCategoryInput{Name: "Aksesuar"})
	if err != nil {
		return err
	}
	bags, err := categories.Create(ctx, application.CreateCategoryInput{Name: "Çanta", ParentID: &accessories.ID})
	if err != nil {
		return err
	}

	demo := []application.CreateProductInput{
		{
			Name: "Deri Ceket", Brand: "Rovena", CategoryID: outer.ID, Status: domain.StatusActive,
			Description: "Gerçek deri, slim fit biker ceket.",
			Images:      []domain.Image{{URL: "https://picsum.photos/seed/rovena-jacket/800/1000", Alt: "Deri ceket"}},
			Variants: []application.VariantInput{
				{SKU: "JKT-LTH-BLK-S", Size: "S", Color: "Siyah", PriceCents: 459900},
				{SKU: "JKT-LTH-BLK-M", Size: "M", Color: "Siyah", PriceCents: 459900},
				{SKU: "JKT-LTH-BRN-M", Size: "M", Color: "Kahverengi", PriceCents: 479900},
			},
		},
		{
			Name: "Basic Tişört", Brand: "Rovena", CategoryID: tshirts.ID, Status: domain.StatusActive,
			Description: "Organik pamuk, regular fit.",
			Images:      []domain.Image{{URL: "https://picsum.photos/seed/rovena-tee/800/1000", Alt: "Basic tişört"}},
			Variants: []application.VariantInput{
				{SKU: "TEE-BSC-WHT-S", Size: "S", Color: "Beyaz", PriceCents: 34900},
				{SKU: "TEE-BSC-WHT-M", Size: "M", Color: "Beyaz", PriceCents: 34900},
				{SKU: "TEE-BSC-BLK-M", Size: "M", Color: "Siyah", PriceCents: 34900},
			},
		},
		{
			Name: "Deri Omuz Çantası", Brand: "Aurea", CategoryID: bags.ID, Status: domain.StatusActive,
			Description: "El yapımı, ayarlanabilir askılı omuz çantası.",
			Images:      []domain.Image{{URL: "https://picsum.photos/seed/rovena-bag/800/1000", Alt: "Omuz çantası"}},
			Variants: []application.VariantInput{
				{SKU: "BAG-SHL-TAN", Color: "Taba", PriceCents: 219900},
				{SKU: "BAG-SHL-BLK", Color: "Siyah", PriceCents: 219900},
			},
		},
		{
			Name: "Keten Gömlek", Brand: "Rovena", CategoryID: clothing.ID, Status: domain.StatusDraft,
			Description: "Taslak ürün: vitrinde görünmez.",
			Variants:    []application.VariantInput{{SKU: "SHT-LNN-BLU-M", Size: "M", Color: "Mavi", PriceCents: 89900}},
		},
	}
	for _, in := range demo {
		if _, err := products.Create(ctx, in); err != nil {
			return fmt.Errorf("create %q: %w", in.Name, err)
		}
	}
	fmt.Println("catalog seeded: 5 categories, 4 products")
	return nil
}
