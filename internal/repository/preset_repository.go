package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PresetRepository interface {
	Create(ctx context.Context, preset *model.Preset) error
	// GetByIDForUser returns the user's preset; "preset not found"
	// (constant.ErrNotFound) when it is missing or belongs to someone else.
	GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Preset, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Preset, error)
	Update(ctx context.Context, preset *model.Preset) error
	// Delete soft-deletes the user's preset; constant.ErrNotFound when no
	// live preset with that id belongs to the user.
	Delete(ctx context.Context, userID, id uuid.UUID) error
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

func (r *presetRepository) GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Preset, error) {
	var preset model.Preset
	err := r.db.WithContext(ctx).Preload("Category", withDeleted).
		Where("id = ? AND user_id = ?", id, userID).First(&preset).Error
	if err != nil {
		return nil, notFoundAs("preset", err)
	}
	return &preset, nil
}

func (r *presetRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Preset, error) {
	var presets []model.Preset
	err := r.db.WithContext(ctx).
		Preload("Category", withDeleted).
		Where("user_id = ?", userID).
		Order("sort_order ASC, created_at DESC").
		Find(&presets).Error
	return presets, err
}

// presetEditableColumns are written by Update (an explicit Select also writes
// zero values such as sort_order = 0).
var presetEditableColumns = []string{"category_id", "name", "amount", "type", "description", "icon", "sort_order", "updated_at"}

// Update writes the preset's editable columns, scoped by id AND user_id. Unlike
// Save, it never inserts: a preset deleted meanwhile is not resurrected and
// constant.ErrNotFound is returned instead.
func (r *presetRepository) Update(ctx context.Context, preset *model.Preset) error {
	result := r.db.WithContext(ctx).Model(preset).Omit(clause.Associations).
		Where("id = ? AND user_id = ?", preset.ID, preset.UserID).
		Select(presetEditableColumns).
		Updates(preset)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return constant.ErrNotFound
	}
	return nil
}

func (r *presetRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return deleteForUser(r.db.WithContext(ctx), &model.Preset{}, "preset", userID, id)
}
