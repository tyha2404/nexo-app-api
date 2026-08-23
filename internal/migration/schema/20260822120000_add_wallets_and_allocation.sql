-- +goose Up
CREATE TABLE IF NOT EXISTS wallets (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'CASH',
    balance NUMERIC(15,2) NOT NULL DEFAULT 0.00,
    currency VARCHAR(10) DEFAULT 'VND',
    icon VARCHAR(50),
    jar_category VARCHAR(50),
    allocation_percent NUMERIC(5,2) DEFAULT 0.00,
    is_included_in_total BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT fk_wallets_user FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets(user_id);
CREATE INDEX IF NOT EXISTS idx_wallets_deleted_at ON wallets(deleted_at);

CREATE TABLE IF NOT EXISTS wallet_transfers (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    from_wallet_id UUID NOT NULL,
    to_wallet_id UUID NOT NULL,
    amount NUMERIC(15,2) NOT NULL,
    fee NUMERIC(15,2) DEFAULT 0.00,
    note TEXT,
    transfer_date DATE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_wallet_transfers_user FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_wallet_transfers_from_wallet FOREIGN KEY (from_wallet_id) REFERENCES wallets(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_wallet_transfers_to_wallet FOREIGN KEY (to_wallet_id) REFERENCES wallets(id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_wallet_transfers_user_id ON wallet_transfers(user_id);
CREATE INDEX IF NOT EXISTS idx_wallet_transfers_from_wallet_id ON wallet_transfers(from_wallet_id);
CREATE INDEX IF NOT EXISTS idx_wallet_transfers_to_wallet_id ON wallet_transfers(to_wallet_id);

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS wallet_id UUID;
CREATE INDEX IF NOT EXISTS idx_transactions_wallet_id ON transactions(wallet_id);
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_transactions_wallet') THEN
        ALTER TABLE transactions ADD CONSTRAINT fk_transactions_wallet FOREIGN KEY (wallet_id) REFERENCES wallets(id) ON UPDATE CASCADE ON DELETE SET NULL;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS fk_transactions_wallet;
DROP INDEX IF EXISTS idx_transactions_wallet_id;
ALTER TABLE transactions DROP COLUMN IF EXISTS wallet_id;

DROP TABLE IF EXISTS wallet_transfers;
DROP TABLE IF EXISTS wallets;
