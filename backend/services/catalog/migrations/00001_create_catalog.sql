-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE categories (
    id          uuid PRIMARY KEY,
    parent_id   uuid        CONSTRAINT categories_parent_id_fkey REFERENCES categories (id) ON DELETE RESTRICT,
    name        text        NOT NULL,
    slug        text        NOT NULL CONSTRAINT categories_slug_key UNIQUE,
    description text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX categories_parent_id_idx ON categories (parent_id);

CREATE TABLE products (
    id          uuid PRIMARY KEY,
    category_id uuid        NOT NULL CONSTRAINT products_category_id_fkey REFERENCES categories (id) ON DELETE RESTRICT,
    name        text        NOT NULL,
    slug        text        NOT NULL CONSTRAINT products_slug_key UNIQUE,
    description text        NOT NULL DEFAULT '',
    brand       text        NOT NULL DEFAULT '',
    status      text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'archived')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX products_category_id_idx ON products (category_id);
CREATE INDEX products_status_created_idx ON products (status, created_at DESC);
CREATE INDEX products_search_trgm_idx ON products USING gin ((name || ' ' || brand) gin_trgm_ops);

CREATE TABLE product_images (
    id         uuid PRIMARY KEY,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    url        text NOT NULL,
    alt        text NOT NULL DEFAULT '',
    position   int  NOT NULL,
    UNIQUE (product_id, position)
);

CREATE TABLE variants (
    id          uuid PRIMARY KEY,
    product_id  uuid        NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    sku         text        NOT NULL CONSTRAINT variants_sku_key UNIQUE,
    size        text        NOT NULL DEFAULT '',
    color       text        NOT NULL DEFAULT '',
    price_cents bigint      NOT NULL CHECK (price_cents > 0),
    currency    char(3)     NOT NULL DEFAULT 'TRY',
    active      boolean     NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX variants_product_options_key ON variants (product_id, lower(size), lower(color));

-- +goose Down
DROP TABLE variants;
DROP TABLE product_images;
DROP TABLE products;
DROP TABLE categories;
