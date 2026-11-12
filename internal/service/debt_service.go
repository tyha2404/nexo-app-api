package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

var (
	ErrDebtNotFound     = repository.ErrDebtNotFound
	ErrInvalidRepayment = repository.ErrInvalidRepayment
	// ErrDebtAlreadyCompleted wraps ErrInvalidRepayment so callers treat it as a 400.
	ErrDebtAlreadyCompleted = fmt.Errorf("khoản nợ đã hoàn tất, không thể trả thêm: %w", repository.ErrInvalidRepayment)
	// ErrInvalidDebtUpdate is returned (as a 400) when an edit would leave the debt inconsistent.
	ErrInvalidDebtUpdate = constant.NewKindError(constant.ErrInvalidInput, "dữ liệu sửa khoản nợ không hợp lệ")
)

type DebtService interface {
	CreateDebt(ctx context.Context, userID uuid.UUID, req dto.CreateDebtRequest) (*dto.DebtResponse, error)
	GetDebts(ctx context.Context, userID uuid.UUID, debtType model.DebtType, status model.DebtStatus) ([]dto.DebtResponse, error)
	GetDebtSummary(ctx context.Context, userID uuid.UUID) (*dto.DebtSummaryResponse, error)
	AddRepayment(ctx context.Context, userID uuid.UUID, debtID uuid.UUID, req dto.AddRepaymentRequest) (*dto.DebtResponse, error)
	UpdateDebt(ctx context.Context, userID uuid.UUID, debtID uuid.UUID, req dto.UpdateDebtRequest) (*dto.DebtResponse, error)
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
	// Start/due dates are calendar days (YYYY-MM-DD, like transaction dates).
	startDate := debtDay(req.StartDate)
	if startDate == nil {
		startDate = util.CalendarDatePtr(&now)
	}
	dueDate := debtDay(req.DueDate)
	if dueDate != nil && util.IsPastDate(*dueDate) {
		status = model.DebtStatusOverdue
	}

	debt := &model.Debt{
		ID:          uuid.New(),
		UserID:      userID,
		Type:        req.Type,
		Title:       req.Title,
		TotalAmount: req.TotalAmount,
		PaidAmount:  0,
		StartDate:   startDate,
		DueDate:     dueDate,
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
		normalizeDebtDates(&debt)
		// Auto update status to overdue if applicable
		if debt.Status == model.DebtStatusPending && debt.DueDate != nil && util.IsPastDate(*debt.DueDate) {
			debt.Status = model.DebtStatusOverdue
		}
		res[i] = *s.toResponse(&debt)
	}

	return res, nil
}

func (s *debtService) GetDebtSummary(ctx context.Context, userID uuid.UUID) (*dto.DebtSummaryResponse, error) {
	sum, err := s.debtRepo.GetSummaryByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &dto.DebtSummaryResponse{
		TotalPayable:    sum.TotalPayable,
		TotalReceivable: sum.TotalReceivable,
		OverdueCount:    sum.OverdueCount,
		PendingCount:    sum.PendingCount,
	}, nil
}

func (s *debtService) AddRepayment(ctx context.Context, userID uuid.UUID, debtID uuid.UUID, req dto.AddRepaymentRequest) (*dto.DebtResponse, error) {
	debt, err := s.debtRepo.GetByIDForUser(ctx, userID, debtID)
	if err != nil {
		return nil, err
	}
	if debt.Status == model.DebtStatusCompleted {
		return nil, ErrDebtAlreadyCompleted
	}

	if req.Amount <= 0 || math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) || req.Amount > math.Round((debt.TotalAmount-debt.PaidAmount)*100)/100 {
		return nil, ErrInvalidRepayment
	}
	// Do not mutate the repository snapshot before persistence succeeds.
	snapshot := *debt
	debt = &snapshot

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
	debt.Status = deriveDebtStatus(debt)

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

// UpdateDebt replaces the debt's title, amount, dates, notes and wallet. The
// total cannot drop below what was already repaid; the status is recomputed.
func (s *debtService) UpdateDebt(ctx context.Context, userID uuid.UUID, debtID uuid.UUID, req dto.UpdateDebtRequest) (*dto.DebtResponse, error) {
	if req.TotalAmount <= 0 || math.IsNaN(req.TotalAmount) || math.IsInf(req.TotalAmount, 0) {
		return nil, fmt.Errorf("số tiền phải lớn hơn 0: %w", ErrInvalidDebtUpdate)
	}
	startDate := debtDay(req.StartDate)
	dueDate := debtDay(req.DueDate)
	if startDate != nil && dueDate != nil && dueDate.Before(*startDate) {
		return nil, fmt.Errorf("hạn thanh toán phải từ ngày bắt đầu trở đi: %w", ErrInvalidDebtUpdate)
	}

	debt, err := s.debtRepo.UpdateWithWallet(ctx, userID, debtID, func(d *model.Debt) error {
		if math.Round(req.TotalAmount*100) < math.Round(d.PaidAmount*100) {
			return fmt.Errorf("tổng số tiền không được nhỏ hơn số đã thanh toán (%.0f): %w", d.PaidAmount, ErrInvalidDebtUpdate)
		}
		d.Title = req.Title
		d.TotalAmount = req.TotalAmount
		if startDate != nil {
			d.StartDate = startDate
		}
		d.DueDate = dueDate
		d.Notes = req.Notes
		d.WalletID = req.WalletID
		d.Status = deriveDebtStatus(d)
		d.UpdatedAt = util.Now()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.toResponse(debt), nil
}

// DeleteDebt soft-deletes the debt and its repayments and reverses their wallet
// effects in one DB transaction (see model.DebtCreationWalletDelta).
func (s *debtService) DeleteDebt(ctx context.Context, userID uuid.UUID, debtID uuid.UUID) error {
	return s.debtRepo.Delete(ctx, userID, debtID)
}

// deriveDebtStatus returns the status a debt has after its paid amount or due
// date changed: COMPLETED once fully paid, OVERDUE past its due date
// (util.IsPastDate), PENDING otherwise.
func deriveDebtStatus(d *model.Debt) model.DebtStatus {
	switch {
	case d.PaidAmount >= d.TotalAmount:
		return model.DebtStatusCompleted
	case d.DueDate != nil && util.IsPastDate(*d.DueDate):
		return model.DebtStatusOverdue
	default:
		return model.DebtStatusPending
	}
}

// debtDay turns a requested start/due date into UTC midnight of its calendar
// day (see util.CalendarDate); nil or empty means "not set".
func debtDay(d *dto.CustomTime) *time.Time {
	if d == nil || d.IsZero() {
		return nil
	}
	return util.CalendarDatePtr(&d.Time)
}

// dayString formats an optional calendar day as YYYY-MM-DD for the API.
func dayString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := util.DateString(*t)
	return &s
}

// normalizeDebtDates turns stored start/due dates into UTC midnight of their
// calendar day, so the API always returns the day the user picked (this also
// recovers rows saved before the fix as local midnight shifted to 17:00 UTC).
func normalizeDebtDates(debt *model.Debt) {
	debt.StartDate = util.CalendarDatePtr(debt.StartDate)
	debt.DueDate = util.CalendarDatePtr(debt.DueDate)
}

func (s *debtService) toResponse(debt *model.Debt) *dto.DebtResponse {
	normalizeDebtDates(debt)
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
		StartDate:   dayString(debt.StartDate),
		DueDate:     dayString(debt.DueDate),
		Status:      debt.Status,
		Notes:       debt.Notes,
		WalletID:    debt.WalletID,
		Repayments:  debt.Repayments,
		CreatedAt:   debt.CreatedAt,
		UpdatedAt:   debt.UpdatedAt,
	}
}
