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
	ErrStatementNotFound = errors.New("không tìm thấy kỳ sao kê")
	ErrInvalidDate       = errors.New("định dạng ngày không hợp lệ (cần YYYY-MM-DD)")
)

type CreditCardStatementService interface {
	CreateStatement(ctx context.Context, userID uuid.UUID, req dto.CreateStatementRequest) (*dto.StatementResponse, error)
	GetStatementByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*dto.StatementResponse, error)
	ListStatements(ctx context.Context, userID uuid.UUID, walletID *uuid.UUID, year *int, month *int) (*dto.StatementListResponse, error)
	UpdateStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID, req dto.UpdateStatementRequest) (*dto.StatementResponse, error)
	PayStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID, req dto.PayStatementRequest) (*dto.StatementResponse, error)
	DeleteStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

type creditCardStatementService struct {
	repo       repository.CreditCardStatementRepository
	walletRepo repository.WalletRepository
}

func NewCreditCardStatementService(
	repo repository.CreditCardStatementRepository,
	walletRepo repository.WalletRepository,
) CreditCardStatementService {
	return &creditCardStatementService{
		repo:       repo,
		walletRepo: walletRepo,
	}
}

func parseDateString(str string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", str)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse(time.RFC3339, str)
	if err == nil {
		return t, nil
	}
	return time.Time{}, ErrInvalidDate
}

func (s *creditCardStatementService) CreateStatement(ctx context.Context, userID uuid.UUID, req dto.CreateStatementRequest) (*dto.StatementResponse, error) {
	wallet, err := s.walletRepo.GetByID(ctx, req.WalletID, userID)
	if err != nil {
		return nil, err
	}
	if wallet == nil {
		return nil, ErrWalletNotFound
	}

	stmtDate, err := parseDateString(req.StatementDate)
	if err != nil {
		return nil, err
	}

	dueDate, err := parseDateString(req.DueDate)
	if err != nil {
		return nil, err
	}

	minPay := req.MinimumPayment
	if minPay <= 0 && req.StatementBalance > 0 {
		minPay = req.StatementBalance * 0.05
		if minPay < 50000 && req.StatementBalance >= 50000 {
			minPay = 50000
		}
	}

	status := model.StatementStatusUnpaid
	if req.PaidAmount >= req.StatementBalance && req.StatementBalance > 0 {
		status = model.StatementStatusPaid
	} else if req.PaidAmount > 0 {
		status = model.StatementStatusPartiallyPaid
	}

	note := ""
	if req.Note != nil {
		note = *req.Note
	}

	stmt := &model.CreditCardStatement{
		ID:               uuid.New(),
		UserID:           userID,
		WalletID:         req.WalletID,
		StatementMonth:   req.StatementMonth,
		StatementYear:    req.StatementYear,
		StatementDate:    stmtDate,
		DueDate:          dueDate,
		StatementBalance: req.StatementBalance,
		MinimumPayment:   minPay,
		PreviousBalance:  req.PreviousBalance,
		PaidAmount:       req.PaidAmount,
		Status:           status,
		Note:             note,
	}

	if err := s.repo.Create(ctx, stmt); err != nil {
		return nil, err
	}

	// Also sync latest active statement to wallet
	wallet.StatementBalance = &req.StatementBalance
	wallet.MinimumPayment = &minPay
	wallet.PreviousBalance = &req.PreviousBalance
	_ = s.walletRepo.Update(ctx, wallet)

	stmt.Wallet = wallet
	res := dto.ToStatementResponse(stmt)
	return &res, nil
}

func (s *creditCardStatementService) GetStatementByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*dto.StatementResponse, error) {
	stmt, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if stmt == nil {
		return nil, ErrStatementNotFound
	}
	res := dto.ToStatementResponse(stmt)
	return &res, nil
}

func (s *creditCardStatementService) ListStatements(ctx context.Context, userID uuid.UUID, walletID *uuid.UUID, year *int, month *int) (*dto.StatementListResponse, error) {
	items, err := s.repo.List(ctx, userID, walletID, year, month)
	if err != nil {
		return nil, err
	}

	resItems := make([]dto.StatementResponse, 0, len(items))
	var totalBal, totalPaid, totalRemaining float64

	for _, item := range items {
		resp := dto.ToStatementResponse(&item)
		totalBal += resp.StatementBalance
		totalPaid += resp.PaidAmount
		totalRemaining += resp.RemainingAmount
		resItems = append(resItems, resp)
	}

	return &dto.StatementListResponse{
		Items:          resItems,
		Total:          int64(len(resItems)),
		TotalBalance:   totalBal,
		TotalPaid:      totalPaid,
		TotalRemaining: totalRemaining,
	}, nil
}

func (s *creditCardStatementService) UpdateStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID, req dto.UpdateStatementRequest) (*dto.StatementResponse, error) {
	stmt, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if stmt == nil {
		return nil, ErrStatementNotFound
	}

	if req.StatementMonth != nil {
		stmt.StatementMonth = *req.StatementMonth
	}
	if req.StatementYear != nil {
		stmt.StatementYear = *req.StatementYear
	}
	if req.StatementDate != nil {
		d, err := parseDateString(*req.StatementDate)
		if err == nil {
			stmt.StatementDate = d
		}
	}
	if req.DueDate != nil {
		d, err := parseDateString(*req.DueDate)
		if err == nil {
			stmt.DueDate = d
		}
	}
	if req.StatementBalance != nil {
		stmt.StatementBalance = *req.StatementBalance
	}
	if req.MinimumPayment != nil {
		stmt.MinimumPayment = *req.MinimumPayment
	}
	if req.PreviousBalance != nil {
		stmt.PreviousBalance = *req.PreviousBalance
	}
	if req.PaidAmount != nil {
		stmt.PaidAmount = *req.PaidAmount
	}
	if req.Status != nil {
		stmt.Status = *req.Status
	}
	if req.Note != nil {
		stmt.Note = *req.Note
	}

	// Evaluate status
	if stmt.PaidAmount >= stmt.StatementBalance && stmt.StatementBalance > 0 {
		stmt.Status = model.StatementStatusPaid
	} else if stmt.PaidAmount > 0 {
		stmt.Status = model.StatementStatusPartiallyPaid
	} else {
		stmt.Status = model.StatementStatusUnpaid
	}

	if err := s.repo.Update(ctx, stmt); err != nil {
		return nil, err
	}

	res := dto.ToStatementResponse(stmt)
	return &res, nil
}

func (s *creditCardStatementService) PayStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID, req dto.PayStatementRequest) (*dto.StatementResponse, error) {
	stmt, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if stmt == nil {
		return nil, ErrStatementNotFound
	}

	// Update paid amount
	stmt.PaidAmount += req.Amount
	if stmt.PaidAmount >= stmt.StatementBalance && stmt.StatementBalance > 0 {
		stmt.Status = model.StatementStatusPaid
	} else {
		stmt.Status = model.StatementStatusPartiallyPaid
	}

	if err := s.repo.Update(ctx, stmt); err != nil {
		return nil, err
	}

	// If source wallet is provided, execute balance deduction
	if req.SourceWalletID != nil {
		sourceWallet, err := s.walletRepo.GetByID(ctx, *req.SourceWalletID, userID)
		if err == nil && sourceWallet != nil {
			sourceWallet.Balance -= req.Amount
			_ = s.walletRepo.Update(ctx, sourceWallet)
		}
	}

	// Also reduce credit card debt
	cardWallet, err := s.walletRepo.GetByID(ctx, stmt.WalletID, userID)
	if err == nil && cardWallet != nil {
		cardWallet.Balance += req.Amount // Adding to negative balance reduces debt
		rem := stmt.StatementBalance - stmt.PaidAmount
		if rem < 0 {
			rem = 0
		}
		cardWallet.StatementBalance = &rem
		_ = s.walletRepo.Update(ctx, cardWallet)
	}

	res := dto.ToStatementResponse(stmt)
	return &res, nil
}

func (s *creditCardStatementService) DeleteStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	return s.repo.Delete(ctx, id, userID)
}
