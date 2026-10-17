package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

// Single source of truth for how transactions roll up into report totals.
// Reports, monthly targets and the AI chat tools all aggregate through these
// expressions so "income", "expense" and "invested" mean the same thing
// everywhere:
//   - income:  type = INCOME
//   - expense: type = EXPENSE
//   - active investment: type = INVESTMENT that is still HOLDING (or legacy NULL status)
//   - realized PnL: sum over all INVESTMENT rows
//
// Wallet transfers and debts/repayments live in their own tables
// (wallet_transfers, debts, repayments), so they are never counted here.
//
// categories.exclude_from_average_daily only affects the "average daily
// spending" figure (TransactionRepository.GetSummaryByUserID); totals and
// breakdowns intentionally include those categories.
const (
	sqlSumIncome           = "COALESCE(SUM(CASE WHEN transactions.type = 'INCOME' THEN transactions.amount ELSE 0 END), 0)"
	sqlSumExpense          = "COALESCE(SUM(CASE WHEN transactions.type = 'EXPENSE' THEN transactions.amount ELSE 0 END), 0)"
	sqlSumActiveInvestment = "COALESCE(SUM(CASE WHEN transactions.type = 'INVESTMENT' AND (transactions.status IS NULL OR transactions.status = 'HOLDING') THEN transactions.amount ELSE 0 END), 0)"
	sqlSumRealizedPnL      = "COALESCE(SUM(CASE WHEN transactions.type = 'INVESTMENT' THEN COALESCE(transactions.realized_pnl, 0) ELSE 0 END), 0)"
)

// TransactionTotals are the per-type sums for a user and date range.
type TransactionTotals struct {
	Income           float64
	Expense          float64
	ActiveInvestment float64
	RealizedPnL      float64
}

// CategoryTotal is the expense total of one category.
type CategoryTotal struct {
	CategoryID   uuid.UUID
	CategoryName string
	Total        float64
}

// MonthlyTotal is income/expense for one calendar month ("YYYY-MM").
type MonthlyTotal struct {
	Month   string
	Income  float64
	Expense float64
}

// ReportRepository runs report aggregations in SQL (SUM / GROUP BY) instead
// of loading rows into Go. Date strings are "YYYY-MM-DD" and inclusive; empty
// or malformed bounds are ignored (open range), matching ListByUserID.
type ReportRepository interface {
	GetTransactionTotals(ctx context.Context, userID uuid.UUID, startDate, endDate string) (TransactionTotals, error)
	GetExpenseByCategory(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]CategoryTotal, error)
	GetMonthlyIncomeExpense(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]MonthlyTotal, error)
	// ListTargetsInRange returns targets of one type for every month in
	// [from, to] (inclusive, by year/month) in a single query.
	ListTargetsInRange(ctx context.Context, userID uuid.UUID, targetType model.TargetType, from, to time.Time) ([]model.MonthlyTarget, error)
}

type reportRepository struct {
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) ReportRepository {
	return &reportRepository{db: db}
}

// AsReportRepository returns a ReportRepository backed by the same database
// as repo when repo is one of this package's DB-backed repositories (e.g. the
// TargetRepository from NewTargetRepository). It lets services build one
// without changing their constructors.
func AsReportRepository(repo interface{}) (ReportRepository, bool) {
	rr, ok := repo.(ReportRepository)
	return rr, ok && rr != nil
}

// transactionsInRange scopes a query on transactions to one user and an
// inclusive date range. Soft-deleted rows are excluded by GORM via Model.
func transactionsInRange(db *gorm.DB, userID uuid.UUID, startDate, endDate string) *gorm.DB {
	q := db.Model(&model.Transaction{}).Where("transactions.user_id = ?", userID)
	if t, err := time.Parse("2006-01-02", startDate); err == nil {
		q = q.Where("transactions.transaction_date >= ?", t.Format("2006-01-02"))
	}
	if t, err := time.Parse("2006-01-02", endDate); err == nil {
		q = q.Where("transactions.transaction_date <= ?", t.Format("2006-01-02"))
	}
	return q
}

func sumTransactionTotals(ctx context.Context, db *gorm.DB, userID uuid.UUID, startDate, endDate string) (TransactionTotals, error) {
	var row struct {
		Income           float64
		Expense          float64
		ActiveInvestment float64
		RealizedPnL      float64 `gorm:"column:realized_pnl"`
	}
	err := transactionsInRange(db.WithContext(ctx), userID, startDate, endDate).
		Select(sqlSumIncome + " AS income, " +
			sqlSumExpense + " AS expense, " +
			sqlSumActiveInvestment + " AS active_investment, " +
			sqlSumRealizedPnL + " AS realized_pnl").
		Scan(&row).Error
	if err != nil {
		return TransactionTotals{}, err
	}
	return TransactionTotals{
		Income:           row.Income,
		Expense:          row.Expense,
		ActiveInvestment: row.ActiveInvestment,
		RealizedPnL:      row.RealizedPnL,
	}, nil
}

func (r *reportRepository) GetTransactionTotals(ctx context.Context, userID uuid.UUID, startDate, endDate string) (TransactionTotals, error) {
	return sumTransactionTotals(ctx, r.db, userID, startDate, endDate)
}

func (r *reportRepository) GetExpenseByCategory(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]CategoryTotal, error) {
	var rows []struct {
		CategoryID   uuid.UUID
		CategoryName string
		Total        float64
	}
	// LEFT JOIN without a deleted_at filter: a soft-deleted category still
	// labels its historical spending.
	err := transactionsInRange(r.db.WithContext(ctx), userID, startDate, endDate).
		Joins("LEFT JOIN categories ON categories.id = transactions.category_id").
		Where("transactions.type = ?", string(model.TransactionTypeExpense)).
		Select("transactions.category_id AS category_id, COALESCE(MAX(categories.name), '') AS category_name, " + sqlSumExpense + " AS total").
		Group("transactions.category_id").
		Order("total DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]CategoryTotal, 0, len(rows))
	for _, row := range rows {
		out = append(out, CategoryTotal{CategoryID: row.CategoryID, CategoryName: row.CategoryName, Total: row.Total})
	}
	return out, nil
}

func (r *reportRepository) GetMonthlyIncomeExpense(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]MonthlyTotal, error) {
	var rows []struct {
		Month   string
		Income  float64
		Expense float64
	}
	// transaction_date is a DATE, so month grouping is timezone-independent.
	err := transactionsInRange(r.db.WithContext(ctx), userID, startDate, endDate).
		Select("to_char(transactions.transaction_date, 'YYYY-MM') AS month, " +
			sqlSumIncome + " AS income, " + sqlSumExpense + " AS expense").
		Group("to_char(transactions.transaction_date, 'YYYY-MM')").
		Order("month").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]MonthlyTotal, 0, len(rows))
	for _, row := range rows {
		out = append(out, MonthlyTotal{Month: row.Month, Income: row.Income, Expense: row.Expense})
	}
	return out, nil
}

func (r *reportRepository) ListTargetsInRange(ctx context.Context, userID uuid.UUID, targetType model.TargetType, from, to time.Time) ([]model.MonthlyTarget, error) {
	var targets []model.MonthlyTarget
	fromKey := from.Year()*12 + int(from.Month())
	toKey := to.Year()*12 + int(to.Month())
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND target_type = ? AND (year * 12 + month) BETWEEN ? AND ?", userID, targetType, fromKey, toKey).
		Find(&targets).Error
	return targets, err
}
