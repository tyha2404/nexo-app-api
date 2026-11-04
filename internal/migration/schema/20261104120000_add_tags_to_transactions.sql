-- +goose Up
-- Free-form labels on transactions (e.g. "Du lịch Đà Lạt") that cut across
-- categories. The GIN index serves the `tags @> ARRAY[...]` list filter.
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';
CREATE INDEX IF NOT EXISTS idx_transactions_tags ON transactions USING GIN (tags);

-- +goose Down
DROP INDEX IF EXISTS idx_transactions_tags;
ALTER TABLE transactions DROP COLUMN IF EXISTS tags;
