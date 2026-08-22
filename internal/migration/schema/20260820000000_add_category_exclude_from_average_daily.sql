-- +goose Up
ALTER TABLE categories ADD COLUMN IF NOT EXISTS exclude_from_average_daily BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE categories DROP COLUMN IF EXISTS exclude_from_average_daily;