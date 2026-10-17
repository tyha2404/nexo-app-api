package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"math"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrSameWalletTransfer  = errors.New("fromWalletId and toWalletId cannot be the same")
	ErrInsufficientBalance = constant.ErrInsufficientBalance
	ErrInvalidTransfer     = fmt.Errorf("transfer amount must be > 0 and fee must not be negative: %w", constant.ErrInvalidInput)
	// ErrAllocationPercentExceeded: a user's wallets may allocate at most 100% in total.
	ErrAllocationPercentExceeded = repository.ErrAllocationPercentExceeded
	ErrInvalidAllocationPercent  = fmt.Errorf("allocationPercent must be between 0 and 100: %w", constant.ErrInvalidInput)
)

// validateAllocationPercent rejects a wallet allocation that would push the
// total of the user's wallets above 100%. excludeWalletID is the wallet being
// updated (its current percent is replaced by percent).
func (s *walletService) validateAllocationPercent(ctx context.Context, userID uuid.UUID, excludeWalletID *uuid.UUID, percent float64) error {
	if percent < 0 || percent > 100 || math.IsNaN(percent) {
		return ErrInvalidAllocationPercent
	}
	if percent == 0 {
		return nil
	}
	others, err := s.repo.SumAllocationPercent(ctx, userID, excludeWalletID)
	if err != nil {
		return err
	}
	if others+percent > 100.0001 {
		return ErrAllocationPercentExceeded
	}
	return nil
}

type WalletService interface {
	CreateWallet(ctx context.Context, userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error)
	GetWallets(ctx context.Context, userID uuid.UUID) (*dto.WalletSummaryResponse, error)
	GetWalletByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*dto.WalletResponse, error)
	UpdateWallet(ctx context.Context, userID uuid.UUID, id uuid.UUID, req dto.UpdateWalletRequest) (*dto.WalletResponse, error)
	DeleteWallet(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	TransferMoney(ctx context.Context, userID uuid.UUID, req dto.TransferMoneyRequest) (*dto.WalletTransferResponse, error)
	AutoAllocateIncome(ctx context.Context, userID uuid.UUID, req dto.AutoAllocateRequest) (*dto.AutoAllocateResponse, error)
}

type walletService struct {
	repo repository.WalletRepository
}

func NewWalletService(repo repository.WalletRepository) WalletService {
	return &walletService{repo: repo}
}

func (s *walletService) CreateWallet(ctx context.Context, userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error) {
	currency := "VND"
	if req.Currency != "" {
		currency = req.Currency
	}
	isIncluded := true
	if req.IsIncludedInTotal != nil {
		isIncluded = *req.IsIncludedInTotal
	}
	if err := s.validateAllocationPercent(ctx, userID, nil, req.AllocationPercent); err != nil {
		return nil, err
	}

	wallet := &model.Wallet{
		UserID:            userID,
		Name:              req.Name,
		Type:              req.Type,
		Balance:           req.Balance,
		Currency:          currency,
		Icon:              req.Icon,
		JarCategory:       req.JarCategory,
		AllocationPercent: req.AllocationPercent,
		IsIncludedInTotal: isIncluded,
		CreditLimit:       req.CreditLimit,
		StatementDay:      req.StatementDay,
		DueDay:            req.DueDay,
		StatementBalance:  req.StatementBalance,
		MinimumPayment:    req.MinimumPayment,
		PreviousBalance:   req.PreviousBalance,
	}

	if err := s.repo.Create(ctx, wallet); err != nil {
		return nil, err
	}

	res := dto.ToWalletResponse(wallet)
	return &res, nil
}

func (s *walletService) GetWallets(ctx context.Context, userID uuid.UUID) (*dto.WalletSummaryResponse, error) {
	return s.repo.GetSummaryByUserID(ctx, userID)
}

func (s *walletService) GetWalletByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*dto.WalletResponse, error) {
	wallet, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if wallet == nil {
		return nil, ErrWalletNotFound
	}
	res := dto.ToWalletResponse(wallet)
	return &res, nil
}

func (s *walletService) UpdateWallet(ctx context.Context, userID uuid.UUID, id uuid.UUID, req dto.UpdateWalletRequest) (*dto.WalletResponse, error) {
	wallet, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if wallet == nil {
		return nil, ErrWalletNotFound
	}

	if req.Name != nil {
		wallet.Name = *req.Name
	}
	if req.Type != nil {
		wallet.Type = *req.Type
	}
	if req.Balance != nil {
		wallet.Balance = *req.Balance
	}
	if req.Currency != nil {
		wallet.Currency = *req.Currency
	}
	if req.Icon != nil {
		wallet.Icon = *req.Icon
	}
	if req.JarCategory != nil {
		wallet.JarCategory = req.JarCategory
	}
	if req.AllocationPercent != nil {
		if err := s.validateAllocationPercent(ctx, userID, &wallet.ID, *req.AllocationPercent); err != nil {
			return nil, err
		}
		wallet.AllocationPercent = *req.AllocationPercent
	}
	if req.IsIncludedInTotal != nil {
		wallet.IsIncludedInTotal = *req.IsIncludedInTotal
	}
	if req.CreditLimit != nil {
		wallet.CreditLimit = req.CreditLimit
	}
	if req.StatementDay != nil {
		wallet.StatementDay = req.StatementDay
	}
	if req.DueDay != nil {
		wallet.DueDay = req.DueDay
	}
	if req.StatementBalance != nil {
		wallet.StatementBalance = req.StatementBalance
	}
	if req.MinimumPayment != nil {
		wallet.MinimumPayment = req.MinimumPayment
	}
	if req.PreviousBalance != nil {
		wallet.PreviousBalance = req.PreviousBalance
	}

	if err := s.repo.Update(ctx, wallet, req.Balance != nil); err != nil {
		return nil, err
	}

	res := dto.ToWalletResponse(wallet)
	return &res, nil
}

func (s *walletService) DeleteWallet(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	wallet, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if wallet == nil {
		return ErrWalletNotFound
	}
	return s.repo.Delete(ctx, id, userID)
}

func (s *walletService) TransferMoney(ctx context.Context, userID uuid.UUID, req dto.TransferMoneyRequest) (*dto.WalletTransferResponse, error) {
	if req.FromWalletID == req.ToWalletID {
		return nil, ErrSameWalletTransfer
	}
	// Enforced here too, not only by DTO validation (other callers, e.g. AI tools).
	if req.Fee < 0 || req.Amount <= 0 || math.IsNaN(req.Fee) || math.IsNaN(req.Amount) {
		return nil, ErrInvalidTransfer
	}

	transferDate := util.Now()
	if req.TransferDate != nil && !req.TransferDate.IsZero() {
		transferDate = req.TransferDate.Time
	}

	transfer := &model.WalletTransfer{
		UserID:       userID,
		FromWalletID: req.FromWalletID,
		ToWalletID:   req.ToWalletID,
		Amount:       req.Amount,
		Fee:          req.Fee,
		Note:         req.Note,
		TransferDate: transferDate,
	}

	if err := s.repo.TransferMoney(ctx, transfer); err != nil {
		return nil, err
	}

	return &dto.WalletTransferResponse{
		ID:           transfer.ID,
		UserID:       transfer.UserID,
		FromWalletID: transfer.FromWalletID,
		ToWalletID:   transfer.ToWalletID,
		Amount:       transfer.Amount,
		Fee:          transfer.Fee,
		Note:         transfer.Note,
		TransferDate: transfer.TransferDate.Format("2006-01-02"),
		CreatedAt:    transfer.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func (s *walletService) AutoAllocateIncome(ctx context.Context, userID uuid.UUID, req dto.AutoAllocateRequest) (*dto.AutoAllocateResponse, error) {
	return s.repo.AutoAllocateIncome(ctx, userID, &req)
}
