-- +goose Up
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'HOLDING';
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS realized_pnl NUMERIC(15,2) DEFAULT 0.00;

ALTER TABLE transactions DROP CONSTRAINT IF EXISTS chk_transactions_status;
ALTER TABLE transactions ADD CONSTRAINT chk_transactions_status CHECK (status IN ('HOLDING', 'SOLD', 'MATURED', 'CANCELLED'));

-- +goose Down
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS chk_transactions_status;
ALTER TABLE transactions DROP COLUMN IF EXISTS realized_pnl;
ALTER TABLE transactions DROP COLUMN IF EXISTS status;
