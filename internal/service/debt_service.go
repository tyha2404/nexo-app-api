package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

var (
	ErrDebtNotFound = errors.New("debt not found")
)

type DebtService interface {
	CreateDebt(ctx context.Context, userID uuid.UUID, req dto.CreateDebtRequest) (*dto.DebtResponse, error)
	GetDebts(ctx context.Context, userID uuid.UUID, debtType model.DebtType, status model.DebtStatus) ([]dto.DebtResponse, error)
	GetDebtSummary(ctx context.Context, userID uuid.UUID) (*dto.DebtSummaryResponse, error)
	AddRepayment(ctx context.Context, userID uuid.UUID, debtID uuid.UUID, req dto.AddRepaymentRequest) (*dto.DebtResponse, error)
	DeleteDebt(ctx context.Context, userID uuid.UUID, debtID uuid.UUID) error
}

type debtService struct {
	debtRepo repository.DebtRepository
}

func NewDebtService(debtRepo repository.DebtRepository) DebtService {
	return &debtService{debtRepo: debtRepo}
}

func (s *debtService) CreateDebt(ctx context.Context, userID uuid.UUID, req dto.CreateDebtRequest) (*dto.DebtResponse, error) {
	status := model.DebtStatusPending
	now := time.Now()
	if req.DueDate != nil && req.DueDate.Before(now) {
		status = model.DebtStatusOverdue
	}

	startDate := req.StartDate
	if startDate == nil {
		startDate = &now
	}

	debt := &model.Debt{
		ID:          uuid.New(),
		UserID:      userID,
		Type:        req.Type,
		Title:       req.Title,
		TotalAmount: req.TotalAmount,
		PaidAmount:  0,
		StartDate:   startDate,
		DueDate:     req.DueDate,
		Status:      status,
		Notes:       req.Notes,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.debtRepo.Create(ctx, debt); err != nil {
		return nil, err
	}

	return s.toResponse(debt), nil
}

func (s *debtService) GetDebts(ctx context.Context, userID uuid.UUID, debtType model.DebtType, status model.DebtStatus) ([]dto.DebtResponse, error) {
	debts, err := s.debtRepo.FindByUserID(ctx, userID, debtType, status)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	res := make([]dto.DebtResponse, len(debts))
	for i, debt := range debts {
		// Auto update status to overdue if applicable
		if debt.Status == model.DebtStatusPending && debt.DueDate != nil && debt.DueDate.Before(now) {
			debt.Status = model.DebtStatusOverdue
		}
		res[i] = *s.toResponse(&debt)
	}

	return res, nil
}

func (s *debtService) GetDebtSummary(ctx context.Context, userID uuid.UUID) (*dto.DebtSummaryResponse, error) {
	return s.debtRepo.GetSummaryByUserID(ctx, userID)
}

func (s *debtService) AddRepayment(ctx context.Context, userID uuid.UUID, debtID uuid.UUID, req dto.AddRepaymentRequest) (*dto.DebtResponse, error) {
	debt, err := s.debtRepo.FindByID(ctx, debtID, userID)
	if err != nil {
		return nil, err
	}
	if debt == nil {
		return nil, ErrDebtNotFound
	}

	paidAt := time.Now()
	if req.PaidAt != nil {
		paidAt = *req.PaidAt
	}

	repayment := &model.Repayment{
		ID:        uuid.New(),
		DebtID:    debtID,
		Amount:    req.Amount,
		PaidAt:    paidAt,
		Notes:     req.Notes,
		CreatedAt: time.Now(),
	}

	debt.PaidAmount += req.Amount
	debt.UpdatedAt = time.Now()

	if debt.PaidAmount >= debt.TotalAmount {
		debt.Status = model.DebtStatusCompleted
	} else if debt.DueDate != nil && debt.DueDate.Before(time.Now()) {
		debt.Status = model.DebtStatusOverdue
	} else {
		debt.Status = model.DebtStatusPending
	}

	if err := s.debtRepo.AddRepayment(ctx, debt, repayment); err != nil {
		return nil, err
	}

	debt.Repayments = append(debt.Repayments, *repayment)
	return s.toResponse(debt), nil
}

func (s *debtService) DeleteDebt(ctx context.Context, userID uuid.UUID, debtID uuid.UUID) error {
	return s.debtRepo.Delete(ctx, debtID, userID)
}

func (s *debtService) toResponse(debt *model.Debt) *dto.DebtResponse {
	remaining := debt.TotalAmount - debt.PaidAmount
	if remaining < 0 {
		remaining = 0
	}

	return &dto.DebtResponse{
		ID:          debt.ID,
		UserID:      debt.UserID,
		Type:        debt.Type,
		Title:       debt.Title,
		TotalAmount: debt.TotalAmount,
		PaidAmount:  debt.PaidAmount,
		Remaining:   remaining,
		StartDate:   debt.StartDate,
		DueDate:     debt.DueDate,
		Status:      debt.Status,
		Notes:       debt.Notes,
		Repayments:  debt.Repayments,
		CreatedAt:   debt.CreatedAt,
		UpdatedAt:   debt.UpdatedAt,
	}
}
