-- Lock down Supabase's auto-generated REST/GraphQL API (PostgREST) and
-- Realtime. The anon key ships in the web bundle, and Supabase grants
-- anon/authenticated full access to tables in `public` by default, so without
-- this anyone could read or modify every user's rows, including users.role and
-- users.password.
--
-- The Go API is unaffected: it connects as the owner of these tables, and
-- owners bypass RLS (FORCE ROW LEVEL SECURITY is deliberately not used).
-- Clients only use Supabase directly for Realtime on the tables listed in
-- realtime_tables below; every read and write goes through the Go API.
--
-- On a plain Postgres (local dev, test DBs) the Supabase roles do not exist
-- and only RLS is enabled.

-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    tbl text;
    fn regprocedure;
    has_supabase_roles boolean := EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon')
        AND EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated');
    has_auth_uid boolean := to_regprocedure('auth.uid()') IS NOT NULL;
    realtime_tables text[] := ARRAY['transactions', 'wallets', 'categories', 'budgets', 'debts', 'credit_card_statements'];
BEGIN
    -- 1. RLS on every table in public. With no policy, RLS denies everything
    --    to non-owners.
    FOR tbl IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', tbl);
        IF has_supabase_roles THEN
            EXECUTE format('REVOKE ALL ON TABLE public.%I FROM anon, authenticated', tbl);
        END IF;
    END LOOP;

    IF has_supabase_roles THEN
        -- 2. Sequences and the search RPCs. match_transactions takes any
        --    p_user_id, so it must not be callable through /rpc. (The sync
        --    trigger function can't be called through /rpc: it returns trigger.)
        EXECUTE 'REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM anon, authenticated';
        FOR fn IN
            SELECT p.oid::regprocedure FROM pg_proc p
            WHERE p.pronamespace = 'public'::regnamespace
              AND p.proname IN ('match_transactions', 'match_financial_knowledge')
        LOOP
            EXECUTE format('REVOKE EXECUTE ON FUNCTION %s FROM PUBLIC, anon, authenticated', fn);
        END LOOP;

        -- 3. Tables and functions created by later migrations (run as this
        --    same role) start without grants to the Supabase API roles.
        EXECUTE 'ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON TABLES FROM anon, authenticated';
        EXECUTE 'ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON SEQUENCES FROM anon, authenticated';
        EXECUTE 'ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON FUNCTIONS FROM anon, authenticated';
    END IF;

    -- 4. Realtime: a signed-in user may read (and so receive changes for)
    --    only their own rows. Realtime checks these policies per subscriber.
    IF has_supabase_roles AND has_auth_uid THEN
        FOREACH tbl IN ARRAY realtime_tables LOOP
            IF to_regclass(format('public.%I', tbl)) IS NOT NULL THEN
                EXECUTE format('GRANT SELECT ON TABLE public.%I TO authenticated', tbl);
                EXECUTE format('DROP POLICY IF EXISTS realtime_select_own ON public.%I', tbl);
                EXECUTE format(
                    'CREATE POLICY realtime_select_own ON public.%I FOR SELECT TO authenticated USING (user_id = (SELECT auth.uid()))',
                    tbl);
            END IF;
        END LOOP;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Restores Supabase's default (open) grants. This re-exposes every table to
-- the public anon key; only roll back temporarily.
-- +goose StatementBegin
DO $$
DECLARE
    tbl text;
    has_supabase_roles boolean := EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon')
        AND EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated');
BEGIN
    FOR tbl IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' LOOP
        EXECUTE format('DROP POLICY IF EXISTS realtime_select_own ON public.%I', tbl);
        EXECUTE format('ALTER TABLE public.%I DISABLE ROW LEVEL SECURITY', tbl);
        IF has_supabase_roles THEN
            EXECUTE format('GRANT ALL ON TABLE public.%I TO anon, authenticated', tbl);
        END IF;
    END LOOP;

    IF has_supabase_roles THEN
        EXECUTE 'GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO anon, authenticated';
        EXECUTE 'GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO anon, authenticated';
        EXECUTE 'ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO anon, authenticated';
        EXECUTE 'ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO anon, authenticated';
        EXECUTE 'ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON FUNCTIONS TO anon, authenticated';
    END IF;
END $$;
-- +goose StatementEnd
