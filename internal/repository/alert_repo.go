package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type AlertRepository interface {
	Create(ctx context.Context, alert *model.Alert) error
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

func (r *alertRepository) Create(ctx context.Context, alert *model.Alert) error {
	return r.db.WithContext(ctx).Create(alert).Error
}

func (r *alertRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Alert, error) {
	var alert model.Alert
	err := r.db.WithContext(ctx).Preload("Budget").First(&alert, "id = ?", id).Error
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

	err := query.Preload("Budget").Order("triggered_at DESC").Limit(limit).Offset(offset).Find(&alerts).Error
	return alerts, total, err
}

func (r *alertRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Alert{}, "id = ?", id).Error
}
