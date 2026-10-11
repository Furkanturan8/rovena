-- +goose Up

-- The outbox cleaner deletes published events and old inbox markers by age (shared/outbox/cleaner.go).
CREATE INDEX outbox_events_published_idx ON outbox_events (published_at) WHERE published_at IS NOT NULL;
CREATE INDEX processed_events_age_idx ON processed_events (processed_at);

-- +goose Down
DROP INDEX processed_events_age_idx;
DROP INDEX outbox_events_published_idx;
