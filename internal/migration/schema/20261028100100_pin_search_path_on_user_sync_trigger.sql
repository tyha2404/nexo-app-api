-- handle_new_supabase_user is SECURITY DEFINER. Pin its search_path so a
-- caller-controlled search_path can't make it resolve objects the caller
-- planted. Its body already schema-qualifies every table (public.*); the
-- built-ins it calls live in pg_catalog, which is always searched.

-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF to_regprocedure('public.handle_new_supabase_user()') IS NOT NULL THEN
        ALTER FUNCTION public.handle_new_supabase_user() SET search_path = '';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF to_regprocedure('public.handle_new_supabase_user()') IS NOT NULL THEN
        ALTER FUNCTION public.handle_new_supabase_user() RESET search_path;
    END IF;
END $$;
-- +goose StatementEnd
