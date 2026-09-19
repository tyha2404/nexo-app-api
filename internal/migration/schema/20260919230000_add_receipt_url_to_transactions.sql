-- +goose Up
ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS receipt_url VARCHAR(1024) NULL,
    ADD COLUMN IF NOT EXISTS metadata JSONB NULL;

CREATE INDEX IF NOT EXISTS idx_transactions_receipt_url ON transactions(receipt_url) WHERE receipt_url IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_transactions_receipt_url;
ALTER TABLE transactions
    DROP COLUMN IF EXISTS receipt_url,
    DROP COLUMN IF EXISTS metadata;
