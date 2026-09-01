package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RolloverRepository interface {
	GetAllActiveUserIDs(ctx context.Context) ([]uuid.UUID, error)
	GetTargetForMonth(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error)
	GetLatestTargetBefore(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error)
	GetMonthlyBudgetsForPeriod(ctx context.Context, userID uuid.UUID, periodStart time.Time) ([]model.Budget, error)
	GetLatestMonthlyBudgetsBefore(ctx context.Context, userID uuid.UUID, periodStart time.Time) ([]model.Budget, error)
	CreateMonthlyTargets(ctx context.Context, targets []model.MonthlyTarget) error
	CreateBudgets(ctx context.Context, budgets []model.Budget) error
}

type rolloverRepository struct {
	db *gorm.DB
}

func NewRolloverRepository(db *gorm.DB) RolloverRepository {
	return &rolloverRepository{db: db}
}

func (r *rolloverRepository) GetAllActiveUserIDs(ctx context.Context) ([]uuid.UUID, error) {
	var userIDs []uuid.UUID
	err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("deleted_at IS NULL").
		Pluck("id", &userIDs).Error
	if err != nil {
		return nil, err
	}
	return userIDs, nil
}

func (r *rolloverRepository) GetTargetForMonth(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error) {
	var target model.MonthlyTarget
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND target_type = ? AND month = ? AND year = ?", userID, targetType, month, year).
		First(&target).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &target, nil
}

func (r *rolloverRepository) GetLatestTargetBefore(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error) {
	var target model.MonthlyTarget
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND target_type = ? AND (year < ? OR (year = ? AND month < ?))", userID, targetType, year, year, month).
		Order("year DESC, month DESC").
		First(&target).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &target, nil
}

func (r *rolloverRepository) GetMonthlyBudgetsForPeriod(ctx context.Context, userID uuid.UUID, periodStart time.Time) ([]model.Budget, error) {
	var budgets []model.Budget
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND period_type = ? AND period_start = ?", userID, "monthly", periodStart).
		Find(&budgets).Error
	if err != nil {
		return nil, err
	}
	return budgets, nil
}

func (r *rolloverRepository) GetLatestMonthlyBudgetsBefore(ctx context.Context, userID uuid.UUID, periodStart time.Time) ([]model.Budget, error) {
	var budgets []model.Budget
	// Using PostgreSQL DISTINCT ON to fetch the latest budget per category before the target date
	err := r.db.WithContext(ctx).
		Model(&model.Budget{}).
		Where("user_id = ? AND period_type = ? AND period_start < ?", userID, "monthly", periodStart).
		Order("category_id, period_start DESC").
		Distinct("category_id, amount, id, user_id, period_type, period_start, created_at, updated_at, deleted_at").
		Find(&budgets).Error
	if err != nil {
		return nil, err
	}
	return budgets, nil
}

func (r *rolloverRepository) CreateMonthlyTargets(ctx context.Context, targets []model.MonthlyTarget) error {
	if len(targets) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&targets).Error
}

func (r *rolloverRepository) CreateBudgets(ctx context.Context, budgets []model.Budget) error {
	if len(budgets) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&budgets).Error
}
