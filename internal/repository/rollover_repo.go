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
	// PostgreSQL DISTINCT ON keeps exactly one row per category: the first one in
	// ORDER BY, i.e. the latest period_start. (A plain DISTINCT over every column
	// incl. id returned every historical budget.) GORM still appends the
	// soft-delete filter (deleted_at IS NULL) to this query.
	err := r.db.WithContext(ctx).
		Model(&model.Budget{}).
		Select("DISTINCT ON (category_id) *").
		Where("user_id = ? AND period_type = ? AND period_start < ?", userID, "monthly", periodStart).
		Order("category_id, period_start DESC, created_at DESC").
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
	// Idempotent: a concurrent or repeated rollover never duplicates a budget.
	// Relies on the partial unique index idx_budgets_user_category_period
	// (user_id, category_id, period_type, period_start) WHERE deleted_at IS NULL.
	return r.db.WithContext(ctx).
		Clauses(budgetPeriodOnConflictDoNothing()).
		Create(&budgets).Error
}

func budgetPeriodOnConflictDoNothing() clause.OnConflict {
	return clause.OnConflict{
		Columns:     []clause.Column{{Name: "user_id"}, {Name: "category_id"}, {Name: "period_type"}, {Name: "period_start"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "deleted_at IS NULL"}}},
		DoNothing:   true,
	}
}
