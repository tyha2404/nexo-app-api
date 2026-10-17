package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TargetRepository interface {
	UpsertTarget(ctx context.Context, target *model.MonthlyTarget) error
	GetTarget(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error)
	GetMonthlyTotalByCategoryType(ctx context.Context, userID uuid.UUID, catType model.CategoryType, startDate, endDate time.Time) (float64, error)
}

// targetRepository also implements ReportRepository (via the embedded
// reportRepository on the same DB), so services that only receive a
// TargetRepository can still aggregate in SQL; see AsReportRepository.
type targetRepository struct {
	db *gorm.DB
	*reportRepository
}

func NewTargetRepository(db *gorm.DB) TargetRepository {
	return &targetRepository{db: db, reportRepository: &reportRepository{db: db}}
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

// GetMonthlyTotalByCategoryType sums transactions of one type in
// [startDate, endDate] (calendar dates) using the shared report definitions
// (investment = still HOLDING).
func (r *targetRepository) GetMonthlyTotalByCategoryType(ctx context.Context, userID uuid.UUID, catType model.CategoryType, startDate, endDate time.Time) (float64, error) {
	totals, err := sumTransactionTotals(ctx, r.db, userID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
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
