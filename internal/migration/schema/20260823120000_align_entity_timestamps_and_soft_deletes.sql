-- +goose Up
-- Migration: Align created_at, updated_at, deleted_at across all entity tables

-- 1. Debts & Repayments
ALTER TABLE debts ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_debts_deleted_at ON debts (deleted_at);

ALTER TABLE repayments ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE repayments ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_repayments_deleted_at ON repayments (deleted_at);

-- 2. Wallet Transfers
ALTER TABLE wallet_transfers ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE wallet_transfers ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_wallet_transfers_deleted_at ON wallet_transfers (deleted_at);

-- 3. Presets
ALTER TABLE presets ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_presets_deleted_at ON presets (deleted_at);

-- 4. Chat Sessions & Messages & Financial Knowledges
ALTER TABLE chat_sessions ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_chat_sessions_deleted_at ON chat_sessions (deleted_at);

ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_chat_messages_deleted_at ON chat_messages (deleted_at);

ALTER TABLE financial_knowledges ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE financial_knowledges ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_financial_knowledges_deleted_at ON financial_knowledges (deleted_at);

-- +goose Down
-- Reverting timestamp alignment
ALTER TABLE financial_knowledges DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;
ALTER TABLE chat_messages DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;
ALTER TABLE chat_sessions DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE wallet_transfers DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;
ALTER TABLE repayments DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;
ALTER TABLE debts DROP COLUMN IF EXISTS deleted_at;

