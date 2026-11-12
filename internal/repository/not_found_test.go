package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

// notFoundDB is a dry-run DB where every First/Take finds nothing, as on a
// real database with no matching row.
func notFoundDB(t *testing.T) (*gorm.DB, *[]string) {
	t.Helper()
	db, statements := dryRunDB(t)
	require.NoError(t, db.Callback().Query().After("test:capture_query").Register("test:not_found", func(tx *gorm.DB) {
		if tx.Statement.RaiseErrorOnNotFound {
			_ = tx.AddError(gorm.ErrRecordNotFound)
		}
	}))
	return db, statements
}

// Every by-id repository lookup takes (ctx, userID, id), scopes the query to
// both, and reports a missing row as an error matching constant.ErrNotFound
// (never (nil, nil) or a raw gorm.ErrRecordNotFound). Distinct userID/id
// values catch an argument swap.
func TestByIDLookups_OrderScopeAndNotFound(t *testing.T) {
	ctx := context.Background()
	userID, id := uuid.New(), uuid.New()
	scope := "id = '" + id.String() + "' AND user_id = '" + userID.String() + "'"

	cases := []struct {
		name  string
		table string
		call  func(db *gorm.DB) (interface{}, error)
		want  error // the specific sentinel, when the repository has one
	}{
		{"wallet", "wallets", func(db *gorm.DB) (interface{}, error) { return NewWalletRepository(db).GetByIDForUser(ctx, userID, id) }, ErrWalletNotFound},
		{"statement", "credit_card_statements", func(db *gorm.DB) (interface{}, error) {
			return NewCreditCardStatementRepository(db).GetByIDForUser(ctx, userID, id)
		}, ErrStatementNotFound},
		{"debt", "debts", func(db *gorm.DB) (interface{}, error) { return NewDebtRepository(db).GetByIDForUser(ctx, userID, id) }, ErrDebtNotFound},
		{"category", "categories", func(db *gorm.DB) (interface{}, error) { return NewCategoryRepo(db).GetByIDForUser(ctx, userID, id) }, nil},
		{"cost", "costs", func(db *gorm.DB) (interface{}, error) { return NewCostRepo(db).GetByIDForUser(ctx, userID, id) }, nil},
		{"preset", "presets", func(db *gorm.DB) (interface{}, error) { return NewPresetRepository(db).GetByIDForUser(ctx, userID, id) }, nil},
		{"budget", "budgets", func(db *gorm.DB) (interface{}, error) { return NewBudgetRepository(db).GetByIDForUser(ctx, userID, id) }, nil},
		{"transaction", "transactions", func(db *gorm.DB) (interface{}, error) {
			return NewTransactionRepository(db).GetByIDForUser(ctx, userID, id)
		}, nil},
		{"chat session", "chat_sessions", func(db *gorm.DB) (interface{}, error) { return NewChatRepository(db).GetSessionByID(ctx, userID, id) }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, statements := notFoundDB(t)
			got, err := tc.call(db)
			require.ErrorIs(t, err, constant.ErrNotFound)
			assert.NotErrorIs(t, err, gorm.ErrRecordNotFound, "the gorm error is translated")
			if tc.want != nil {
				assert.ErrorIs(t, err, tc.want)
			}
			assert.Nil(t, got)
			assert.Contains(t, onlyStatement(t, *statements, `SELECT * FROM "`+tc.table+`"`), scope)
		})
	}

	t.Run("statement payment", func(t *testing.T) {
		db, statements := notFoundDB(t)
		got, err := NewCreditCardStatementRepository(db).PayStatement(ctx, userID, id, 1, nil)
		require.ErrorIs(t, err, ErrStatementNotFound)
		assert.Nil(t, got)
		assert.Contains(t, onlyStatement(t, *statements, `SELECT * FROM "credit_card_statements"`), scope)
	})

	t.Run("debt update", func(t *testing.T) {
		db, statements := notFoundDB(t)
		_, err := NewDebtRepository(db).UpdateWithWallet(ctx, userID, id, func(*model.Debt) error { return nil })
		require.ErrorIs(t, err, ErrDebtNotFound)
		assert.Contains(t, onlyStatement(t, *statements, `SELECT * FROM "debts"`), scope)
	})

	t.Run("debt delete", func(t *testing.T) {
		db, statements := notFoundDB(t)
		require.ErrorIs(t, NewDebtRepository(db).Delete(ctx, userID, id), ErrDebtNotFound)
		assert.Contains(t, onlyStatement(t, *statements, `SELECT * FROM "debts"`), scope)
	})

	t.Run("targets", func(t *testing.T) {
		db, _ := notFoundDB(t)
		_, err := NewTargetRepository(db).GetTarget(ctx, userID, model.TargetTypeExpense, 11, 2026)
		require.ErrorIs(t, err, ErrTargetNotFound)
		_, err = NewRolloverRepository(db).GetLatestTargetBefore(ctx, userID, model.TargetTypeExpense, 11, 2026)
		require.ErrorIs(t, err, ErrTargetNotFound)
	})

	t.Run("chat message and budget period", func(t *testing.T) {
		db, _ := notFoundDB(t)
		_, err := NewChatRepository(db).GetMessageByID(ctx, userID, id)
		require.ErrorIs(t, err, constant.ErrNotFound)
		_, err = NewBudgetRepository(db).GetByCategoryAndPeriod(ctx, userID, id, model.Budget{}.PeriodStart)
		require.ErrorIs(t, err, constant.ErrNotFound)
	})
}

// Deletes (soft) are scoped to (userID, id): a dry run affects no row, so each
// reports not found, and the generated statement carries both ids in order.
func TestDeletes_OrderAndScope(t *testing.T) {
	ctx := context.Background()
	userID, id := uuid.New(), uuid.New()
	scope := "id = '" + id.String() + "' AND user_id = '" + userID.String() + "'"

	db, statements := dryRunDB(t)
	captureDeletes(t, db, statements)
	assert.ErrorIs(t, NewWalletRepository(db).Delete(ctx, userID, id), constant.ErrNotFound)
	assert.ErrorIs(t, NewCreditCardStatementRepository(db).Delete(ctx, userID, id), constant.ErrNotFound)
	// An unknown (or someone else's) chat session is reported as not found → 404.
	assert.ErrorIs(t, NewChatRepository(db).DeleteSession(ctx, userID, id), constant.ErrNotFound)
	for _, table := range []string{"wallets", "credit_card_statements", "chat_sessions"} {
		var found bool
		for _, s := range *statements {
			if strings.Contains(s, `"`+table+`"`) && strings.Contains(s, scope) {
				found = true
			}
		}
		assert.True(t, found, "%s delete must be scoped to (id, user_id): %v", table, *statements)
	}
}
