package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type AlertService interface {
	ListAlerts(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.AlertResponse, int64, error)
	DeleteAlert(ctx context.Context, userID, id uuid.UUID) error
}

type alertService struct {
	alertRepo repository.AlertRepository
}

func NewAlertService(alertRepo repository.AlertRepository) AlertService {
	return &alertService{
		alertRepo: alertRepo,
	}
}

func (s *alertService) ListAlerts(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.AlertResponse, int64, error) {
	offset := (page - 1) * limit
	alerts, total, err := s.alertRepo.ListByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	var res []dto.AlertResponse
	for _, a := range alerts {
		res = append(res, *dto.ToAlertResponse(&a))
	}
	return res, total, nil
}

func (s *alertService) DeleteAlert(ctx context.Context, userID, id uuid.UUID) error {
	alert, err := s.alertRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if alert.UserID != userID {
		return errors.New("alert not found")
	}
	return s.alertRepo.Delete(ctx, id)
}
