package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"gorm.io/gorm/clause"

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
	return mapBudgetUniqueViolation(r.db.WithContext(ctx).Omit(clause.Associations).Create(budget).Error)
}

// mapBudgetUniqueViolation turns a hit on idx_budgets_user_category_period
// (one live budget per user/category/period) into constant.ErrAlreadyExists.
func mapBudgetUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%w: budget for this category and period", constant.ErrAlreadyExists)
	}
	return err
}

func (r *budgetRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Budget, error) {
	var budget model.Budget
	err := r.db.WithContext(ctx).Preload("Category", withDeleted).First(&budget, "id = ?", id).Error
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

	err := query.Preload("Category", withDeleted).Limit(limit).Offset(offset).Find(&budgets).Error
	return budgets, total, err
}

func (r *budgetRepository) Update(ctx context.Context, budget *model.Budget) error {
	return mapBudgetUniqueViolation(r.db.WithContext(ctx).Omit(clause.Associations).Save(budget).Error)
}

func (r *budgetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Budget{}, "id = ?", id).Error
}

func (r *budgetRepository) GetByCategoryAndPeriod(ctx context.Context, userID, categoryID uuid.UUID, t time.Time) (*model.Budget, error) {
	var budget model.Budget

	err := r.db.WithContext(ctx).
		Where("user_id = ? AND category_id = ?", userID, categoryID).
		Where("period_start <= ?", t).
		Where("(period_type = 'monthly' AND ? < period_start + INTERVAL '1 month') OR (period_type = 'yearly' AND ? < period_start + INTERVAL '1 year')", t, t).
		Order("period_start DESC").
		First(&budget).Error

	if err != nil {
		return nil, err
	}
	return &budget, nil
}
