package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
)

// Integration test (skipped unless NEXO_TEST_DATABASE_URL is set).
func TestCreditCardStatementDelete_NotFound(t *testing.T) {
	db := testDatabase(t)
	repo := NewCreditCardStatementRepository(db)
	ctx := context.Background()
	owner, stranger, id := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO credit_card_statements (id,user_id,wallet_id,statement_month,statement_year,statement_date,due_date,statement_balance,status)
		VALUES (?,?,?,11,2026,'2026-11-20','2026-12-05',1000000,'UNPAID')`, id, owner, uuid.New()).Error)

	require.ErrorIs(t, repo.Delete(ctx, stranger, id), constant.ErrNotFound, "another user's statement")
	require.ErrorIs(t, repo.Delete(ctx, owner, uuid.New()), constant.ErrNotFound, "unknown id")
	require.NoError(t, repo.Delete(ctx, owner, id))
	require.ErrorIs(t, repo.Delete(ctx, owner, id), constant.ErrNotFound, "already deleted")
}
