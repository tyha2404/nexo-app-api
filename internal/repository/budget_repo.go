package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/constant"
	"gorm.io/gorm/clause"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type BudgetRepository interface {
	Create(ctx context.Context, budget *model.Budget) error
	// GetByIDForUser returns the user's budget; "budget not found"
	// (constant.ErrNotFound) when it is missing or belongs to someone else.
	GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Budget, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Budget, int64, error)
	Update(ctx context.Context, budget *model.Budget) error
	// Delete soft-deletes the user's budget; constant.ErrNotFound when no
	// live budget with that id belongs to the user.
	Delete(ctx context.Context, userID, id uuid.UUID) error
	GetByCategoryAndPeriod(ctx context.Context, userID, categoryID uuid.UUID, t time.Time) (*model.Budget, error)
	// ListActiveByUserID returns the user's budgets whose period covers the
	// calendar day of at (same predicate as GetByCategoryAndPeriod), newest
	// period_start first. The monthly rollover keeps one row per category per
	// month, so this is what "the budgets of a month" means.
	ListActiveByUserID(ctx context.Context, userID uuid.UUID, at time.Time) ([]model.Budget, error)
}

type budgetRepository struct {
	db *gorm.DB
}

func NewBudgetRepository(db *gorm.DB) BudgetRepository {
	return &budgetRepository{db: db}
}

func (r *budgetRepository) Create(ctx context.Context, budget *model.Budget) error {
	return mapBudgetUniqueViolation(r.db.WithContext(ctx).Omit(clause.Associations).Create(budget).Error)
}

// mapBudgetUniqueViolation turns a hit on idx_budgets_user_category_period
// (one live budget per user/category/period) into constant.ErrAlreadyExists.
func mapBudgetUniqueViolation(err error) error {
	if isUniqueViolation(err) {
		return fmt.Errorf("%w: budget for this category and period", constant.ErrAlreadyExists)
	}
	return err
}

func (r *budgetRepository) GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Budget, error) {
	var budget model.Budget
	err := r.db.WithContext(ctx).Preload("Category", withDeleted).
		Where("id = ? AND user_id = ?", id, userID).First(&budget).Error
	if err != nil {
		return nil, notFoundAs("budget", err)
	}
	return &budget, nil
}

func (r *budgetRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Budget, int64, error) {
	var budgets []model.Budget
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Budget{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("Category", withDeleted).Limit(limit).Offset(offset).Find(&budgets).Error
	return budgets, total, err
}

// budgetEditableColumns are written by Update (an explicit Select also writes
// zero values).
var budgetEditableColumns = []string{"category_id", "amount", "period_type", "period_start", "updated_at"}

// Update writes the budget's editable columns, scoped by id AND user_id. Unlike
// Save, it never inserts: a budget deleted meanwhile is not resurrected and
// constant.ErrNotFound is returned instead.
func (r *budgetRepository) Update(ctx context.Context, budget *model.Budget) error {
	result := r.db.WithContext(ctx).Model(budget).Omit(clause.Associations).
		Where("id = ? AND user_id = ?", budget.ID, budget.UserID).
		Select(budgetEditableColumns).
		Updates(budget)
	if result.Error != nil {
		return mapBudgetUniqueViolation(result.Error)
	}
	if result.RowsAffected == 0 {
		return constant.ErrNotFound
	}
	return nil
}

func (r *budgetRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return deleteForUser(r.db.WithContext(ctx), &model.Budget{}, "budget", userID, id)
}

func (r *budgetRepository) GetByCategoryAndPeriod(ctx context.Context, userID, categoryID uuid.UUID, t time.Time) (*model.Budget, error) {
	var budget model.Budget

	err := r.db.WithContext(ctx).
		Where("user_id = ? AND category_id = ?", userID, categoryID).
		Where("period_start <= ?", t).
		Where("(period_type = ? AND ? < period_start + INTERVAL '1 month') OR (period_type = ? AND ? < period_start + INTERVAL '1 year')", model.BudgetPeriodMonthly, t, model.BudgetPeriodYearly, t).
		Order("period_start DESC").
		First(&budget).Error

	if err != nil {
		return nil, notFoundAs("budget", err)
	}
	return &budget, nil
}

func (r *budgetRepository) ListActiveByUserID(ctx context.Context, userID uuid.UUID, at time.Time) ([]model.Budget, error) {
	// Compare against a DATE literal (the day of at in its own location) so the
	// session timezone cannot shift the boundary (period_start is a DATE).
	day := at.Format(time.DateOnly)
	var budgets []model.Budget
	err := r.db.WithContext(ctx).
		Preload("Category", withDeleted).
		Where("user_id = ?", userID).
		Where("period_start <= CAST(? AS date)", day).
		Where("((period_type = ? AND CAST(? AS date) < period_start + INTERVAL '1 month') OR (period_type = ? AND CAST(? AS date) < period_start + INTERVAL '1 year'))",
			model.BudgetPeriodMonthly, day, model.BudgetPeriodYearly, day).
		Order("period_start DESC, created_at DESC").
		Find(&budgets).Error
	return budgets, err
}
