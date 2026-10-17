-- +goose Up
-- Models now use gorm.DeletedAt, so Delete soft-deletes. Unique indexes must
-- ignore soft-deleted rows, otherwise a deleted category/target blocks
-- re-creating one with the same key.

-- categories: (user_id, name)
DROP INDEX IF EXISTS idx_user_category_name;
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_category_name
    ON categories (user_id, name)
    WHERE deleted_at IS NULL;

-- monthly_targets: (user_id, target_type, month, year) - created by GORM AutoMigrate.
-- target_repository.UpsertTarget uses ON CONFLICT (...) WHERE deleted_at IS NULL.
DROP INDEX IF EXISTS idx_user_target_month_type;
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_target_month_type
    ON monthly_targets (user_id, target_type, month, year)
    WHERE deleted_at IS NULL;

-- +goose Down
-- Fails if a soft-deleted row duplicates a live row's key.
DROP INDEX IF EXISTS idx_user_target_month_type;
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_target_month_type
    ON monthly_targets (user_id, target_type, month, year);

DROP INDEX IF EXISTS idx_user_category_name;
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_category_name
    ON categories (user_id, name);
