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

type targetRepository struct {
	db *gorm.DB
}

func NewTargetRepository(db *gorm.DB) TargetRepository {
	return &targetRepository{db: db}
}

func (r *targetRepository) UpsertTarget(ctx context.Context, target *model.MonthlyTarget) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "target_type"}, {Name: "month"}, {Name: "year"}},
		DoUpdates: clause.AssignmentColumns([]string{"target_amount", "updated_at"}),
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

func (r *targetRepository) GetMonthlyTotalByCategoryType(ctx context.Context, userID uuid.UUID, catType model.CategoryType, startDate, endDate time.Time) (float64, error) {
	var total float64
	err := r.db.WithContext(ctx).
		Model(&model.Transaction{}).
		Joins("JOIN categories ON categories.id = transactions.category_id").
		Where("transactions.user_id = ? AND categories.type = ? AND transactions.transaction_date >= ? AND transactions.transaction_date <= ?", userID, catType, startDate, endDate).
		Select("COALESCE(SUM(transactions.amount), 0)").
		Scan(&total).Error
	return total, err
}
