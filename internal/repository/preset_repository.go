package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type PresetRepository interface {
	Create(ctx context.Context, preset *model.Preset) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Preset, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Preset, error)
	Update(ctx context.Context, preset *model.Preset) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type presetRepository struct {
	db *gorm.DB
}

func NewPresetRepository(db *gorm.DB) PresetRepository {
	return &presetRepository{db: db}
}

func (r *presetRepository) Create(ctx context.Context, preset *model.Preset) error {
	return r.db.WithContext(ctx).Create(preset).Error
}

func (r *presetRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Preset, error) {
	var preset model.Preset
	err := r.db.WithContext(ctx).Preload("Category").First(&preset, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &preset, nil
}

func (r *presetRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Preset, error) {
	var presets []model.Preset
	err := r.db.WithContext(ctx).
		Preload("Category").
		Where("user_id = ?", userID).
		Order("sort_order ASC, created_at DESC").
		Find(&presets).Error
	return presets, err
}

func (r *presetRepository) Update(ctx context.Context, preset *model.Preset) error {
	return r.db.WithContext(ctx).Save(preset).Error
}

func (r *presetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Preset{}, "id = ?", id).Error
}
