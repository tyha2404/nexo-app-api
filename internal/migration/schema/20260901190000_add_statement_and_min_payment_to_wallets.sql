-- +goose Up
ALTER TABLE wallets ADD COLUMN IF NOT EXISTS statement_balance NUMERIC(15,2) DEFAULT 0.00;
ALTER TABLE wallets ADD COLUMN IF NOT EXISTS minimum_payment NUMERIC(15,2) DEFAULT 0.00;
ALTER TABLE wallets ADD COLUMN IF NOT EXISTS previous_balance NUMERIC(15,2) DEFAULT 0.00;

-- +goose Down
ALTER TABLE wallets DROP COLUMN IF EXISTS previous_balance;
ALTER TABLE wallets DROP COLUMN IF EXISTS minimum_payment;
ALTER TABLE wallets DROP COLUMN IF EXISTS statement_balance;
