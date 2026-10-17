-- +goose Up
-- true once a transaction's wallet balance effect has actually been applied.
-- Existing rows stay false: their effect was never applied, so editing or
-- deleting them must not reverse anything.
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS balance_applied BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE transactions DROP COLUMN IF EXISTS balance_applied;
