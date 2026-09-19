-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    tbl text;
    tables_to_add text[] := ARRAY['transactions', 'wallets', 'categories', 'budgets', 'debts', 'credit_card_statements'];
BEGIN
    IF EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'supabase_realtime') THEN
        FOREACH tbl IN ARRAY tables_to_add LOOP
            IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = tbl) THEN
                IF NOT EXISTS (
                    SELECT 1 FROM pg_publication_tables 
                    WHERE pubname = 'supabase_realtime' AND schemaname = 'public' AND tablename = tbl
                ) THEN
                    EXECUTE format('ALTER PUBLICATION supabase_realtime ADD TABLE public.%I', tbl);
                END IF;
            END IF;
        END LOOP;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    -- No-op for down to preserve publication
END $$;
-- +goose StatementEnd
