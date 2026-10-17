package repository

import (
	"context"

	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

// ReminderRepository reads, across all users, the items the daily push
// reminder job notifies about. Dates are "YYYY-MM-DD" in the app timezone
// (the Postgres session TimeZone is the app timezone, see db.buildDSN).
type ReminderRepository interface {
	// ListDebtsDueBetween returns unfinished debts whose due date is in [from, to].
	ListDebtsDueBetween(ctx context.Context, from, to string) ([]model.Debt, error)
	// ListStatementsDueBetween returns unpaid credit-card statements (with
	// their wallet) whose due date is in [from, to].
	ListStatementsDueBetween(ctx context.Context, from, to string) ([]model.CreditCardStatement, error)
}

type reminderRepository struct {
	db *gorm.DB
}

func NewReminderRepository(db *gorm.DB) ReminderRepository {
	return &reminderRepository{db: db}
}

func (r *reminderRepository) ListDebtsDueBetween(ctx context.Context, from, to string) ([]model.Debt, error) {
	var debts []model.Debt
	err := r.db.WithContext(ctx).
		Where("due_date IS NOT NULL AND due_date::date BETWEEN ?::date AND ?::date", from, to).
		Where("status <> ?", model.DebtStatusCompleted).
		Where("total_amount > paid_amount").
		Order("due_date ASC").
		Find(&debts).Error
	return debts, err
}

func (r *reminderRepository) ListStatementsDueBetween(ctx context.Context, from, to string) ([]model.CreditCardStatement, error) {
	var stmts []model.CreditCardStatement
	err := r.db.WithContext(ctx).
		Preload("Wallet").
		Where("due_date BETWEEN ?::date AND ?::date", from, to).
		Where("status <> ?", model.StatementStatusPaid).
		Where("statement_balance > paid_amount").
		Order("due_date ASC").
		Find(&stmts).Error
	return stmts, err
}
