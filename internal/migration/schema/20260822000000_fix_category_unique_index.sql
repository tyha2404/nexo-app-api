-- +goose Up
DROP INDEX IF EXISTS idx_user_category_name;
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_category_name ON categories (user_id, name);

-- +goose Down
DROP INDEX IF EXISTS idx_user_category_name;
