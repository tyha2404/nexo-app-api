package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// dryPool is a gorm.ConnPool that never reaches a database. With DryRun GORM
// builds every statement without executing it (so RowsAffected is 0), which
// lets these tests assert the generated SQL without NEXO_TEST_DATABASE_URL.
type dryPool struct{}

func (*dryPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry run")
}
func (*dryPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry run")
}
func (*dryPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry run")
}
func (*dryPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row { return nil }
func (p *dryPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return p, nil
}
func (*dryPool) Commit() error   { return nil }
func (*dryPool) Rollback() error { return nil }

// dryRunDB returns a DryRun *gorm.DB and the SQL (with values inlined) of
// every statement it builds.
func dryRunDB(t *testing.T) (*gorm.DB, *[]string) {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: &dryPool{}}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	require.NoError(t, err)
	var statements []string
	capture := func(tx *gorm.DB) {
		statements = append(statements, tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...))
	}
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:capture_query", capture))
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("test:capture_update", capture))
	require.NoError(t, db.Callback().Create().After("gorm:create").Register("test:capture_create", capture))
	return db, &statements
}

func onlyStatement(t *testing.T, statements []string, prefix string) string {
	t.Helper()
	var found []string
	for _, s := range statements {
		if strings.HasPrefix(s, prefix) {
			found = append(found, s)
		}
	}
	require.Len(t, found, 1, "statements: %v", statements)
	return found[0]
}

// setClause is the "SET ..." part of an UPDATE (before WHERE).
func setClause(t *testing.T, update string) string {
	t.Helper()
	i, j := strings.Index(update, " SET "), strings.Index(update, " WHERE ")
	require.True(t, i > 0 && j > i, update)
	return update[i:j]
}

func assertNoInsert(t *testing.T, statements []string) {
	t.Helper()
	for _, s := range statements {
		assert.False(t, strings.HasPrefix(s, "INSERT"), "an update must never upsert a (soft-deleted) row: %s", s)
	}
}

func TestPresetUpdate_ScopedColumnUpdate(t *testing.T) {
	db, statements := dryRunDB(t)
	p := &model.Preset{ID: uuid.New(), UserID: uuid.New(), CategoryID: uuid.New(), Name: "Cà phê", Amount: 30000, Type: "EXPENSE", SortOrder: 0}

	err := NewPresetRepository(db).Update(context.Background(), p)
	require.ErrorIs(t, err, constant.ErrNotFound, "0 rows affected = missing, deleted or another user's preset")

	update := onlyStatement(t, *statements, "UPDATE")
	assert.Contains(t, update, fmt.Sprintf("id = '%s' AND user_id = '%s'", p.ID, p.UserID))
	assert.Contains(t, update, `"presets"."deleted_at" IS NULL`)
	set := setClause(t, update)
	assert.Contains(t, set, `"sort_order"=0`, "zero values are written")
	for _, col := range []string{`"user_id"`, `"created_at"`, `"deleted_at"`, `"id"`} {
		assert.NotContains(t, set, col)
	}
	assertNoInsert(t, *statements)
}

func TestBudgetUpdate_ScopedColumnUpdate(t *testing.T) {
	db, statements := dryRunDB(t)
	b := &model.Budget{ID: uuid.New(), UserID: uuid.New(), CategoryID: uuid.New(), Amount: 0, PeriodType: model.BudgetPeriodMonthly,
		PeriodStart: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Category: model.Category{ID: uuid.New(), Name: "Ăn uống"}}

	err := NewBudgetRepository(db).Update(context.Background(), b)
	require.ErrorIs(t, err, constant.ErrNotFound)

	update := onlyStatement(t, *statements, "UPDATE")
	assert.True(t, strings.HasPrefix(update, `UPDATE "budgets"`), "associations are not saved: %s", update)
	assert.Contains(t, update, fmt.Sprintf("id = '%s' AND user_id = '%s'", b.ID, b.UserID))
	assert.Contains(t, update, `"budgets"."deleted_at" IS NULL`)
	set := setClause(t, update)
	assert.Contains(t, set, `"amount"=0`)
	assert.Contains(t, set, `"period_type"='monthly'`)
	assert.NotContains(t, set, `"user_id"`)
	assert.NotContains(t, set, `"deleted_at"`)
	assertNoInsert(t, *statements)
}

func TestWalletUpdate_ScopedColumnUpdate(t *testing.T) {
	db, statements := dryRunDB(t)
	w := &model.Wallet{ID: uuid.New(), UserID: uuid.New(), Name: "Ví", Balance: 123, AllocationPercent: 0, IsIncludedInTotal: false}

	err := NewWalletRepository(db).Update(context.Background(), w, false)
	require.ErrorIs(t, err, constant.ErrNotFound)
	update := onlyStatement(t, *statements, "UPDATE")
	assert.Contains(t, update, fmt.Sprintf("id = '%s' AND user_id = '%s'", w.ID, w.UserID))
	assert.Contains(t, update, `"wallets"."deleted_at" IS NULL`)
	set := setClause(t, update)
	assert.Contains(t, set, `"is_included_in_total"=false`, "false is written")
	assert.Contains(t, set, `"allocation_percent"=0`)
	assert.NotContains(t, set, `"balance"`, "the stale balance is not written")
	assert.NotContains(t, set, `"user_id"`)
	assert.NotContains(t, set, `"deleted_at"`)
	assertNoInsert(t, *statements)

	db, statements = dryRunDB(t)
	_ = NewWalletRepository(db).Update(context.Background(), w, true)
	assert.Contains(t, setClause(t, onlyStatement(t, *statements, "UPDATE")), `"balance"=123`)
}

func TestCreditCardStatementUpdate_LocksAndWritesEditableColumns(t *testing.T) {
	db, statements := dryRunDB(t)
	userID, id := uuid.New(), uuid.New()
	mutated := false

	_, err := NewCreditCardStatementRepository(db).Update(context.Background(), userID, id, func(s *model.CreditCardStatement) error {
		mutated = true
		s.Note = "sửa"
		s.WalletID = uuid.New() // not editable: must not be written
		s.UserID = uuid.New()   // reset by Update
		return nil
	})
	require.ErrorIs(t, err, constant.ErrNotFound, "0 rows affected")
	require.True(t, mutated)

	var locked string
	for _, s := range *statements {
		if strings.HasSuffix(s, "FOR UPDATE") {
			locked = s
		}
	}
	require.NotEmpty(t, locked, "the row is read FOR UPDATE: %v", *statements)
	assert.Contains(t, locked, fmt.Sprintf("id = '%s' AND user_id = '%s'", id, userID))

	update := onlyStatement(t, *statements, "UPDATE")
	assert.Contains(t, update, fmt.Sprintf("id = '%s' AND user_id = '%s'", id, userID))
	assert.Contains(t, update, `"credit_card_statements"."deleted_at" IS NULL`)
	set := setClause(t, update)
	assert.Contains(t, set, `"note"='sửa'`)
	assert.Contains(t, set, `"paid_amount"=`)
	for _, col := range []string{`"wallet_id"`, `"user_id"`, `"created_at"`, `"deleted_at"`} {
		assert.NotContains(t, set, col)
	}
	assertNoInsert(t, *statements)

	// mutate's error aborts before anything is written.
	db, statements = dryRunDB(t)
	boom := errors.New("boom")
	_, err = NewCreditCardStatementRepository(db).Update(context.Background(), userID, id, func(*model.CreditCardStatement) error { return boom })
	require.ErrorIs(t, err, boom)
	for _, s := range *statements {
		assert.False(t, strings.HasPrefix(s, "UPDATE"), s)
	}
}

func TestBudgetListActiveByUserID_SQL(t *testing.T) {
	db, statements := dryRunDB(t)
	userID := uuid.New()
	at := time.Date(2026, 11, 30, 23, 30, 0, 0, time.FixedZone("ICT", 7*3600))

	_, err := NewBudgetRepository(db).ListActiveByUserID(context.Background(), userID, at)
	require.NoError(t, err)
	q := onlyStatement(t, *statements, `SELECT * FROM "budgets"`)
	assert.Contains(t, q, fmt.Sprintf("user_id = '%s'", userID))
	assert.Contains(t, q, "period_start <= CAST('2026-11-30' AS date)", "the calendar day of at, not a timestamp")
	assert.Contains(t, q, "((period_type = 'monthly' AND CAST('2026-11-30' AS date) < period_start + INTERVAL '1 month') OR (period_type = 'yearly' AND CAST('2026-11-30' AS date) < period_start + INTERVAL '1 year'))")
	assert.Contains(t, q, `"budgets"."deleted_at" IS NULL`)
	assert.Contains(t, q, "ORDER BY period_start DESC")
}

func TestUniqueViolationMapping(t *testing.T) {
	unique := fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505", ConstraintName: "idx_user_category_name"})
	other := &pgconn.PgError{Code: "23503"}

	assert.ErrorIs(t, mapCategoryUniqueViolation(unique), constant.ErrAlreadyExists)
	assert.ErrorIs(t, mapStatementUniqueViolation(unique), constant.ErrAlreadyExists)
	assert.ErrorIs(t, mapBudgetUniqueViolation(unique), constant.ErrAlreadyExists)
	assert.Equal(t, error(other), mapCategoryUniqueViolation(other), "other errors pass through")
	assert.NoError(t, mapCategoryUniqueViolation(nil))
}
