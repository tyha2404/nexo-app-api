-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'supabase_realtime') THEN
        ALTER PUBLICATION supabase_realtime ADD TABLE public.transactions;
        ALTER PUBLICATION supabase_realtime ADD TABLE public.wallets;
        ALTER PUBLICATION supabase_realtime ADD TABLE public.categories;
        ALTER PUBLICATION supabase_realtime ADD TABLE public.budgets;
        ALTER PUBLICATION supabase_realtime ADD TABLE public.debts;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'supabase_realtime') THEN
        ALTER PUBLICATION supabase_realtime DROP TABLE IF EXISTS public.transactions;
        ALTER PUBLICATION supabase_realtime DROP TABLE IF EXISTS public.wallets;
        ALTER PUBLICATION supabase_realtime DROP TABLE IF EXISTS public.categories;
        ALTER PUBLICATION supabase_realtime DROP TABLE IF EXISTS public.budgets;
        ALTER PUBLICATION supabase_realtime DROP TABLE IF EXISTS public.debts;
    END IF;
END $$;
-- +goose StatementEnd

