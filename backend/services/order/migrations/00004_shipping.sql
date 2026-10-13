-- +goose Up

ALTER TABLE orders DROP CONSTRAINT orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
    CHECK (status IN ('pending', 'awaiting_payment', 'paid', 'confirmed', 'shipped', 'delivered', 'cancelled'));

-- The delivery address as the customer chose it at checkout. A copy, not a reference:
-- editing or deleting an address in Identity never changes an order already placed.
ALTER TABLE orders ADD COLUMN shipping_address jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE orders ADD COLUMN carrier       text NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN tracking_code text NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN shipped_at    timestamptz;
ALTER TABLE orders ADD COLUMN delivered_at  timestamptz;

-- +goose Down

ALTER TABLE orders DROP COLUMN delivered_at;
ALTER TABLE orders DROP COLUMN shipped_at;
ALTER TABLE orders DROP COLUMN tracking_code;
ALTER TABLE orders DROP COLUMN carrier;
ALTER TABLE orders DROP COLUMN shipping_address;
ALTER TABLE orders DROP CONSTRAINT orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
    CHECK (status IN ('pending', 'awaiting_payment', 'paid', 'confirmed', 'cancelled'));
