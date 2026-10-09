-- +goose Up
CREATE TABLE users (
    id            uuid PRIMARY KEY,
    email         text        NOT NULL UNIQUE CHECK (email = lower(email)),
    password_hash text        NOT NULL,
    first_name    text        NOT NULL DEFAULT '',
    last_name     text        NOT NULL DEFAULT '',
    role          text        NOT NULL DEFAULT 'customer' CHECK (role IN ('customer', 'admin')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
