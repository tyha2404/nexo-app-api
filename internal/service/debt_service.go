package service

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

var (
	ErrDebtNotFound     = errors.New("debt not found")
	ErrInvalidRepayment = repository.ErrInvalidRepayment
	// ErrDebtAlreadyCompleted wraps ErrInvalidRepayment so callers treat it as a 400.
	ErrDebtAlreadyCompleted = fmt.Errorf("khoản nợ đã hoàn tất, không thể trả thêm: %w", repository.ErrInvalidRepayment)
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
	now := util.Now()
	if req.DueDate != nil && util.IsPastDate(*req.DueDate) {
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
		WalletID:    req.WalletID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	var balanceDelta float64
	if req.WalletID != nil {
		balanceDelta = model.DebtCreationWalletDelta(req.Type, req.TotalAmount)
	}

	if err := s.debtRepo.CreateWithWallet(ctx, debt, req.WalletID, balanceDelta); err != nil {
		return nil, err
	}

	return s.toResponse(debt), nil
}

func (s *debtService) GetDebts(ctx context.Context, userID uuid.UUID, debtType model.DebtType, status model.DebtStatus) ([]dto.DebtResponse, error) {
	debts, err := s.debtRepo.FindByUserID(ctx, userID, debtType, status)
	if err != nil {
		return nil, err
	}

	res := make([]dto.DebtResponse, len(debts))
	for i, debt := range debts {
		// Auto update status to overdue if applicable
		if debt.Status == model.DebtStatusPending && debt.DueDate != nil && util.IsPastDate(*debt.DueDate) {
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
	if debt.Status == model.DebtStatusCompleted {
		return nil, ErrDebtAlreadyCompleted
	}

	if req.Amount <= 0 || math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) || req.Amount > math.Round((debt.TotalAmount-debt.PaidAmount)*100)/100 {
		return nil, ErrInvalidRepayment
	}
	// Do not mutate the repository snapshot before persistence succeeds.
	copy := *debt
	debt = &copy

	paidAt := util.Now()
	if req.PaidAt != nil {
		paidAt = *req.PaidAt
	}

	repayment := &model.Repayment{
		ID:        uuid.New(),
		DebtID:    debtID,
		Amount:    req.Amount,
		PaidAt:    paidAt,
		Notes:     req.Notes,
		WalletID:  req.WalletID,
		CreatedAt: util.Now(),
	}

	debt.PaidAmount += req.Amount
	debt.UpdatedAt = util.Now()

	if debt.PaidAmount >= debt.TotalAmount {
		debt.Status = model.DebtStatusCompleted
	} else if debt.DueDate != nil && util.IsPastDate(*debt.DueDate) {
		debt.Status = model.DebtStatusOverdue
	} else {
		debt.Status = model.DebtStatusPending
	}

	var balanceDelta float64
	if req.WalletID != nil {
		balanceDelta = model.DebtRepaymentWalletDelta(debt.Type, req.Amount)
	}

	if err := s.debtRepo.AddRepaymentWithWallet(ctx, debt, repayment, req.WalletID, balanceDelta); err != nil {
		return nil, err
	}

	debt.Repayments = append(debt.Repayments, *repayment)
	return s.toResponse(debt), nil
}

// DeleteDebt soft-deletes the debt and its repayments and reverses their wallet
// effects in one DB transaction (see model.DebtCreationWalletDelta).
func (s *debtService) DeleteDebt(ctx context.Context, userID uuid.UUID, debtID uuid.UUID) error {
	err := s.debtRepo.Delete(ctx, debtID, userID)
	if errors.Is(err, repository.ErrDebtNotFound) {
		return ErrDebtNotFound
	}
	return err
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
		WalletID:    debt.WalletID,
		Repayments:  debt.Repayments,
		CreatedAt:   debt.CreatedAt,
		UpdatedAt:   debt.UpdatedAt,
	}
}
