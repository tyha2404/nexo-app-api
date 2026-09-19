-- +goose Up
-- Add wallet_id to debts and repayments table
ALTER TABLE debts ADD COLUMN IF NOT EXISTS wallet_id UUID REFERENCES wallets(id) ON DELETE SET NULL;
ALTER TABLE repayments ADD COLUMN IF NOT EXISTS wallet_id UUID REFERENCES wallets(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_debts_wallet_id ON debts(wallet_id);
CREATE INDEX IF NOT EXISTS idx_repayments_wallet_id ON repayments(wallet_id);

-- +goose Down
DROP INDEX IF EXISTS idx_repayments_wallet_id;
DROP INDEX IF EXISTS idx_debts_wallet_id;
ALTER TABLE repayments DROP COLUMN IF EXISTS wallet_id;
ALTER TABLE debts DROP COLUMN IF EXISTS wallet_id;
