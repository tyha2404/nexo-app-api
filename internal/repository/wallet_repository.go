package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type WalletRepository interface {
	Create(ctx context.Context, wallet *model.Wallet) error
	GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Wallet, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error)
	Update(ctx context.Context, wallet *model.Wallet) error
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	TransferMoney(ctx context.Context, transfer *model.WalletTransfer) error
	AutoAllocateIncome(ctx context.Context, userID uuid.UUID, req *dto.AutoAllocateRequest) (*dto.AutoAllocateResponse, error)
	GetSummaryByUserID(ctx context.Context, userID uuid.UUID) (*dto.WalletSummaryResponse, error)
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

func (r *walletRepository) Update(ctx context.Context, wallet *model.Wallet) error {
	return r.db.WithContext(ctx).Save(wallet).Error
}

func (r *walletRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.Wallet{}).Error
}

func (r *walletRepository) TransferMoney(ctx context.Context, transfer *model.WalletTransfer) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var fromWallet model.Wallet
		if err := tx.Where("id = ? AND user_id = ?", transfer.FromWalletID, transfer.UserID).First(&fromWallet).Error; err != nil {
			return fmt.Errorf("source wallet not found: %w", err)
		}

		totalDeduction := transfer.Amount + transfer.Fee
		if fromWallet.Balance < totalDeduction {
			return fmt.Errorf("insufficient balance in source wallet")
		}

		var toWallet model.Wallet
		if err := tx.Where("id = ? AND user_id = ?", transfer.ToWalletID, transfer.UserID).First(&toWallet).Error; err != nil {
			return fmt.Errorf("destination wallet not found: %w", err)
		}

		if err := tx.Model(&fromWallet).Update("balance", gorm.Expr("balance - ?", totalDeduction)).Error; err != nil {
			return err
		}

		if err := tx.Model(&toWallet).Update("balance", gorm.Expr("balance + ?", transfer.Amount)).Error; err != nil {
			return err
		}

		if err := tx.Create(transfer).Error; err != nil {
			return err
		}

		return nil
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
		if req.SourceWalletID != uuid.Nil {
			var sourceWallet model.Wallet
			if err := tx.Where("id = ? AND user_id = ?", req.SourceWalletID, userID).First(&sourceWallet).Error; err != nil {
				return fmt.Errorf("source wallet not found: %w", err)
			}
		}

		var targetWallets []model.Wallet
		if err := tx.Where("user_id = ? AND allocation_percent > 0", userID).Find(&targetWallets).Error; err != nil {
			return err
		}

		if len(targetWallets) == 0 {
			return fmt.Errorf("no target wallets configured with allocation percentage")
		}

		var totalAllocated float64
		var details []dto.AllocatedWalletDetail

		for i := range targetWallets {
			w := &targetWallets[i]
			allocated := (req.IncomeAmount * w.AllocationPercent) / 100.0
			w.Balance += allocated

			if err := tx.Model(w).Update("balance", w.Balance).Error; err != nil {
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
			if err := tx.Model(&model.Wallet{}).
				Where("id = ? AND user_id = ?", req.SourceWalletID, userID).
				Update("balance", gorm.Expr("balance - ?", totalAllocated)).Error; err != nil {
				return err
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

	var totalBalance float64
	walletDTOs := make([]dto.WalletResponse, 0, len(wallets))

	for _, w := range wallets {
		if w.IsIncludedInTotal {
			totalBalance += w.Balance
		}
		walletDTOs = append(walletDTOs, dto.WalletResponse{
			ID:                w.ID,
			UserID:            w.UserID,
			Name:              w.Name,
			Type:              w.Type,
			Balance:           w.Balance,
			Currency:          w.Currency,
			Icon:              w.Icon,
			JarCategory:       w.JarCategory,
			AllocationPercent: w.AllocationPercent,
			IsIncludedInTotal: w.IsIncludedInTotal,
			CreatedAt:         w.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:         w.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return &dto.WalletSummaryResponse{
		TotalBalance: totalBalance,
		Wallets:      walletDTOs,
		TotalWallets: len(wallets),
	}, nil
}
