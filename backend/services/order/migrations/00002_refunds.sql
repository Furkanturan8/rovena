-- +goose Up

-- Money that was taken for an order that ended up cancelled must go back. The order tracks
-- that separately from its status: a cancelled order has refund_state 'none' (nothing was
-- paid), 'requested' (Payment was asked to give the money back) or 'refunded'.
ALTER TABLE orders
    ADD COLUMN refund_state    text NOT NULL DEFAULT 'none' CHECK (refund_state IN ('none', 'requested', 'refunded')),
    -- How often RefundRequested was sent; bounds the recovery job's retries.
    ADD COLUMN refund_requests int  NOT NULL DEFAULT 0 CHECK (refund_requests >= 0),
    ADD CONSTRAINT orders_refund_only_when_cancelled CHECK (refund_state = 'none' OR status = 'cancelled');

DROP INDEX orders_unfinished_idx;
CREATE INDEX orders_unfinished_idx ON orders (updated_at)
    WHERE status IN ('pending', 'awaiting_payment', 'paid')
       OR (status = 'cancelled' AND (reservation_state = 'held' OR refund_state = 'requested'));

-- +goose Down
DROP INDEX orders_unfinished_idx;
CREATE INDEX orders_unfinished_idx ON orders (updated_at)
    WHERE status IN ('pending', 'awaiting_payment', 'paid')
       OR (status = 'cancelled' AND reservation_state = 'held');
ALTER TABLE orders DROP CONSTRAINT orders_refund_only_when_cancelled,
    DROP COLUMN refund_requests, DROP COLUMN refund_state;
