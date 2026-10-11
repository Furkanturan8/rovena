-- +goose Up
-- Accent-folded, lowercase "name brand" maintained by the application (domain.SearchText)
-- so that searching "canta" finds "Çanta".
ALTER TABLE products ADD COLUMN search_text text NOT NULL DEFAULT '';

UPDATE products
SET search_text = translate(lower(name || ' ' || brand), 'çğıöşüâî', 'cgiosuai');

DROP INDEX products_search_trgm_idx;
CREATE INDEX products_search_text_trgm_idx ON products USING gin (search_text gin_trgm_ops);

-- +goose Down
DROP INDEX products_search_text_trgm_idx;
ALTER TABLE products DROP COLUMN search_text;
CREATE INDEX products_search_trgm_idx ON products USING gin ((name || ' ' || brand) gin_trgm_ops);
