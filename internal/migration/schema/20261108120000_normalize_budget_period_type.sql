-- The AI chat set_budget tool used to save budgets with period_type
-- 'MONTHLY'. Every query, the rollover and budget alerts only match lowercase
-- 'monthly'/'yearly', so those budgets were never enforced.
--
-- 1. Where the user also has a live lowercase budget for the same category
--    and period, keep that one (it is the one alerts were using) and
--    soft-delete the uppercase copy.
-- 2. Lowercase the rest.
-- 3. Add the check constraint the GORM model declares, so it can't recur.
--    NOT VALID + VALIDATE avoids a long lock on big tables.

-- +goose Up
-- +goose StatementBegin
UPDATE budgets u
SET deleted_at = CURRENT_TIMESTAMP
WHERE u.deleted_at IS NULL
  AND u.period_type <> lower(u.period_type)
  AND EXISTS (
      SELECT 1 FROM budgets l
      WHERE l.deleted_at IS NULL
        AND l.user_id = u.user_id
        AND l.category_id = u.category_id
        AND l.period_type = lower(u.period_type)
        AND l.period_start = u.period_start
  );

UPDATE budgets
SET period_type = lower(period_type)
WHERE period_type <> lower(period_type);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'budgets'::regclass AND conname = 'period_type_check'
    ) THEN
        ALTER TABLE budgets
            ADD CONSTRAINT period_type_check CHECK (period_type IN ('monthly', 'yearly')) NOT VALID;
        ALTER TABLE budgets VALIDATE CONSTRAINT period_type_check;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Lowercased values are not restored (they were never valid). The constraint
-- is kept if it predates this migration.
SELECT 1;
