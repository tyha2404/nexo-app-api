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

// captureDeletes also records the statements built by the Delete callbacks
// (a soft delete is an UPDATE ... SET deleted_at).
func captureDeletes(t *testing.T, db *gorm.DB, statements *[]string) {
	t.Helper()
	require.NoError(t, db.Callback().Delete().After("gorm:delete").Register("test:capture_delete", func(tx *gorm.DB) {
		*statements = append(*statements, tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...))
	}))
}

func TestByIDQueriesAreScopedToTheUser(t *testing.T) {
	ctx := context.Background()
	userID, id := uuid.New(), uuid.New()
	scope := "id = '" + id.String() + "' AND user_id = '" + userID.String() + "'"

	t.Run("get", func(t *testing.T) {
		db, statements := dryRunDB(t)
		_, _ = NewPresetRepository(db).GetByIDForUser(ctx, userID, id)
		_, _ = NewBudgetRepository(db).GetByIDForUser(ctx, userID, id)
		_, _ = NewTransactionRepository(db).GetByIDForUser(ctx, userID, id)
		for _, table := range []string{"presets", "budgets", "transactions"} {
			assert.Contains(t, onlyStatement(t, *statements, `SELECT * FROM "`+table+`"`), scope)
		}
	})

	t.Run("delete", func(t *testing.T) {
		db, statements := dryRunDB(t)
		captureDeletes(t, db, statements)
		// Dry run: nothing is affected, so each reports not found.
		assert.ErrorIs(t, NewPresetRepository(db).Delete(ctx, userID, id), constant.ErrNotFound)
		assert.ErrorIs(t, NewBudgetRepository(db).Delete(ctx, userID, id), constant.ErrNotFound)
		assert.ErrorIs(t, NewAlertRepository(db).Delete(ctx, userID, id), constant.ErrNotFound)
		for _, table := range []string{"presets", "budgets", "alerts"} {
			stmt := onlyStatement(t, *statements, `UPDATE "`+table+`" SET "deleted_at"`)
			assert.Contains(t, stmt, scope)
		}
	})

	t.Run("chat messages", func(t *testing.T) {
		db, statements := dryRunDB(t)
		repo := NewChatRepository(db)
		_, _ = repo.GetMessageByID(ctx, userID, id)
		_ = repo.UpdateMessage(ctx, userID, id, "hi", model.MessageStatusSuccess)
		_ = repo.UpdateMessageToolResults(ctx, userID, id, nil)
		sub := "id = '" + id.String() + "' AND session_id IN (SELECT id FROM chat_sessions WHERE user_id = '" + userID.String() + "')"
		require.Len(t, *statements, 3)
		for _, s := range *statements {
			assert.Contains(t, s, sub)
		}
	})
}

func TestNotFoundAs(t *testing.T) {
	err := notFoundAs("preset", gorm.ErrRecordNotFound)
	assert.ErrorIs(t, err, constant.ErrNotFound)
	assert.EqualError(t, err, "preset not found")
	other := assert.AnError
	assert.Same(t, other, notFoundAs("preset", other))
}

func TestTransactionFilters_SharedBySearch(t *testing.T) {
	db, statements := dryRunDB(t)
	userID, catID := uuid.New(), uuid.New()
	_, _, _ = NewTransactionRepository(db).SearchByUserID(context.Background(), userID, 10, 0, map[string]interface{}{
		"type":       "ALL",
		"keyword":    " 50%_off ",
		"categoryId": catID.String(),
		"startDate":  "2026-03-01",
		"endDate":    "not-a-date",
		"minAmount":  1000.0,
	})
	// The count query carries every filter of the search.
	require.NotEmpty(t, *statements)
	sel := (*statements)[0]
	require.True(t, strings.HasPrefix(sel, `SELECT count(*) FROM "transactions"`), sel)
	assert.Contains(t, sel, `transactions.description ILIKE '%50\%\_off%'`, "keyword trimmed and LIKE-escaped")
	assert.NotContains(t, sel, "transactions.type", `type "ALL" means any type`)
	assert.Contains(t, sel, "transactions.category_id = '"+catID.String()+"'")
	assert.Contains(t, sel, "transactions.transaction_date >= '2026-03-01 00:00:00'")
	assert.NotContains(t, sel, "transaction_date <=", "a malformed date is ignored")
	assert.Contains(t, sel, "amount >= 1000")
}
