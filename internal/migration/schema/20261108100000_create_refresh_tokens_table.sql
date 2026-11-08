-- Refresh tokens for sessions issued by POST /auth/login (local accounts;
-- Supabase accounts refresh through Supabase Auth).
--
-- Only the SHA-256 of a token is stored. Every refresh rotates the token: the
-- old row is revoked and points at its successor (replaced_by), and the new
-- row keeps the same family_id. Presenting a revoked token again means it was
-- copied, so the whole family (that login session) is revoked.
-- Rows are hard-deleted once expired; there is no soft delete.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_id UUID NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    replaced_by UUID,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON refresh_tokens(family_id);

-- Same lockdown as 20261028100000_enable_rls_on_public_tables: only the Go
-- API (table owner) may touch this table, never PostgREST.
ALTER TABLE refresh_tokens ENABLE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon')
        AND EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        REVOKE ALL ON TABLE refresh_tokens FROM anon, authenticated;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS refresh_tokens;
-- +goose StatementEnd
