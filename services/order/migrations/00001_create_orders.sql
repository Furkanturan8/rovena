-- +goose Up

CREATE TABLE carts (
    customer_id uuid PRIMARY KEY,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- A cart stores only what the customer chose. Names and prices are looked up in
-- Catalog whenever the cart is shown, so the client can never set a price.
CREATE TABLE cart_items (
    customer_id uuid        NOT NULL REFERENCES carts (customer_id) ON DELETE CASCADE,
    variant_id  uuid        NOT NULL,
    quantity    int         NOT NULL CHECK (quantity BETWEEN 1 AND 99),
    added_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (customer_id, variant_id)
);

CREATE TABLE orders (
    id              uuid PRIMARY KEY,
    customer_id     uuid        NOT NULL,
    -- pending: created, stock not yet held. awaiting_payment: stock held, payment
    -- requested. paid: payment succeeded, stock not yet committed. confirmed / cancelled: final.
    status          text        NOT NULL CHECK (status IN ('pending', 'awaiting_payment', 'paid', 'confirmed', 'cancelled')),
    total_cents     bigint      NOT NULL CHECK (total_cents > 0),
    currency        text        NOT NULL CHECK (char_length(currency) = 3),
    -- A repeated checkout request with the same key finds this order instead of making another.
    idempotency_key text        NOT NULL,
    -- Provider token, never a card number. Kept so a lost PaymentRequested can be sent again.
    payment_method  text        NOT NULL,
    reservation_id  uuid,
    -- Tracks the stock side separately from the order status, so the recovery job knows
    -- when a commit or a release is still owed to Inventory.
    reservation_state text      NOT NULL DEFAULT 'none' CHECK (reservation_state IN ('none', 'held', 'committed', 'released')),
    -- How many times PaymentRequested was sent. Bounds the recovery job's retries.
    payment_requests int        NOT NULL DEFAULT 0 CHECK (payment_requests >= 0),
    failure_code    text        NOT NULL DEFAULT '',
    failure_message text        NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT orders_customer_idempotency_key UNIQUE (customer_id, idempotency_key),
    CONSTRAINT orders_cancelled_has_reason CHECK (status <> 'cancelled' OR failure_code <> ''),
    CONSTRAINT orders_reservation_consistent CHECK (
        (reservation_state = 'none') = (reservation_id IS NULL)
    )
);
CREATE INDEX orders_customer_created_idx ON orders (customer_id, created_at DESC);
CREATE INDEX orders_status_created_idx ON orders (status, created_at DESC);
-- The recovery job scans only orders that still need something done.
CREATE INDEX orders_unfinished_idx ON orders (updated_at)
    WHERE status IN ('pending', 'awaiting_payment', 'paid')
       OR (status = 'cancelled' AND reservation_state = 'held');

-- The order's lines as they were when it was placed. Later catalog edits never change them.
CREATE TABLE order_items (
    order_id         uuid   NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    variant_id       uuid   NOT NULL,
    product_id       uuid   NOT NULL,
    sku              text   NOT NULL,
    product_name     text   NOT NULL,
    size             text   NOT NULL DEFAULT '',
    color            text   NOT NULL DEFAULT '',
    unit_price_cents bigint NOT NULL CHECK (unit_price_cents > 0),
    quantity         int    NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (order_id, variant_id)
);

CREATE TABLE order_status_history (
    id          bigserial PRIMARY KEY,
    order_id    uuid        NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    from_status text,
    to_status   text        NOT NULL,
    reason      text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX order_status_history_order_idx ON order_status_history (order_id, id);

-- Outbox and inbox tables (copied from shared/outbox/schema.sql).
CREATE TABLE outbox_events (
    seq          bigserial PRIMARY KEY,
    id           uuid        NOT NULL CONSTRAINT outbox_events_id_key UNIQUE,
    type         text        NOT NULL,
    envelope     jsonb       NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    attempts     int         NOT NULL DEFAULT 0,
    last_error   text        NOT NULL DEFAULT ''
);
CREATE INDEX outbox_events_pending_idx ON outbox_events (seq) WHERE published_at IS NULL;

CREATE TABLE processed_events (
    consumer     text        NOT NULL,
    event_id     uuid        NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);

-- +goose Down
DROP TABLE processed_events;
DROP TABLE outbox_events;
DROP TABLE order_status_history;
DROP TABLE order_items;
DROP TABLE orders;
DROP TABLE cart_items;
DROP TABLE carts;
