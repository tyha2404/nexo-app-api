package repository

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Integration tests only use an explicitly supplied disposable database. Each
// test gets an isolated schema; no application migrations or .env are loaded.
func testDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("NEXO_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set NEXO_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		sqlAdmin, _ := admin.DB()
		sqlAdmin.Close()
	})
	ddl := []string{
		`CREATE TABLE debts (id uuid PRIMARY KEY,user_id uuid,type varchar(20),title text,total_amount numeric(14,2),paid_amount numeric(14,2),start_date timestamp,due_date timestamp,status text,notes text,wallet_id uuid,created_at timestamp DEFAULT now(),updated_at timestamp DEFAULT now(),deleted_at timestamp)`,
		`CREATE TABLE repayments (id uuid PRIMARY KEY,debt_id uuid,amount numeric(14,2),paid_at timestamp,notes text,wallet_id uuid,created_at timestamp DEFAULT now(),updated_at timestamp DEFAULT now(),deleted_at timestamp)`,
		`CREATE TABLE wallets (id uuid PRIMARY KEY,user_id uuid,name text,type text,balance numeric(15,2),currency text DEFAULT 'VND',allocation_percent numeric(5,2) DEFAULT 0,statement_balance numeric(15,2) DEFAULT 0,minimum_payment numeric(15,2) DEFAULT 0,previous_balance numeric(15,2) DEFAULT 0,updated_at timestamp,deleted_at timestamp)`,
		`CREATE TABLE credit_card_statements (id uuid PRIMARY KEY,user_id uuid,wallet_id uuid,statement_month int,statement_year int,statement_date date,due_date date,statement_balance numeric(15,2),minimum_payment numeric(15,2) DEFAULT 0,previous_balance numeric(15,2) DEFAULT 0,paid_amount numeric(15,2) DEFAULT 0,status text,note text,created_at timestamp DEFAULT now(),updated_at timestamp DEFAULT now(),deleted_at timestamp)`,
		`CREATE TABLE categories (id uuid PRIMARY KEY,user_id uuid,name text,type text,description text,exclude_from_average_daily boolean DEFAULT false,created_at timestamp DEFAULT now(),updated_at timestamp DEFAULT now(),deleted_at timestamp)`,
		`CREATE TABLE transactions (id uuid PRIMARY KEY,user_id uuid,category_id uuid,wallet_id uuid,amount numeric(15,2),type text,status text DEFAULT 'HOLDING',realized_pnl numeric(15,2) DEFAULT 0,description text,receipt_url text,balance_applied boolean NOT NULL DEFAULT false,transaction_date date,created_at timestamp DEFAULT now(),updated_at timestamp DEFAULT now(),deleted_at timestamp)`,
		`CREATE TABLE budgets (id uuid PRIMARY KEY,user_id uuid,category_id uuid,amount numeric(15,2),period_type text,period_start date,created_at timestamp DEFAULT now(),updated_at timestamp DEFAULT now(),deleted_at timestamp)`,
		`CREATE UNIQUE INDEX idx_budgets_user_category_period ON budgets (user_id, category_id, period_type, period_start) WHERE deleted_at IS NULL`,
		`CREATE TABLE alerts (id uuid PRIMARY KEY,user_id uuid,budget_id uuid,alert_type text,triggered_at timestamp DEFAULT now(),message text,created_at timestamp DEFAULT now(),updated_at timestamp DEFAULT now(),deleted_at timestamp)`,
	}
	for _, q := range ddl {
		require.NoError(t, db.Exec(q).Error)
	}
	return db
}

func TestRepaymentsConcurrentAndOverpayment(t *testing.T) {
	db := testDatabase(t)
	repo := NewDebtRepository(db)
	ctx := context.Background()
	user, debtID, walletID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, db.Exec("INSERT INTO wallets (id,user_id,balance) VALUES (?,?,100)", walletID, user).Error)
	original := model.Debt{ID: debtID, UserID: user, Type: model.DebtTypePayable, TotalAmount: 100, PaidAmount: 0, Status: model.DebtStatusPending}
	require.NoError(t, db.Create(&original).Error)
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snapshot := original
			errs <- repo.AddRepaymentWithWallet(ctx, &snapshot, &model.Repayment{ID: uuid.New(), DebtID: debtID, Amount: 10, PaidAt: time.Now()}, &walletID, -10)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var saved model.Debt
	require.NoError(t, db.First(&saved, "id = ?", debtID).Error)
	require.Equal(t, 100.0, saved.PaidAmount)
	require.Equal(t, model.DebtStatusCompleted, saved.Status)
	var balance float64
	require.NoError(t, db.Raw("SELECT balance FROM wallets WHERE id = ?", walletID).Scan(&balance).Error)
	require.Zero(t, balance)
	err := repo.AddRepaymentWithWallet(ctx, &original, &model.Repayment{ID: uuid.New(), DebtID: debtID, Amount: 1}, &walletID, -1)
	require.ErrorIs(t, err, ErrInvalidRepayment)
	var count int64
	db.Model(&model.Repayment{}).Count(&count)
	require.EqualValues(t, 10, count)
}

func TestRepaymentRollsBackWhenWalletUnavailable(t *testing.T) {
	db := testDatabase(t)
	repo := NewDebtRepository(db)
	debt := model.Debt{ID: uuid.New(), UserID: uuid.New(), Type: model.DebtTypePayable, TotalAmount: 100, PaidAmount: 0, Status: model.DebtStatusPending}
	require.NoError(t, db.Create(&debt).Error)
	walletID := uuid.New()
	err := repo.AddRepaymentWithWallet(context.Background(), &debt, &model.Repayment{ID: uuid.New(), DebtID: debt.ID, Amount: 50}, &walletID, -50)
	require.Error(t, err)
	var saved model.Debt
	db.First(&saved, "id = ?", debt.ID)
	require.Zero(t, saved.PaidAmount)
	var count int64
	db.Model(&model.Repayment{}).Count(&count)
	require.Zero(t, count)
}

func TestTransactionSearchPaginationSummaryAndTenant(t *testing.T) {
	db := testDatabase(t)
	repo := NewTransactionRepository(db)
	user, other, category := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, db.Exec("INSERT INTO categories (id,user_id,name,type) VALUES (?,?,'Food','EXPENSE')", category, user).Error)
	for i := 0; i < 26; i++ {
		description := fmt.Sprintf("Coffee %02d", i)
		if i == 25 {
			description = "Special 100%_bill"
		}
		tx := model.Transaction{UserID: user, CategoryID: category, Type: model.TransactionTypeExpense, Amount: 10, Description: &description, TransactionDate: time.Now()}
		require.NoError(t, repo.Create(context.Background(), &tx))
	}
	hidden := "Coffee another user"
	require.NoError(t, repo.Create(context.Background(), &model.Transaction{UserID: other, CategoryID: category, Type: model.TransactionTypeExpense, Amount: 999, Description: &hidden, TransactionDate: time.Now()}))
	filters := map[string]interface{}{"keyword": "coffee", "type": "EXPENSE"}
	rows, total, err := repo.ListByUserID(context.Background(), user, 10, 20, filters)
	require.NoError(t, err)
	require.Len(t, rows, 5)
	require.EqualValues(t, 25, total)
	sum, _, count, _, _, _, err := repo.GetSummaryByUserID(context.Background(), user, filters)
	require.NoError(t, err)
	require.Equal(t, 250.0, sum)
	require.EqualValues(t, 25, count)
	rows, total, err = repo.ListByUserID(context.Background(), user, 10, 0, map[string]interface{}{"keyword": "%_"})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Contains(t, *rows[0].Description, "100%_")
}

func TestBudgetAlertsDeduplicateConcurrentThresholds(t *testing.T) {
	db := testDatabase(t)
	repo := NewAlertRepository(db)
	user, budget := uuid.New(), uuid.New()
	require.NoError(t, db.Exec("INSERT INTO budgets (id,user_id,amount,period_type,period_start) VALUES (?,?,100,'monthly','2026-10-01')", budget, user).Error)
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Create(context.Background(), &model.Alert{UserID: user, BudgetID: budget, AlertType: "over_limit", Message: "limit"})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var count int64
	db.Model(&model.Alert{}).Count(&count)
	require.EqualValues(t, 1, count)
	created, err := repo.Create(context.Background(), &model.Alert{UserID: user, BudgetID: budget, AlertType: "approaching_limit", Message: "80%"})
	require.NoError(t, err)
	require.True(t, created)
	db.Model(&model.Alert{}).Count(&count)
	require.EqualValues(t, 2, count)
}

func TestExpiredBudgetDoesNotApplyToNewMonth(t *testing.T) {
	db := testDatabase(t)
	repo := NewBudgetRepository(db)
	user, category := uuid.New(), uuid.New()
	require.NoError(t, db.Exec("INSERT INTO budgets (id,user_id,category_id,amount,period_type,period_start) VALUES (?,?,?,100,'monthly','2026-09-01')", uuid.New(), user, category).Error)
	_, err := repo.GetByCategoryAndPeriod(context.Background(), user, category, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
