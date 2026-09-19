-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

-- 1. Upgrade financial_knowledges if table exists
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'financial_knowledges' AND column_name = 'embedding'
    ) THEN
        IF (SELECT data_type FROM information_schema.columns WHERE table_name = 'financial_knowledges' AND column_name = 'embedding') = 'text' THEN
            ALTER TABLE financial_knowledges ALTER COLUMN embedding DROP NOT NULL;
            ALTER TABLE financial_knowledges ALTER COLUMN embedding TYPE vector(768) USING NULL;
        END IF;
    END IF;
END $$;
-- +goose StatementEnd

-- 2. Add embedding column to transactions for semantic search
ALTER TABLE transactions 
    ADD COLUMN IF NOT EXISTS embedding vector(768) NULL;

-- 3. Create HNSW indexes
CREATE INDEX IF NOT EXISTS idx_transactions_embedding_hnsw 
    ON transactions USING hnsw (embedding vector_cosine_ops);

-- 4. Create match_financial_knowledge RPC
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION match_financial_knowledge(
    query_embedding vector(768),
    match_threshold float,
    match_count int
)
RETURNS TABLE (
    id UUID,
    topic VARCHAR,
    title VARCHAR,
    content TEXT,
    similarity float
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT
        fk.id,
        fk.topic,
        fk.title,
        fk.content,
        (1 - (fk.embedding <=> query_embedding))::float AS similarity
    FROM financial_knowledges fk
    WHERE fk.embedding IS NOT NULL
      AND 1 - (fk.embedding <=> query_embedding) > match_threshold
    ORDER BY fk.embedding <=> query_embedding
    LIMIT match_count;
END;
$$;
-- +goose StatementEnd

-- 5. Create match_transactions RPC
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION match_transactions(
    p_user_id UUID,
    query_embedding vector(768),
    match_threshold float,
    match_count int
)
RETURNS TABLE (
    id UUID,
    user_id UUID,
    description TEXT,
    amount NUMERIC,
    type VARCHAR,
    category_id UUID,
    wallet_id UUID,
    transaction_date DATE,
    receipt_url VARCHAR,
    similarity float
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT
        t.id,
        t.user_id,
        t.description,
        t.amount,
        t.type::VARCHAR,
        t.category_id,
        t.wallet_id,
        t.transaction_date,
        t.receipt_url,
        (1 - (t.embedding <=> query_embedding))::float AS similarity
    FROM transactions t
    WHERE t.user_id = p_user_id
      AND t.deleted_at IS NULL
      AND t.embedding IS NOT NULL
      AND 1 - (t.embedding <=> query_embedding) > match_threshold
    ORDER BY t.embedding <=> query_embedding
    LIMIT match_count;
END;
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS match_transactions(UUID, vector, float, int);
DROP FUNCTION IF EXISTS match_financial_knowledge(vector, float, int);
DROP INDEX IF EXISTS idx_transactions_embedding_hnsw;
ALTER TABLE transactions DROP COLUMN IF EXISTS embedding;
