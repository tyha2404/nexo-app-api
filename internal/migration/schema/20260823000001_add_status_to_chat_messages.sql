-- +goose Up
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'SUCCESS';

-- +goose Down
ALTER TABLE chat_messages DROP COLUMN IF EXISTS status;
