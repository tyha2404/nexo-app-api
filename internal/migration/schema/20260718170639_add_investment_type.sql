-- +goose Up
-- Drop old check constraints if they exist
ALTER TABLE categories DROP CONSTRAINT IF EXISTS chk_categories_type;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS chk_transactions_type;

-- Add updated check constraints including 'INVESTMENT'
ALTER TABLE categories ADD CONSTRAINT chk_categories_type CHECK (type IN ('INCOME', 'EXPENSE', 'INVESTMENT'));
ALTER TABLE transactions ADD CONSTRAINT chk_transactions_type CHECK (type IN ('INCOME', 'EXPENSE', 'INVESTMENT'));

-- +goose Down
-- Revert constraints to only allow 'INCOME' and 'EXPENSE'
ALTER TABLE categories DROP CONSTRAINT IF EXISTS chk_categories_type;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS chk_transactions_type;

ALTER TABLE categories ADD CONSTRAINT chk_categories_type CHECK (type IN ('INCOME', 'EXPENSE'));
ALTER TABLE transactions ADD CONSTRAINT chk_transactions_type CHECK (type IN ('INCOME', 'EXPENSE'));
