-- +goose Up

-- A succeeded payment can be refunded once. The money-taken states are 'succeeded' and 'refunded',
-- so an order whose payment was given back can not be charged again under another key.
ALTER TABLE payments
    ADD COLUMN refund_ref   text        NOT NULL DEFAULT '',
    ADD COLUMN refunded_at  timestamptz;

ALTER TABLE payments DROP CONSTRAINT payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('pending', 'succeeded', 'failed', 'refunded'));

ALTER TABLE payments DROP CONSTRAINT payments_outcome_consistent;
ALTER TABLE payments ADD CONSTRAINT payments_outcome_consistent CHECK (
    (status = 'pending'   AND completed_at IS NULL     AND provider_ref = '' AND failure_code = '' AND refund_ref = '' AND refunded_at IS NULL)
 OR (status = 'succeeded' AND completed_at IS NOT NULL AND provider_ref <> '' AND failure_code = '' AND refund_ref = '' AND refunded_at IS NULL)
 OR (status = 'failed'    AND completed_at IS NOT NULL AND failure_code <> '' AND refund_ref = '' AND refunded_at IS NULL)
 OR (status = 'refunded'  AND completed_at IS NOT NULL AND provider_ref <> '' AND failure_code = '' AND refund_ref <> '' AND refunded_at IS NOT NULL)
);

DROP INDEX payments_one_success_per_order_idx;
CREATE UNIQUE INDEX payments_one_success_per_order_idx ON payments (order_id) WHERE status IN ('succeeded', 'refunded');

-- +goose Down
DROP INDEX payments_one_success_per_order_idx;
CREATE UNIQUE INDEX payments_one_success_per_order_idx ON payments (order_id) WHERE status = 'succeeded';
ALTER TABLE payments DROP CONSTRAINT payments_outcome_consistent;
ALTER TABLE payments ADD CONSTRAINT payments_outcome_consistent CHECK (
    (status = 'pending'   AND completed_at IS NULL     AND provider_ref = '' AND failure_code = '')
 OR (status = 'succeeded' AND completed_at IS NOT NULL AND provider_ref <> '' AND failure_code = '')
 OR (status = 'failed'    AND completed_at IS NOT NULL AND failure_code <> '')
);
ALTER TABLE payments DROP CONSTRAINT payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check CHECK (status IN ('pending', 'succeeded', 'failed'));
ALTER TABLE payments DROP COLUMN refunded_at, DROP COLUMN refund_ref;
