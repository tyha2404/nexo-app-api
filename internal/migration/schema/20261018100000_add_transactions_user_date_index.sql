-- +goose NO TRANSACTION
-- +goose Up
-- Lists and reports always filter live rows by user and date range.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_transactions_user_date_live
    ON transactions (user_id, transaction_date DESC)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_transactions_user_date_live;
