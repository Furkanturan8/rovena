-- +goose Up
-- A customer's saved delivery addresses. Orders keep their own copy of the address they were
-- shipped to, so editing or deleting an address never changes an old order.
CREATE TABLE addresses (
    id          uuid PRIMARY KEY,
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title       text        NOT NULL,
    full_name   text        NOT NULL,
    phone       text        NOT NULL,
    line1       text        NOT NULL,
    line2       text        NOT NULL DEFAULT '',
    district    text        NOT NULL,
    city        text        NOT NULL,
    postal_code text        NOT NULL DEFAULT '',
    country     text        NOT NULL DEFAULT 'TR',
    is_default  boolean     NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX addresses_user_idx ON addresses (user_id, created_at);
-- At most one default address per customer, enforced by the database.
CREATE UNIQUE INDEX addresses_one_default_per_user ON addresses (user_id) WHERE is_default;

-- +goose Down
DROP TABLE addresses;
