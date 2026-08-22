package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type PresetService interface {
	CreatePreset(ctx context.Context, userID uuid.UUID, req dto.CreatePresetRequest) (*dto.PresetResponse, error)
	GetPreset(ctx context.Context, userID, id uuid.UUID) (*dto.PresetResponse, error)
	ListPresets(ctx context.Context, userID uuid.UUID) ([]dto.PresetResponse, error)
	UpdatePreset(ctx context.Context, userID, id uuid.UUID, req dto.UpdatePresetRequest) (*dto.PresetResponse, error)
	DeletePreset(ctx context.Context, userID, id uuid.UUID) error
}

type presetService struct {
	presetRepo   repository.PresetRepository
	categoryRepo repository.CategoryRepo
}

func NewPresetService(presetRepo repository.PresetRepository, categoryRepo repository.CategoryRepo) PresetService {
	return &presetService{
		presetRepo:   presetRepo,
		categoryRepo: categoryRepo,
	}
}

func (s *presetService) CreatePreset(ctx context.Context, userID uuid.UUID, req dto.CreatePresetRequest) (*dto.PresetResponse, error) {
	category, err := s.categoryRepo.GetByID(ctx, req.CategoryID)
	if err != nil {
		return nil, errors.New("category not found")
	}
	if category.UserID != userID {
		return nil, errors.New("unauthorized category access")
	}

	preset := &model.Preset{
		UserID:      userID,
		CategoryID:  req.CategoryID,
		Name:        req.Name,
		Amount:      req.Amount,
		Type:        model.TransactionType(req.Type),
		Description: req.Description,
		Icon:        req.Icon,
		SortOrder:   req.SortOrder,
	}

	if err := s.presetRepo.Create(ctx, preset); err != nil {
		return nil, err
	}

	preset.Category = *category
	return dto.ToPresetResponse(preset), nil
}

func (s *presetService) GetPreset(ctx context.Context, userID, id uuid.UUID) (*dto.PresetResponse, error) {
	preset, err := s.presetRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if preset.UserID != userID {
		return nil, errors.New("preset not found")
	}
	return dto.ToPresetResponse(preset), nil
}

func (s *presetService) ListPresets(ctx context.Context, userID uuid.UUID) ([]dto.PresetResponse, error) {
	presets, err := s.presetRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	res := make([]dto.PresetResponse, 0, len(presets))
	for _, p := range presets {
		res = append(res, *dto.ToPresetResponse(&p))
	}
	return res, nil
}

func (s *presetService) UpdatePreset(ctx context.Context, userID, id uuid.UUID, req dto.UpdatePresetRequest) (*dto.PresetResponse, error) {
	preset, err := s.presetRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if preset.UserID != userID {
		return nil, errors.New("preset not found")
	}

	if req.CategoryID != nil {
		category, err := s.categoryRepo.GetByID(ctx, *req.CategoryID)
		if err != nil {
			return nil, errors.New("category not found")
		}
		if category.UserID != userID {
			return nil, errors.New("unauthorized category access")
		}
		preset.CategoryID = *req.CategoryID
		preset.Category = *category
	}

	if req.Name != nil {
		preset.Name = *req.Name
	}
	if req.Amount != nil {
		preset.Amount = *req.Amount
	}
	if req.Type != nil {
		preset.Type = model.TransactionType(*req.Type)
	}
	if req.Description != nil {
		preset.Description = *req.Description
	}
	if req.Icon != nil {
		preset.Icon = *req.Icon
	}
	if req.SortOrder != nil {
		preset.SortOrder = *req.SortOrder
	}

	if err := s.presetRepo.Update(ctx, preset); err != nil {
		return nil, err
	}

	return dto.ToPresetResponse(preset), nil
}

func (s *presetService) DeletePreset(ctx context.Context, userID, id uuid.UUID) error {
	preset, err := s.presetRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if preset.UserID != userID {
		return errors.New("preset not found")
	}
	return s.presetRepo.Delete(ctx, id)
}
