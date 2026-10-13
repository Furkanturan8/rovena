package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/application"
	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	"github.com/Furkanturan8/rovena/services/inventory/internal/infrastructure/config"
	"github.com/Furkanturan8/rovena/services/inventory/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/inventory/internal/repository/postgres"
	"github.com/google/uuid"
)

// stock.json maps a variant SKU to its opening stock (the storefront demo catalog, sold-out and
// last-pieces sizes included). SKUs it does not know get a stable number derived from the SKU.
//
//go:embed stock.json
var stockJSON []byte

// Gives every variant of the demo catalog a starting stock. Inventory has no access
// to the catalog database, so the variants are read from the catalog public API —
// the same way any other service would. With -reset all stock, movements and reservations are
// deleted first; that is meant for local development, after the catalog was re-seeded.
func main() {
	reset := flag.Bool("reset", false, "delete all stock data before seeding (development only)")
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if reset {
		if _, err := db.ExecContext(ctx, "TRUNCATE reservation_items, reservations, stock_movements, stock_items CASCADE"); err != nil {
			return err
		}
		fmt.Println("stock cleared")
	}

	var opening map[string]int
	if err := json.Unmarshal(stockJSON, &opening); err != nil {
		return fmt.Errorf("parse stock.json: %w", err)
	}

	stock := application.NewStockService(postgres.NewStockRepository(db))

	_, total, err := stock.List(ctx, domain.StockFilter{Page: 1, PageSize: 1})
	if err != nil {
		return err
	}
	if total > 0 {
		fmt.Println("inventory already seeded")
		return nil
	}

	variants, err := fetchVariants(ctx, cfg.CatalogBaseURL)
	if err != nil {
		return err
	}
	if len(variants) == 0 {
		fmt.Println("catalog has no products yet; run make seed-catalog first")
		return nil
	}

	created := 0
	for _, v := range variants {
		_, err := stock.Create(ctx, application.CreateStockInput{
			VariantID: v.id, SKU: v.sku, OnHand: openingStock(opening, v.sku), Reason: "demo seed",
		})
		if errors.Is(err, domain.ErrStockItemExists) || errors.Is(err, domain.ErrSKUTaken) {
			continue
		}
		if err != nil {
			return fmt.Errorf("create stock for %s: %w", v.sku, err)
		}
		created++
	}
	fmt.Printf("inventory seeded: %d stock items\n", created)
	return nil
}

type variant struct {
	id  uuid.UUID
	sku string
}

// openingStock returns the quantity listed in stock.json, or a stable one derived from the SKU,
// so repeated seeding of a rebuilt database always produces the same demo numbers.
func openingStock(listed map[string]int, sku string) int {
	if n, ok := listed[sku]; ok {
		return n
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(sku))
	return 5 + int(h.Sum32()%46)
}

type catalogEnvelope[T any] struct {
	Data T `json:"data"`
}

type catalogPage struct {
	Items []struct {
		ID string `json:"id"`
	} `json:"items"`
	Total int `json:"total"`
}

type catalogProduct struct {
	Variants []struct {
		ID  string `json:"id"`
		SKU string `json:"sku"`
	} `json:"variants"`
}

func fetchVariants(ctx context.Context, baseURL string) ([]variant, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	const pageSize = 100
	var ids []string
	for page := 1; ; page++ {
		listURL := fmt.Sprintf("%s/api/v1/products?page=%d&page_size=%d", baseURL, page, pageSize)
		body, err := get[catalogPage](ctx, client, listURL)
		if err != nil {
			return nil, err
		}
		for _, item := range body.Items {
			ids = append(ids, item.ID)
		}
		if len(ids) >= body.Total || len(body.Items) == 0 {
			break
		}
	}

	var out []variant
	for _, id := range ids {
		product, err := get[catalogProduct](ctx, client, baseURL+"/api/v1/products/"+url.PathEscape(id))
		if err != nil {
			return nil, err
		}
		for _, v := range product.Variants {
			variantID, err := uuid.Parse(v.ID)
			if err != nil {
				return nil, fmt.Errorf("catalog returned an invalid variant id %q", v.ID)
			}
			out = append(out, variant{id: variantID, sku: v.SKU})
		}
	}
	return out, nil
}

func get[T any](ctx context.Context, client *http.Client, rawURL string) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return zero, err
	}
	res, err := client.Do(req)
	if err != nil {
		return zero, fmt.Errorf("call catalog: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("catalog returned %s for %s", res.Status, rawURL)
	}
	var envelope catalogEnvelope[T]
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return zero, fmt.Errorf("decode catalog response: %w", err)
	}
	return envelope.Data, nil
}
