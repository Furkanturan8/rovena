-- +goose Up

-- Copy this file into a service's migrations directory to give it an outbox and an inbox.

-- Events waiting to be sent (or already sent). A service inserts a row in the same
-- transaction as its business change, so the change and its event commit together or
-- not at all; the relay delivers the row to RabbitMQ afterwards.
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
-- Partial index: the relay only ever looks for rows that are still unsent.
CREATE INDEX outbox_events_pending_idx ON outbox_events (seq) WHERE published_at IS NULL;
-- ...and the cleaner only ever looks for rows that were sent long ago.
CREATE INDEX outbox_events_published_idx ON outbox_events (published_at) WHERE published_at IS NOT NULL;

-- One row per (consumer, event) already handled. The primary key is what turns a
-- redelivered event into a no-op.
CREATE TABLE processed_events (
    consumer     text        NOT NULL,
    event_id     uuid        NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
CREATE INDEX processed_events_age_idx ON processed_events (processed_at);

-- +goose Down
DROP TABLE processed_events;
DROP TABLE outbox_events;
