-- +goose Up
-- numeric(10,2) caps budgets at 99,999,999.99 VND; align with transactions/wallets.
ALTER TABLE budgets ALTER COLUMN amount TYPE NUMERIC(15,2);

-- +goose Down
-- Fails if any budget amount exceeds 99,999,999.99.
ALTER TABLE budgets ALTER COLUMN amount TYPE NUMERIC(10,2);
