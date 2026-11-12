package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/tyha2404/nexo-app-api/internal/constant"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WalletRepository interface {
	Create(ctx context.Context, wallet *model.Wallet) error
	GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Wallet, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error)
	Update(ctx context.Context, wallet *model.Wallet, includeBalance bool) error
	Delete(ctx context.Context, userID, id uuid.UUID) error
	TransferMoney(ctx context.Context, transfer *model.WalletTransfer) error
	// AutoAllocateIncome splits incomeAmount over the wallets with an
	// allocation percent (and debits sourceWalletID unless it is uuid.Nil).
	AutoAllocateIncome(ctx context.Context, userID, sourceWalletID uuid.UUID, incomeAmount float64) (*AutoAllocation, error)
	UpdateStatementSnapshot(ctx context.Context, userID, walletID uuid.UUID, statementBalance, minimumPayment, previousBalance float64) error
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

// ErrWalletNotFound: no live wallet with that id belongs to the user.
var ErrWalletNotFound = constant.NewKindError(constant.ErrNotFound, "wallet not found")

// GetByID returns the user's wallet, ErrWalletNotFound when it is missing or
// belongs to someone else.
func (r *walletRepository) GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Wallet, error) {
	var wallet model.Wallet
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&wallet).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWalletNotFound
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

// walletEditableColumns are written by Update (an explicit Select also writes
// zero values and NULLs, e.g. allocation_percent = 0 or is_included_in_total =
// false). balance is added only on request, see Update.
var walletEditableColumns = []string{
	"name", "type", "currency", "icon", "jar_category", "allocation_percent", "is_included_in_total",
	"credit_limit", "statement_day", "due_day", "statement_balance", "minimum_payment", "previous_balance", "updated_at",
}

// Update writes the wallet's editable columns, scoped by id AND user_id.
// balance is written only when includeBalance is true (the user explicitly set
// it); otherwise the stale in-memory balance of this read-modify-write is never
// persisted, so a concurrent transaction/transfer that moved the balance is not
// overwritten. Unlike Save, it never inserts: a wallet deleted meanwhile is not
// resurrected and constant.ErrNotFound is returned instead.
func (r *walletRepository) Update(ctx context.Context, wallet *model.Wallet, includeBalance bool) error {
	cols := walletEditableColumns
	if includeBalance {
		cols = append(append([]string{}, walletEditableColumns...), "balance")
	}
	result := r.db.WithContext(ctx).Model(wallet).Omit(clause.Associations).
		Where("id = ? AND user_id = ?", wallet.ID, wallet.UserID).
		Select(cols).
		Updates(wallet)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("wallet %w", constant.ErrNotFound)
	}
	return nil
}

// Delete soft-deletes the user's wallet; "wallet not found"
// (constant.ErrNotFound) when no live wallet with that id belongs to the user.
func (r *walletRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return deleteForUser(r.db.WithContext(ctx), &model.Wallet{}, "wallet", userID, id)
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

// WalletAllocation is the share of an auto-allocated income one wallet got.
type WalletAllocation struct {
	WalletID          uuid.UUID
	WalletName        string
	AllocationPercent float64
	AllocatedAmount   float64
	NewBalance        float64
}

// AutoAllocation is the outcome of AutoAllocateIncome.
type AutoAllocation struct {
	Allocations    []WalletAllocation
	TotalAllocated float64
}

func (r *walletRepository) AutoAllocateIncome(ctx context.Context, userID, sourceWalletID uuid.UUID, incomeAmount float64) (*AutoAllocation, error) {
	resp := &AutoAllocation{}

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
		details := make([]WalletAllocation, 0, len(targetWallets))
		for i := range targetWallets {
			w := &targetWallets[i]
			allocated := (incomeAmount * w.AllocationPercent) / 100.0
			// Relative update; RETURNING gives the committed balance for the response.
			if err := tx.Model(w).
				Clauses(clause.Returning{Columns: []clause.Column{{Name: "balance"}}}).
				Update("balance", gorm.Expr("balance + ?", allocated)).Error; err != nil {
				return err
			}
			totalAllocated += allocated
			details = append(details, WalletAllocation{
				WalletID:          w.ID,
				WalletName:        w.Name,
				AllocationPercent: w.AllocationPercent,
				AllocatedAmount:   allocated,
				NewBalance:        w.Balance,
			})
		}

		if sourceWalletID != uuid.Nil {
			found, err := applyWalletEffect(tx, &sourceWalletID, userID, -totalAllocated)
			if err != nil {
				return err
			}
			if !found {
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

// UpdateStatementSnapshot writes only the credit-card statement snapshot columns.
// It never touches balance, so it cannot clobber a concurrent balance change the
// way a read-modify-write Save of the whole wallet would.
func (r *walletRepository) UpdateStatementSnapshot(ctx context.Context, userID, walletID uuid.UUID, statementBalance, minimumPayment, previousBalance float64) error {
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
