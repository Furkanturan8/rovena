-- +goose Up

-- One row per catalog variant. variant_id and sku are copies of catalog data:
-- there is no foreign key across services, Inventory only stores what it needs.
-- The CHECK constraints are the last line of defence against overselling, even
-- if application code has a bug.
CREATE TABLE stock_items (
    id         uuid PRIMARY KEY,
    variant_id uuid        NOT NULL CONSTRAINT stock_items_variant_id_key UNIQUE,
    sku        text        NOT NULL CONSTRAINT stock_items_sku_key UNIQUE,
    on_hand    int         NOT NULL DEFAULT 0 CHECK (on_hand >= 0),
    reserved   int         NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT stock_items_reserved_within_on_hand CHECK (reserved <= on_hand)
);
CREATE INDEX stock_items_available_idx ON stock_items ((on_hand - reserved));

-- A reservation holds stock for one order. order_id is unique, which makes
-- reserving idempotent: a redelivered checkout request finds the existing row
-- instead of holding the stock twice.
CREATE TABLE reservations (
    id         uuid PRIMARY KEY,
    order_id   uuid        NOT NULL CONSTRAINT reservations_order_id_key UNIQUE,
    status     text        NOT NULL CHECK (status IN ('pending', 'committed', 'released', 'expired')),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
-- Partial index: the expirer only ever scans pending reservations.
CREATE INDEX reservations_due_idx ON reservations (expires_at) WHERE status = 'pending';

CREATE TABLE reservation_items (
    reservation_id uuid NOT NULL REFERENCES reservations (id) ON DELETE CASCADE,
    variant_id     uuid NOT NULL REFERENCES stock_items (variant_id) ON DELETE RESTRICT,
    quantity       int  NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (reservation_id, variant_id)
);

-- Append-only ledger of every on_hand change. Reservations do not appear here:
-- they move stock between "available" and "reserved", not in or out of the warehouse.
CREATE TABLE stock_movements (
    id             uuid PRIMARY KEY,
    variant_id     uuid        NOT NULL REFERENCES stock_items (variant_id) ON DELETE RESTRICT,
    type           text        NOT NULL CHECK (type IN ('receipt', 'adjustment', 'sale')),
    quantity_delta int         NOT NULL CHECK (quantity_delta <> 0),
    reason         text        NOT NULL DEFAULT '',
    reservation_id uuid        REFERENCES reservations (id) ON DELETE SET NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX stock_movements_variant_created_idx ON stock_movements (variant_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE stock_movements;
DROP TABLE reservation_items;
DROP TABLE reservations;
DROP TABLE stock_items;
