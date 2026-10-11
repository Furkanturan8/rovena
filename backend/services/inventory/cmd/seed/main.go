package main

import (
	"context"
	"encoding/json"
	"errors"
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

// Gives every variant of the demo catalog a starting stock. Inventory has no access
// to the catalog database, so the variants are read from the catalog public API —
// the same way any other service would.
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

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
			VariantID: v.id, SKU: v.sku, OnHand: openingStock(v.sku), Reason: "demo seed",
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

// openingStock derives a stable quantity from the SKU, so repeated seeding of a
// rebuilt database always produces the same demo numbers.
func openingStock(sku string) int {
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
