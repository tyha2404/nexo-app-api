package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"strings"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WalletRepository interface {
	Create(ctx context.Context, wallet *model.Wallet) error
	GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Wallet, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error)
	Update(ctx context.Context, wallet *model.Wallet, includeBalance bool) error
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	TransferMoney(ctx context.Context, transfer *model.WalletTransfer) error
	AutoAllocateIncome(ctx context.Context, userID uuid.UUID, req *dto.AutoAllocateRequest) (*dto.AutoAllocateResponse, error)
	GetSummaryByUserID(ctx context.Context, userID uuid.UUID) (*dto.WalletSummaryResponse, error)
	UpdateStatementSnapshot(ctx context.Context, walletID, userID uuid.UUID, statementBalance, minimumPayment, previousBalance float64) error
	SumAllocationPercent(ctx context.Context, userID uuid.UUID, excludeWalletID *uuid.UUID) (float64, error)
}

type walletRepository struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) WalletRepository {
	return &walletRepository{db: db}
}

func (r *walletRepository) Create(ctx context.Context, wallet *model.Wallet) error {
	return r.db.WithContext(ctx).Create(wallet).Error
}

func (r *walletRepository) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Wallet, error) {
	var wallet model.Wallet
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&wallet).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &wallet, nil
}

func (r *walletRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error) {
	var wallets []model.Wallet
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at ASC").Find(&wallets).Error
	return wallets, err
}

// Update saves the wallet's editable fields. balance is written only when
// includeBalance is true (the user explicitly set it); otherwise the stale
// in-memory balance of this read-modify-write is never persisted, so a
// concurrent transaction/transfer that moved the balance is not overwritten.
func (r *walletRepository) Update(ctx context.Context, wallet *model.Wallet, includeBalance bool) error {
	omit := []string{clause.Associations, "user_id", "created_at"}
	if !includeBalance {
		omit = append(omit, "balance")
	}
	return r.db.WithContext(ctx).Omit(omit...).Save(wallet).Error
}

func (r *walletRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.Wallet{}).Error
}

// ErrAllocationPercentExceeded means the user's wallets allocate more than 100%.
var ErrAllocationPercentExceeded = fmt.Errorf("tổng tỷ lệ phân bổ của các ví vượt quá 100%%: %w", constant.ErrInvalidInput)

func (r *walletRepository) TransferMoney(ctx context.Context, transfer *model.WalletTransfer) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock both wallets FOR UPDATE before the balance check, always in id
		// order so two opposite concurrent transfers cannot deadlock. Without the
		// lock two transfers could both pass the check and overdraw the source.
		var locked []model.Wallet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ? AND user_id = ?", []uuid.UUID{transfer.FromWalletID, transfer.ToWalletID}, transfer.UserID).
			Order("id").
			Find(&locked).Error; err != nil {
			return err
		}
		var fromWallet, toWallet *model.Wallet
		for i := range locked {
			switch locked[i].ID {
			case transfer.FromWalletID:
				fromWallet = &locked[i]
			case transfer.ToWalletID:
				toWallet = &locked[i]
			}
		}
		if fromWallet == nil {
			return fmt.Errorf("source wallet %w", constant.ErrNotFound)
		}
		if toWallet == nil {
			return fmt.Errorf("destination wallet %w", constant.ErrNotFound)
		}

		totalDeduction := transfer.Amount + transfer.Fee
		if fromWallet.Balance < totalDeduction {
			return constant.ErrInsufficientBalance
		}

		if err := tx.Model(fromWallet).Update("balance", gorm.Expr("balance - ?", totalDeduction)).Error; err != nil {
			return err
		}
		if err := tx.Model(toWallet).Update("balance", gorm.Expr("balance + ?", transfer.Amount)).Error; err != nil {
			return err
		}
		return tx.Create(transfer).Error
	})
}

func (r *walletRepository) AutoAllocateIncome(ctx context.Context, userID uuid.UUID, req *dto.AutoAllocateRequest) (*dto.AutoAllocateResponse, error) {
	resp := &dto.AutoAllocateResponse{
		IncomeAmount: req.IncomeAmount,
	}
	if req.SourceWalletID != uuid.Nil {
		resp.SourceWalletID = &req.SourceWalletID
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var targetWallets []model.Wallet
		if err := tx.Where("user_id = ? AND allocation_percent > 0", userID).Order("id").Find(&targetWallets).Error; err != nil {
			return err
		}
		if len(targetWallets) == 0 {
			return fmt.Errorf("no target wallets configured with allocation percentage: %w", constant.ErrInvalidInput)
		}
		var totalPercent float64
		for _, w := range targetWallets {
			totalPercent += w.AllocationPercent
		}
		if totalPercent > 100.0001 {
			return ErrAllocationPercentExceeded
		}

		var totalAllocated float64
		details := make([]dto.AllocatedWalletDetail, 0, len(targetWallets))
		for i := range targetWallets {
			w := &targetWallets[i]
			allocated := (req.IncomeAmount * w.AllocationPercent) / 100.0
			// Relative update; RETURNING gives the committed balance for the response.
			if err := tx.Model(w).
				Clauses(clause.Returning{Columns: []clause.Column{{Name: "balance"}}}).
				Update("balance", gorm.Expr("balance + ?", allocated)).Error; err != nil {
				return err
			}
			totalAllocated += allocated
			details = append(details, dto.AllocatedWalletDetail{
				WalletID:          w.ID,
				WalletName:        w.Name,
				AllocationPercent: w.AllocationPercent,
				AllocatedAmount:   allocated,
				NewBalance:        w.Balance,
			})
		}

		if req.SourceWalletID != uuid.Nil {
			res := tx.Model(&model.Wallet{}).
				Where("id = ? AND user_id = ?", req.SourceWalletID, userID).
				Update("balance", gorm.Expr("balance - ?", totalAllocated))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("source wallet %w", constant.ErrNotFound)
			}
		}

		resp.Allocations = details
		resp.TotalAllocated = totalAllocated
		return nil
	})

	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (r *walletRepository) GetSummaryByUserID(ctx context.Context, userID uuid.UUID) (*dto.WalletSummaryResponse, error) {
	wallets, err := r.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return SummarizeWallets(wallets), nil
}

// BaseCurrency is the user's base currency; TotalBalance is expressed in it.
const BaseCurrency = "VND"

// SummarizeWallets builds the wallet summary. Balances in different currencies
// are never added together (there is no FX conversion): TotalBalance only sums
// included wallets in BaseCurrency (an empty currency counts as VND, the column
// default), and TotalsByCurrency reports every currency separately.
func SummarizeWallets(wallets []model.Wallet) *dto.WalletSummaryResponse {
	totals := make(map[string]float64)
	walletDTOs := make([]dto.WalletResponse, 0, len(wallets))
	for i := range wallets {
		w := &wallets[i]
		if w.IsIncludedInTotal {
			currency := strings.ToUpper(strings.TrimSpace(w.Currency))
			if currency == "" {
				currency = BaseCurrency
			}
			totals[currency] += w.Balance
		}
		walletDTOs = append(walletDTOs, dto.ToWalletResponse(w))
	}
	return &dto.WalletSummaryResponse{
		TotalBalance:     totals[BaseCurrency],
		BaseCurrency:     BaseCurrency,
		TotalsByCurrency: totals,
		Wallets:          walletDTOs,
		TotalWallets:     len(wallets),
	}
}

// UpdateStatementSnapshot writes only the credit-card statement snapshot columns.
// It never touches balance, so it cannot clobber a concurrent balance change the
// way a read-modify-write Save of the whole wallet would.
func (r *walletRepository) UpdateStatementSnapshot(ctx context.Context, walletID, userID uuid.UUID, statementBalance, minimumPayment, previousBalance float64) error {
	res := r.db.WithContext(ctx).Model(&model.Wallet{}).
		Where("id = ? AND user_id = ?", walletID, userID).
		Updates(map[string]interface{}{
			"statement_balance": statementBalance,
			"minimum_payment":   minimumPayment,
			"previous_balance":  previousBalance,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("wallet %w", constant.ErrNotFound)
	}
	return nil
}

// SumAllocationPercent returns the total allocation_percent of the user's live
// wallets, optionally excluding one wallet (the one being updated).
func (r *walletRepository) SumAllocationPercent(ctx context.Context, userID uuid.UUID, excludeWalletID *uuid.UUID) (float64, error) {
	var total float64
	q := r.db.WithContext(ctx).Model(&model.Wallet{}).Where("user_id = ?", userID)
	if excludeWalletID != nil {
		q = q.Where("id <> ?", *excludeWalletID)
	}
	err := q.Select("COALESCE(SUM(allocation_percent), 0)").Scan(&total).Error
	return total, err
}
