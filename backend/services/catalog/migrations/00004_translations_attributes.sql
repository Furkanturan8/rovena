-- +goose Up
-- Localized texts and storefront facts that are not columns of their own.
--   translations: {"en": {"name": "...", "description": "...", "kind": "...", ...}, "tr": {...}}
--                 Name and description columns stay the default (Turkish) text.
--   attributes:   language-independent product facts (tag, rating, craft hours, colour swatches, ...)
ALTER TABLE categories ADD COLUMN translations jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE products   ADD COLUMN translations jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE products   ADD COLUMN attributes   jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE categories ADD CONSTRAINT categories_translations_object CHECK (jsonb_typeof(translations) = 'object');
ALTER TABLE products   ADD CONSTRAINT products_translations_object   CHECK (jsonb_typeof(translations) = 'object');
ALTER TABLE products   ADD CONSTRAINT products_attributes_object     CHECK (jsonb_typeof(attributes) = 'object');

-- +goose Down
ALTER TABLE products   DROP CONSTRAINT products_attributes_object;
ALTER TABLE products   DROP CONSTRAINT products_translations_object;
ALTER TABLE categories DROP CONSTRAINT categories_translations_object;
ALTER TABLE products   DROP COLUMN attributes;
ALTER TABLE products   DROP COLUMN translations;
ALTER TABLE categories DROP COLUMN translations;
