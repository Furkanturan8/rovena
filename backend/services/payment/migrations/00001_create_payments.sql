-- +goose Up

-- One row per payment attempt for an order. A customer who fails and tries again
-- (with a new idempotency key) gets a new row; a repeated request with the same
-- key finds the existing one.
CREATE TABLE payments (
    id              uuid PRIMARY KEY,
    order_id        uuid        NOT NULL,
    customer_id     uuid        NOT NULL,
    -- Identifies the customer's intent to pay. The unique constraint is what makes a
    -- duplicated PaymentRequested charge only once, even across service replicas.
    idempotency_key text        NOT NULL CONSTRAINT payments_idempotency_key_key UNIQUE,
    amount_cents    bigint      NOT NULL CHECK (amount_cents > 0),
    currency        text        NOT NULL CHECK (char_length(currency) = 3),
    payment_method  text        NOT NULL,
    status          text        NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed')),
    -- How many times the provider was called. Bounds retries after provider outages.
    attempts        int         NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    provider_ref    text        NOT NULL DEFAULT '',
    failure_code    text        NOT NULL DEFAULT '',
    failure_message text        NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    completed_at    timestamptz,
    -- A finished payment has a definite outcome; a pending one has none.
    CONSTRAINT payments_outcome_consistent CHECK (
        (status = 'pending'   AND completed_at IS NULL     AND provider_ref = '' AND failure_code = '')
     OR (status = 'succeeded' AND completed_at IS NOT NULL AND provider_ref <> '' AND failure_code = '')
     OR (status = 'failed'    AND completed_at IS NOT NULL AND failure_code <> '')
    )
);

-- An order is paid at most once: the last line of defence against charging twice.
CREATE UNIQUE INDEX payments_one_success_per_order_idx ON payments (order_id) WHERE status = 'succeeded';
CREATE INDEX payments_order_idx ON payments (order_id, created_at DESC);
CREATE INDEX payments_status_created_idx ON payments (status, created_at DESC);

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
DROP TABLE payments;
