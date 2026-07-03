package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type BudgetRepository interface {
	Create(ctx context.Context, budget *model.Budget) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Budget, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Budget, int64, error)
	Update(ctx context.Context, budget *model.Budget) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByCategoryAndPeriod(ctx context.Context, userID, categoryID uuid.UUID, t time.Time) (*model.Budget, error)
}

type budgetRepository struct {
	db *gorm.DB
}

func NewBudgetRepository(db *gorm.DB) BudgetRepository {
	return &budgetRepository{db: db}
}

func (r *budgetRepository) Create(ctx context.Context, budget *model.Budget) error {
	return r.db.WithContext(ctx).Create(budget).Error
}

func (r *budgetRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Budget, error) {
	var budget model.Budget
	err := r.db.WithContext(ctx).Preload("Category").First(&budget, "id = ?", id).Error
	if err != nil {
		return nil, err
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

	err := query.Preload("Category").Limit(limit).Offset(offset).Find(&budgets).Error
	return budgets, total, err
}

func (r *budgetRepository) Update(ctx context.Context, budget *model.Budget) error {
	return r.db.WithContext(ctx).Save(budget).Error
}

func (r *budgetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Budget{}, "id = ?", id).Error
}

func (r *budgetRepository) GetByCategoryAndPeriod(ctx context.Context, userID, categoryID uuid.UUID, t time.Time) (*model.Budget, error) {
	var budget model.Budget

	// Formulate period start (beginning of month/year) based on budget logic
	// In model, PeriodType is 'monthly' or 'yearly'.
	// We check if budget covers the date `t`.
	// For monthly, PeriodStart is start of the month, and we check if it is within that month.
	// Since period_start is a date, we compare:
	// monthly: period_start <= t AND period_start >= start_of_month(t)
	// Let's do simple: find budget where user_id, category_id match, and t is within the period.
	// For simplicity, we match: period_start <= t AND (period_start + 1 month > t for monthly)
	// We query GORM:
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND category_id = ?", userID, categoryID).
		Where("period_start <= ?", t).
		Order("period_start DESC").
		First(&budget).Error

	if err != nil {
		return nil, err
	}
	return &budget, nil
}
