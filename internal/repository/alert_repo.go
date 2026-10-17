package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AlertRepository interface {
	// Create inserts the alert unless one of the same type already exists for
	// the budget; created reports whether a new row was inserted.
	Create(ctx context.Context, alert *model.Alert) (created bool, err error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Alert, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Alert, int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type alertRepository struct {
	db *gorm.DB
}

func NewAlertRepository(db *gorm.DB) AlertRepository {
	return &alertRepository{db: db}
}

func (r *alertRepository) Create(ctx context.Context, alert *model.Alert) (bool, error) {
	// A budget identifies one period. Lock it so concurrent expenses cannot
	// create duplicate notifications for the same threshold, even across servers.
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var budget model.Budget
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", alert.BudgetID, alert.UserID).First(&budget).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Unscoped().Model(&model.Alert{}).Where("budget_id = ? AND user_id = ? AND alert_type = ?", alert.BudgetID, alert.UserID, alert.AlertType).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		if err := tx.Create(alert).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return created, nil
}

func (r *alertRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Alert, error) {
	var alert model.Alert
	err := r.db.WithContext(ctx).Preload("Budget", withDeleted).First(&alert, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

func (r *alertRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Alert, int64, error) {
	var alerts []model.Alert
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Alert{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("Budget", withDeleted).Order("triggered_at DESC").Limit(limit).Offset(offset).Find(&alerts).Error
	return alerts, total, err
}

func (r *alertRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Alert{}, "id = ?", id).Error
}
