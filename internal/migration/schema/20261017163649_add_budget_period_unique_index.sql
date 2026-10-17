-- +goose Up
-- One live budget per (user, category, period type, period start).
-- 1. Remove duplicates created by the old rollover (DISTINCT over all columns
--    copied every historical budget). Keep the oldest row per key; alerts that
--    reference the removed duplicates are deleted by FK ON DELETE CASCADE.
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY user_id, category_id, period_type, period_start
               ORDER BY created_at ASC, id ASC
           ) AS rn
    FROM budgets
    WHERE deleted_at IS NULL
)
DELETE FROM budgets b
USING ranked r
WHERE b.id = r.id AND r.rn > 1;

-- 2. Partial unique index (soft-deleted budgets never block a new one).
--    Used by rollover CreateBudgets: ON CONFLICT (...) WHERE deleted_at IS NULL DO NOTHING.
CREATE UNIQUE INDEX IF NOT EXISTS idx_budgets_user_category_period
    ON budgets (user_id, category_id, period_type, period_start)
    WHERE deleted_at IS NULL;

-- +goose Down
-- Deleted duplicate rows are not restored.
DROP INDEX IF EXISTS idx_budgets_user_category_period;
