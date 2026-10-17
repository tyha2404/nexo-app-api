package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

func TestApplyStatementPayment(t *testing.T) {
	paid, status, err := ApplyStatementPayment(5000000, 0, 2000000)
	require.NoError(t, err)
	require.Equal(t, 2000000.0, paid)
	require.Equal(t, model.StatementStatusPartiallyPaid, status)

	paid, status, err = ApplyStatementPayment(5000000, 2000000, 3000000)
	require.NoError(t, err)
	require.Equal(t, 5000000.0, paid)
	require.Equal(t, model.StatementStatusPaid, status)

	// Float noise at 2-decimal precision is not an overpayment.
	_, status, err = ApplyStatementPayment(0.3, 0.1, 0.2)
	require.NoError(t, err)
	require.Equal(t, model.StatementStatusPaid, status)

	for _, amount := range []float64{0, -1, 3000000.01} {
		_, _, err = ApplyStatementPayment(5000000, 2000000, amount)
		require.ErrorIs(t, err, ErrStatementInvalidPayment, "amount %v", amount)
	}
	_, _, err = ApplyStatementPayment(5000000, 5000000, 1)
	require.ErrorIs(t, err, ErrStatementInvalidPayment)
}

func TestDebtDeletionWalletReversals(t *testing.T) {
	walletA, walletB := uuid.New(), uuid.New()

	// Payable 1000 borrowed into A (+1000), repaid 300 from A and 200 from B.
	payable := &model.Debt{Type: model.DebtTypePayable, TotalAmount: 1000, WalletID: &walletA}
	got := DebtDeletionWalletReversals(payable, []model.Repayment{
		{Amount: 300, WalletID: &walletA},
		{Amount: 200, WalletID: &walletB},
		{Amount: 50}, // no wallet -> no effect
	})
	require.Equal(t, []WalletDelta{{WalletID: walletA, Delta: -1000 + 300}, {WalletID: walletB, Delta: 200}}, got)

	// Receivable 500 lent from A (-500), repaid 500 into A: net zero, nothing to do.
	receivable := &model.Debt{Type: model.DebtTypeReceivable, TotalAmount: 500, WalletID: &walletA}
	got = DebtDeletionWalletReversals(receivable, []model.Repayment{{Amount: 500, WalletID: &walletA}})
	require.Equal(t, []WalletDelta{{WalletID: walletA, Delta: 0}}, got)

	// Legacy debt without wallets.
	require.Empty(t, DebtDeletionWalletReversals(&model.Debt{Type: model.DebtTypePayable, TotalAmount: 10}, nil))
}

func TestSummarizeWalletsSeparatesCurrencies(t *testing.T) {
	s := SummarizeWallets([]model.Wallet{
		{Balance: 1000000, Currency: "VND", IsIncludedInTotal: true},
		{Balance: 500000, Currency: "", IsIncludedInTotal: true}, // legacy default VND
		{Balance: 200, Currency: "USD", IsIncludedInTotal: true},
		{Balance: 9999999, Currency: "VND", IsIncludedInTotal: false},
	})
	require.Equal(t, 1500000.0, s.TotalBalance)
	require.Equal(t, "VND", s.BaseCurrency)
	require.Equal(t, map[string]float64{"VND": 1500000, "USD": 200}, s.TotalsByCurrency)
	require.Equal(t, 4, s.TotalWallets)
	require.Len(t, s.Wallets, 4)
}

func TestSameWallet(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	a2 := a
	require.True(t, sameWallet(nil, nil))
	require.True(t, sameWallet(&a, &a2))
	require.False(t, sameWallet(&a, &b))
	require.False(t, sameWallet(&a, nil))
	require.False(t, sameWallet(nil, &b))
}

// ---- Integration tests (skipped unless NEXO_TEST_DATABASE_URL is set) ----

func walletBalance(t *testing.T, db *gorm.DB, id uuid.UUID) float64 {
	t.Helper()
	var b float64
	require.NoError(t, db.Raw("SELECT balance FROM wallets WHERE id = ?", id).Scan(&b).Error)
	return b
}

func TestTransactionWalletBalanceLifecycle(t *testing.T) {
	db := testDatabase(t)
	repo := NewTransactionRepository(db)
	ctx := context.Background()
	user, walletA, walletB, foreign := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, w := range []struct {
		id, user uuid.UUID
	}{{walletA, user}, {walletB, user}, {foreign, uuid.New()}} {
		require.NoError(t, db.Exec("INSERT INTO wallets (id,user_id,balance) VALUES (?,?,1000)", w.id, w.user).Error)
	}

	tx := &model.Transaction{UserID: user, CategoryID: uuid.New(), WalletID: &walletA, Amount: 200, Type: model.TransactionTypeExpense, TransactionDate: time.Now()}
	require.NoError(t, repo.Create(ctx, tx))
	require.Equal(t, 800.0, walletBalance(t, db, walletA))

	// Amount + type change on the same wallet: reverse -200, apply +300.
	tx.Amount, tx.Type = 300, model.TransactionTypeIncome
	require.NoError(t, repo.Update(ctx, tx))
	require.Equal(t, 1300.0, walletBalance(t, db, walletA))

	// Move to wallet B.
	tx.WalletID = &walletB
	require.NoError(t, repo.Update(ctx, tx))
	require.Equal(t, 1000.0, walletBalance(t, db, walletA))
	require.Equal(t, 1300.0, walletBalance(t, db, walletB))

	// Foreign wallet is rejected and nothing changes.
	tx.WalletID = &foreign
	require.ErrorIs(t, repo.Update(ctx, tx), constant.ErrNotFound)
	require.Equal(t, 1300.0, walletBalance(t, db, walletB))
	require.Equal(t, 1000.0, walletBalance(t, db, foreign))

	// Delete reverses (soft delete).
	require.NoError(t, repo.Delete(ctx, tx.ID))
	require.Equal(t, 1000.0, walletBalance(t, db, walletB))
	var deleted int64
	require.NoError(t, db.Raw("SELECT count(*) FROM transactions WHERE id = ? AND deleted_at IS NOT NULL", tx.ID).Scan(&deleted).Error)
	require.EqualValues(t, 1, deleted)

	// Create against a foreign wallet rolls back the insert.
	bad := &model.Transaction{UserID: user, CategoryID: uuid.New(), WalletID: &foreign, Amount: 1, Type: model.TransactionTypeExpense, TransactionDate: time.Now()}
	require.ErrorIs(t, repo.Create(ctx, bad), constant.ErrNotFound)
	var n int64
	require.NoError(t, db.Raw("SELECT count(*) FROM transactions WHERE id = ?", bad.ID).Scan(&n).Error)
	require.Zero(t, n)
}

func TestDebtDeleteReversesWalletEffects(t *testing.T) {
	db := testDatabase(t)
	repo := NewDebtRepository(db)
	ctx := context.Background()
	user, wallet := uuid.New(), uuid.New()
	require.NoError(t, db.Exec("INSERT INTO wallets (id,user_id,balance) VALUES (?,?,1000)", wallet, user).Error)

	debt := &model.Debt{UserID: user, Type: model.DebtTypePayable, TotalAmount: 500, Status: model.DebtStatusPending, WalletID: &wallet}
	require.NoError(t, repo.CreateWithWallet(ctx, debt, &wallet, model.DebtCreationWalletDelta(debt.Type, 500)))
	require.NoError(t, repo.AddRepaymentWithWallet(ctx, debt, &model.Repayment{DebtID: debt.ID, Amount: 200, PaidAt: time.Now(), WalletID: &wallet}, &wallet, model.DebtRepaymentWalletDelta(debt.Type, 200)))
	require.Equal(t, 1300.0, walletBalance(t, db, wallet))

	require.NoError(t, repo.Delete(ctx, debt.ID, user))
	require.Equal(t, 1000.0, walletBalance(t, db, wallet))
	require.ErrorIs(t, repo.Delete(ctx, debt.ID, user), ErrDebtNotFound)
}

func TestRepaymentRejectedOnCompletedDebtAndOverdueSummary(t *testing.T) {
	db := testDatabase(t)
	repo := NewDebtRepository(db)
	ctx := context.Background()
	user := uuid.New()
	past := time.Now().AddDate(0, 0, -3)
	done := model.Debt{ID: uuid.New(), UserID: user, Type: model.DebtTypePayable, TotalAmount: 100, PaidAmount: 100, Status: model.DebtStatusCompleted, DueDate: &past}
	stale := model.Debt{ID: uuid.New(), UserID: user, Type: model.DebtTypePayable, TotalAmount: 100, Status: model.DebtStatusPending, DueDate: &past}
	require.NoError(t, db.Create(&done).Error)
	require.NoError(t, db.Create(&stale).Error)

	require.ErrorIs(t, repo.AddRepayment(ctx, &done, &model.Repayment{DebtID: done.ID, Amount: 1, PaidAt: time.Now()}), ErrInvalidRepayment)

	summary, err := repo.GetSummaryByUserID(ctx, user)
	require.NoError(t, err)
	require.EqualValues(t, 1, summary.OverdueCount, "stored PENDING but past due counts as overdue")
	require.EqualValues(t, 0, summary.PendingCount)
}

func TestPayStatementAtomic(t *testing.T) {
	db := testDatabase(t)
	repo := NewCreditCardStatementRepository(db)
	ctx := context.Background()
	user, card, bank := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, db.Exec("INSERT INTO wallets (id,user_id,balance) VALUES (?,?,-5000), (?,?,10000)", card, user, bank, user).Error)
	stmt := model.CreditCardStatement{ID: uuid.New(), UserID: user, WalletID: card, StatementDate: time.Now(), DueDate: time.Now(), StatementBalance: 5000, Status: model.StatementStatusUnpaid}
	require.NoError(t, db.Create(&stmt).Error)

	missing := uuid.New()
	_, err := repo.PayStatement(ctx, user, stmt.ID, 1000, &missing)
	require.ErrorIs(t, err, constant.ErrNotFound)
	require.Equal(t, -5000.0, walletBalance(t, db, card), "rolled back")

	_, err = repo.PayStatement(ctx, user, stmt.ID, 6000, &bank)
	require.ErrorIs(t, err, ErrStatementInvalidPayment)

	res, err := repo.PayStatement(ctx, user, stmt.ID, 5000, &bank)
	require.NoError(t, err)
	require.Equal(t, model.StatementStatusPaid, res.Status)
	require.Equal(t, 0.0, walletBalance(t, db, card))
	require.Equal(t, 5000.0, walletBalance(t, db, bank))

	_, err = repo.PayStatement(ctx, user, stmt.ID, 1, &bank)
	require.ErrorIs(t, err, ErrStatementAlreadyPaid)
}

func TestRolloverLatestBudgetsDistinctAndIdempotentCreate(t *testing.T) {
	db := testDatabase(t)
	repo := NewRolloverRepository(db)
	ctx := context.Background()
	user, cat := uuid.New(), uuid.New()
	for i, amount := range []float64{100, 200, 300} {
		start := time.Date(2026, time.Month(6+i), 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, db.Create(&model.Budget{UserID: user, CategoryID: cat, Amount: amount, PeriodType: "monthly", PeriodStart: start}).Error)
	}
	target := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	latest, err := repo.GetLatestMonthlyBudgetsBefore(ctx, user, target)
	require.NoError(t, err)
	require.Len(t, latest, 1)
	require.Equal(t, 300.0, latest[0].Amount)

	b := []model.Budget{{UserID: user, CategoryID: cat, Amount: 300, PeriodType: "monthly", PeriodStart: target}}
	require.NoError(t, repo.CreateBudgets(ctx, b))
	b2 := []model.Budget{{UserID: user, CategoryID: cat, Amount: 300, PeriodType: "monthly", PeriodStart: target}}
	require.NoError(t, repo.CreateBudgets(ctx, b2), "second rollover is a no-op")
	var n int64
	require.NoError(t, db.Model(&model.Budget{}).Where("user_id = ? AND period_start = ?", user, target).Count(&n).Error)
	require.EqualValues(t, 1, n)
}

func TestWalletReversalOnlyForAppliedRows(t *testing.T) {
	w := uuid.New()
	legacy := &model.Transaction{WalletID: &w, Type: model.TransactionTypeExpense, Amount: 100, BalanceApplied: false}
	_, ok := walletReversal(legacy)
	require.False(t, ok, "legacy row: effect never applied, never reversed")

	applied := &model.Transaction{WalletID: &w, Type: model.TransactionTypeExpense, Amount: 100, BalanceApplied: true}
	delta, ok := walletReversal(applied)
	require.True(t, ok)
	require.Equal(t, 100.0, delta)

	_, ok = walletReversal(&model.Transaction{Type: model.TransactionTypeIncome, Amount: 5, BalanceApplied: true})
	require.False(t, ok, "no wallet, nothing to reverse")
}

func insertLegacyTransaction(t *testing.T, db *gorm.DB, user, wallet uuid.UUID, amount float64) *model.Transaction {
	t.Helper()
	tx := &model.Transaction{ID: uuid.New(), UserID: user, CategoryID: uuid.New(), WalletID: &wallet, Amount: amount, Type: model.TransactionTypeExpense, TransactionDate: time.Now()}
	// Insert directly, bypassing the repository: like a pre-migration row (balance_applied = false).
	require.NoError(t, db.Omit("Category", "Wallet", "User").Create(tx).Error)
	return tx
}

func TestLegacyTransactionEditAppliesWithoutReversal(t *testing.T) {
	db := testDatabase(t)
	repo := NewTransactionRepository(db)
	ctx := context.Background()
	user, wallet := uuid.New(), uuid.New()
	require.NoError(t, db.Exec("INSERT INTO wallets (id,user_id,balance) VALUES (?,?,1000)", wallet, user).Error)
	legacy := insertLegacyTransaction(t, db, user, wallet, 200)
	require.False(t, legacy.BalanceApplied)

	legacy.Amount = 300
	require.NoError(t, repo.Update(ctx, legacy))
	require.Equal(t, 700.0, walletBalance(t, db, wallet), "no +200 reversal, only the new -300")
	var flag bool
	require.NoError(t, db.Raw("SELECT balance_applied FROM transactions WHERE id = ?", legacy.ID).Scan(&flag).Error)
	require.True(t, flag)

	// Now applied: a further edit reverses -300 and applies -100.
	legacy.Amount = 100
	require.NoError(t, repo.Update(ctx, legacy))
	require.Equal(t, 900.0, walletBalance(t, db, wallet))
}

func TestLegacyTransactionDeleteDoesNotReverse(t *testing.T) {
	db := testDatabase(t)
	repo := NewTransactionRepository(db)
	user, wallet := uuid.New(), uuid.New()
	require.NoError(t, db.Exec("INSERT INTO wallets (id,user_id,balance) VALUES (?,?,1000)", wallet, user).Error)
	legacy := insertLegacyTransaction(t, db, user, wallet, 200)

	require.NoError(t, repo.Delete(context.Background(), legacy.ID))
	require.Equal(t, 1000.0, walletBalance(t, db, wallet))
}

func TestNewTransactionEditAndDeleteReverse(t *testing.T) {
	db := testDatabase(t)
	repo := NewTransactionRepository(db)
	ctx := context.Background()
	user, wallet := uuid.New(), uuid.New()
	require.NoError(t, db.Exec("INSERT INTO wallets (id,user_id,balance) VALUES (?,?,1000)", wallet, user).Error)
	tx := &model.Transaction{UserID: user, CategoryID: uuid.New(), WalletID: &wallet, Amount: 200, Type: model.TransactionTypeExpense, TransactionDate: time.Now()}
	require.NoError(t, repo.Create(ctx, tx))
	require.True(t, tx.BalanceApplied)
	require.Equal(t, 800.0, walletBalance(t, db, wallet))

	tx.Amount = 50
	require.NoError(t, repo.Update(ctx, tx))
	require.Equal(t, 950.0, walletBalance(t, db, wallet))

	require.NoError(t, repo.Delete(ctx, tx.ID))
	require.Equal(t, 1000.0, walletBalance(t, db, wallet))
}

func TestTransactionPreloadsSoftDeletedCategoryName(t *testing.T) {
	db := testDatabase(t)
	repo := NewTransactionRepository(db)
	user, cat := uuid.New(), uuid.New()
	require.NoError(t, db.Exec("INSERT INTO categories (id,user_id,name,type,deleted_at) VALUES (?,?,'Old','EXPENSE',now())", cat, user).Error)
	tx := &model.Transaction{UserID: user, CategoryID: cat, Amount: 1, Type: model.TransactionTypeExpense, TransactionDate: time.Now()}
	require.NoError(t, repo.Create(context.Background(), tx))

	got, err := repo.GetByID(context.Background(), tx.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Category)
	require.Equal(t, "Old", got.Category.Name)

	list, _, err := repo.ListByUserID(context.Background(), user, 10, 0, map[string]interface{}{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "Old", list[0].Category.Name)
}
