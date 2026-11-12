package repository

import (
	"context"
	"errors"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TargetRepository interface {
	UpsertTarget(ctx context.Context, target *model.MonthlyTarget) error
	GetTarget(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error)
	GetMonthlyTotalByCategoryType(ctx context.Context, userID uuid.UUID, catType model.CategoryType, startDate, endDate time.Time) (float64, error)
}

type targetRepository struct {
	db *gorm.DB
}

func NewTargetRepository(db *gorm.DB) TargetRepository {
	return &targetRepository{db: db}
}

func (r *targetRepository) UpsertTarget(ctx context.Context, target *model.MonthlyTarget) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "target_type"}, {Name: "month"}, {Name: "year"}},
		// The unique index is partial (WHERE deleted_at IS NULL) so a soft-deleted
		// target never blocks a new one; the predicate is needed for index inference.
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "deleted_at IS NULL"}}},
		DoUpdates:   clause.AssignmentColumns([]string{"target_amount", "updated_at"}),
	}).Create(target).Error
}

func (r *targetRepository) GetTarget(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error) {
	return findMonthlyTarget(ctx, r.db, userID, targetType, month, year)
}

// ErrTargetNotFound: the user has no target of that type for the month.
var ErrTargetNotFound = constant.NewKindError(constant.ErrNotFound, "target not found")

// findMonthlyTarget returns the user's target of one type for month/year, or
// ErrTargetNotFound when none is set.
func findMonthlyTarget(ctx context.Context, db *gorm.DB, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error) {
	var target model.MonthlyTarget
	err := db.WithContext(ctx).
		Where("user_id = ? AND target_type = ? AND month = ? AND year = ?", userID, targetType, month, year).
		First(&target).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTargetNotFound
		}
		return nil, err
	}
	return &target, nil
}

// GetMonthlyTotalByCategoryType sums transactions of one type in
// [startDate, endDate] (calendar dates) using the shared report definitions
// (investment = still HOLDING).
func (r *targetRepository) GetMonthlyTotalByCategoryType(ctx context.Context, userID uuid.UUID, catType model.CategoryType, startDate, endDate time.Time) (float64, error) {
	totals, err := sumTransactionTotals(ctx, r.db, userID, util.DateString(startDate), util.DateString(endDate))
	if err != nil {
		return 0, err
	}
	switch catType {
	case model.CategoryTypeIncome:
		return totals.Income, nil
	case model.CategoryTypeExpense:
		return totals.Expense, nil
	case model.CategoryTypeInvestment:
		return totals.ActiveInvestment, nil
	default:
		return 0, nil
	}
}
