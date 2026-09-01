-- +goose Up
CREATE TABLE IF NOT EXISTS credit_card_statements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    statement_month INT NOT NULL,
    statement_year INT NOT NULL,
    statement_date DATE NOT NULL,
    due_date DATE NOT NULL,
    statement_balance NUMERIC(15,2) NOT NULL DEFAULT 0.00,
    minimum_payment NUMERIC(15,2) NOT NULL DEFAULT 0.00,
    previous_balance NUMERIC(15,2) NOT NULL DEFAULT 0.00,
    paid_amount NUMERIC(15,2) NOT NULL DEFAULT 0.00,
    status VARCHAR(20) NOT NULL DEFAULT 'UNPAID',
    note TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_cc_statements_user_wallet ON credit_card_statements(user_id, wallet_id);
CREATE INDEX IF NOT EXISTS idx_cc_statements_year_month ON credit_card_statements(statement_year, statement_month);

-- +goose Down
DROP TABLE IF EXISTS credit_card_statements;
