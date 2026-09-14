-- +goose Up
-- Add content_hash to financial_knowledges for reliable knowledge sync detection.
-- Previously the seeder only compared chunk counts, so editing a source doc that
-- still produced the same number of chunks never triggered a re-sync.
ALTER TABLE financial_knowledges ADD COLUMN IF NOT EXISTS content_hash VARCHAR(64) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE financial_knowledges DROP COLUMN IF EXISTS content_hash;
