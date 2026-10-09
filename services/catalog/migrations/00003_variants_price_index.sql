-- +goose Up
-- Serves the "cheapest active variant" lookup used for price_from and price sorting
-- (index-only scan instead of fetching every variant row of every product).
CREATE INDEX variants_active_price_idx ON variants (product_id, price_cents) WHERE active;

-- +goose Down
DROP INDEX variants_active_price_idx;
