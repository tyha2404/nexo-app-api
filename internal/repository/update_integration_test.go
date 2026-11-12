package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

// Integration tests (skipped unless NEXO_TEST_DATABASE_URL is set).

func TestUpdatesNeverResurrectSoftDeletedRows(t *testing.T) {
	db := testDatabase(t)
	ctx := context.Background()
	user := uuid.New()

	walletID := uuid.New()
	require.NoError(t, db.Exec("INSERT INTO wallets (id,user_id,name,type,balance,deleted_at) VALUES (?,?,'Ví','CASH',100,now())", walletID, user).Error)
	err := NewWalletRepository(db).Update(ctx, &model.Wallet{ID: walletID, UserID: user, Name: "Ví mới", Type: model.WalletTypeCash}, true)
	require.ErrorIs(t, err, constant.ErrNotFound)

	budgetID := uuid.New()
	require.NoError(t, db.Exec("INSERT INTO budgets (id,user_id,category_id,amount,period_type,period_start,deleted_at) VALUES (?,?,?,100,'monthly','2026-11-01',now())", budgetID, user, uuid.New()).Error)
	err = NewBudgetRepository(db).Update(ctx, &model.Budget{ID: budgetID, UserID: user, Amount: 5, PeriodType: model.BudgetPeriodMonthly, PeriodStart: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)})
	require.ErrorIs(t, err, constant.ErrNotFound)

	var live int64
	require.NoError(t, db.Raw("SELECT count(*) FROM wallets WHERE deleted_at IS NULL").Scan(&live).Error)
	require.Zero(t, live)
	require.NoError(t, db.Raw("SELECT count(*) FROM budgets WHERE deleted_at IS NULL").Scan(&live).Error)
	require.Zero(t, live)

	// Another user's live wallet is not updated either.
	theirs := uuid.New()
	require.NoError(t, db.Exec("INSERT INTO wallets (id,user_id,name,type,balance) VALUES (?,?,'Của họ','CASH',100)", theirs, uuid.New()).Error)
	err = NewWalletRepository(db).Update(ctx, &model.Wallet{ID: theirs, UserID: user, Name: "Chiếm", Type: model.WalletTypeCash}, false)
	require.ErrorIs(t, err, constant.ErrNotFound)
	var name string
	require.NoError(t, db.Raw("SELECT name FROM wallets WHERE id = ?", theirs).Scan(&name).Error)
	require.Equal(t, "Của họ", name)
}

func TestStatementUpdateKeepsConcurrentPayment(t *testing.T) {
	db := testDatabase(t)
	repo := NewCreditCardStatementRepository(db)
	ctx := context.Background()
	user, id := uuid.New(), uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO credit_card_statements (id,user_id,wallet_id,statement_month,statement_year,statement_date,due_date,statement_balance,paid_amount,status)
		VALUES (?,?,?,11,2026,'2026-11-20','2026-12-05',1000000,0,'UNPAID')`, id, user, uuid.New()).Error)
	// A payment committed after the client loaded the statement.
	require.NoError(t, db.Exec("UPDATE credit_card_statements SET paid_amount = 400000, status = 'PARTIALLY_PAID' WHERE id = ?", id).Error)

	updated, err := repo.Update(ctx, user, id, func(s *model.CreditCardStatement) error {
		s.Note = "sửa ghi chú"
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 400000.0, updated.PaidAmount, "the locked paid_amount is kept")
	require.Equal(t, "sửa ghi chú", updated.Note)

	_, err = repo.Update(ctx, uuid.New(), id, func(*model.CreditCardStatement) error { return nil })
	require.ErrorIs(t, err, constant.ErrNotFound, "another user's statement")
}

func TestBudgetListActiveByUserID(t *testing.T) {
	db := testDatabase(t)
	repo := NewBudgetRepository(db)
	user, food := uuid.New(), uuid.New()
	for _, start := range []string{"2026-09-01", "2026-10-01", "2026-11-01"} {
		require.NoError(t, db.Exec("INSERT INTO budgets (id,user_id,category_id,amount,period_type,period_start) VALUES (?,?,?,100,'monthly',?)", uuid.New(), user, food, start).Error)
	}
	require.NoError(t, db.Exec("INSERT INTO budgets (id,user_id,category_id,amount,period_type,period_start) VALUES (?,?,?,100,'monthly','2026-10-01')", uuid.New(), uuid.New(), food).Error)

	got, err := repo.ListActiveByUserID(context.Background(), user, time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, got, 1, "only October's budget of this user")
	require.Equal(t, "2026-10-01", got[0].PeriodStart.Format("2006-01-02"))
}
