-- +goose Up

-- When a refresh token was replaced by a newer one. Logging out revokes a token too, but is not
-- a rotation: only a token that was rotated a moment ago may be presented once more (a second
-- browser tab refreshing at the same time), a token that was logged out never may.
ALTER TABLE refresh_tokens ADD COLUMN rotated_at timestamptz;

-- +goose Down
ALTER TABLE refresh_tokens DROP COLUMN rotated_at;
