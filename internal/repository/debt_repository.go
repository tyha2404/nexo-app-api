package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidRepayment = errors.New("số tiền trả phải lớn hơn 0 và không vượt dư nợ")
	ErrDebtNotFound     = errors.New("debt not found")
)

type DebtRepository interface {
	Create(ctx context.Context, debt *model.Debt) error
	CreateWithWallet(ctx context.Context, debt *model.Debt, walletID *uuid.UUID, balanceDelta float64) error
	FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Debt, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, debtType model.DebtType, status model.DebtStatus) ([]model.Debt, error)
	AddRepayment(ctx context.Context, debt *model.Debt, repayment *model.Repayment) error
	AddRepaymentWithWallet(ctx context.Context, debt *model.Debt, repayment *model.Repayment, walletID *uuid.UUID, balanceDelta float64) error
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	GetSummaryByUserID(ctx context.Context, userID uuid.UUID) (*dto.DebtSummaryResponse, error)
}

type debtRepository struct {
	db *gorm.DB
}

func NewDebtRepository(db *gorm.DB) DebtRepository {
	return &debtRepository{db: db}
}

func (r *debtRepository) Create(ctx context.Context, debt *model.Debt) error {
	return r.db.WithContext(ctx).Create(debt).Error
}

func (r *debtRepository) CreateWithWallet(ctx context.Context, debt *model.Debt, walletID *uuid.UUID, balanceDelta float64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(debt).Error; err != nil {
			return err
		}
		if walletID != nil && *walletID != uuid.Nil && balanceDelta != 0 {
			var wallet model.Wallet
			if err := tx.Where("id = ? AND user_id = ?", *walletID, debt.UserID).First(&wallet).Error; err != nil {
				return fmt.Errorf("wallet not found: %w", err)
			}
			if err := tx.Model(&wallet).Update("balance", gorm.Expr("balance + ?", balanceDelta)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *debtRepository) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Debt, error) {
	var debt model.Debt
	err := r.db.WithContext(ctx).
		Preload("Repayments").
		Where("id = ? AND user_id = ?", id, userID).
		First(&debt).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &debt, nil
}

func (r *debtRepository) FindByUserID(ctx context.Context, userID uuid.UUID, debtType model.DebtType, status model.DebtStatus) ([]model.Debt, error) {
	var debts []model.Debt
	query := r.db.WithContext(ctx).Preload("Repayments").Where("user_id = ?", userID)

	if debtType != "" {
		query = query.Where("type = ?", debtType)
	}
	// Overdue is derived from due_date (not the stored flag, which is only
	// refreshed on writes): a non-completed debt past its due date is OVERDUE.
	// "Overdue" = due date before today in the app timezone (due today is not
	// overdue). Comparing with a DATE literal avoids session-timezone shifts.
	today := util.Today()
	switch status {
	case "":
	case model.DebtStatusOverdue:
		query = query.Where("status <> ? AND due_date IS NOT NULL AND due_date < CAST(? AS date)", model.DebtStatusCompleted, today)
	case model.DebtStatusPending:
		query = query.Where("status <> ? AND (due_date IS NULL OR due_date >= CAST(? AS date))", model.DebtStatusCompleted, today)
	default:
		query = query.Where("status = ?", status)
	}

	err := query.Order("due_date ASC, created_at DESC").Find(&debts).Error
	return debts, err
}

func (r *debtRepository) AddRepayment(ctx context.Context, debt *model.Debt, repayment *model.Repayment) error {
	return r.AddRepaymentWithWallet(ctx, debt, repayment, nil, 0)
}

func (r *debtRepository) AddRepaymentWithWallet(ctx context.Context, debt *model.Debt, repayment *model.Repayment, walletID *uuid.UUID, balanceDelta float64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if repayment.Amount <= 0 || math.IsNaN(repayment.Amount) || math.IsInf(repayment.Amount, 0) {
			return ErrInvalidRepayment
		}
		// PostgreSQL serializes concurrent updates to this row and rechecks the
		// remaining-amount predicate after waiting. Never write a stale total.
		updated := model.Debt{}
		result := tx.Model(&updated).Clauses(clause.Returning{}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL AND status <> ? AND paid_amount + ? <= total_amount", debt.ID, debt.UserID, model.DebtStatusCompleted, repayment.Amount).
			Updates(map[string]interface{}{
				"paid_amount": gorm.Expr("paid_amount + ?", repayment.Amount),
				"status":      gorm.Expr("CASE WHEN paid_amount + ? >= total_amount THEN ? WHEN due_date < CAST(? AS date) THEN ? ELSE ? END", repayment.Amount, model.DebtStatusCompleted, util.Today(), model.DebtStatusOverdue, model.DebtStatusPending),
				"updated_at":  time.Now(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInvalidRepayment
		}
		if err := tx.Create(repayment).Error; err != nil {
			return err
		}
		// Return the total actually committed, not the caller's stale snapshot.
		debt.PaidAmount = updated.PaidAmount
		debt.Status = updated.Status
		debt.UpdatedAt = updated.UpdatedAt

		if walletID != nil && *walletID != uuid.Nil && balanceDelta != 0 {
			var wallet model.Wallet
			if err := tx.Where("id = ? AND user_id = ?", *walletID, debt.UserID).First(&wallet).Error; err != nil {
				return fmt.Errorf("wallet not found: %w", err)
			}
			if err := tx.Model(&wallet).Update("balance", gorm.Expr("balance + ?", balanceDelta)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Delete soft-deletes the debt and its repayments and reverses every wallet
// effect they had (creation + each repayment) in one DB transaction. Wallets
// that were deleted meanwhile are skipped.
func (r *debtRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var debt model.Debt
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", id, userID).
			First(&debt).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDebtNotFound
		}
		if err != nil {
			return err
		}
		var repayments []model.Repayment
		if err := tx.Where("debt_id = ?", debt.ID).Find(&repayments).Error; err != nil {
			return err
		}
		for _, d := range DebtDeletionWalletReversals(&debt, repayments) {
			walletID := d.WalletID
			if err := adjustWalletBalance(tx, &walletID, debt.UserID, d.Delta, false); err != nil {
				return err
			}
		}
		if err := tx.Where("debt_id = ?", debt.ID).Delete(&model.Repayment{}).Error; err != nil {
			return err
		}
		return tx.Delete(&debt).Error
	})
}

// WalletDelta is a balance change to apply to one wallet.
type WalletDelta struct {
	WalletID uuid.UUID
	Delta    float64
}

// DebtDeletionWalletReversals returns the balance changes that undo the wallet
// effects of a debt's creation and of all its repayments, aggregated per wallet
// (in first-seen order) so each wallet is updated once.
func DebtDeletionWalletReversals(debt *model.Debt, repayments []model.Repayment) []WalletDelta {
	var out []WalletDelta
	index := map[uuid.UUID]int{}
	add := func(walletID *uuid.UUID, delta float64) {
		if walletID == nil || *walletID == uuid.Nil || delta == 0 {
			return
		}
		if i, ok := index[*walletID]; ok {
			out[i].Delta += delta
			return
		}
		index[*walletID] = len(out)
		out = append(out, WalletDelta{WalletID: *walletID, Delta: delta})
	}
	add(debt.WalletID, -model.DebtCreationWalletDelta(debt.Type, debt.TotalAmount))
	for _, rp := range repayments {
		add(rp.WalletID, -model.DebtRepaymentWalletDelta(debt.Type, rp.Amount))
	}
	return out
}

func (r *debtRepository) GetSummaryByUserID(ctx context.Context, userID uuid.UUID) (*dto.DebtSummaryResponse, error) {
	var summary dto.DebtSummaryResponse

	// Total Payable (Tôi Nợ)
	err := r.db.WithContext(ctx).
		Model(&model.Debt{}).
		Where("user_id = ? AND type = ? AND status != ?", userID, model.DebtTypePayable, model.DebtStatusCompleted).
		Select("COALESCE(SUM(total_amount - paid_amount), 0)").
		Scan(&summary.TotalPayable).Error
	if err != nil {
		return nil, err
	}

	// Total Receivable (Người Khác Nợ)
	err = r.db.WithContext(ctx).
		Model(&model.Debt{}).
		Where("user_id = ? AND type = ? AND status != ?", userID, model.DebtTypeReceivable, model.DebtStatusCompleted).
		Select("COALESCE(SUM(total_amount - paid_amount), 0)").
		Scan(&summary.TotalReceivable).Error
	if err != nil {
		return nil, err
	}

	// Overdue / pending counts are computed from due_date at query time instead
	// of the stored status flag, which goes stale once a due date passes.
	today := util.Today()
	err = r.db.WithContext(ctx).
		Model(&model.Debt{}).
		Where("user_id = ? AND status <> ? AND due_date IS NOT NULL AND due_date < CAST(? AS date)", userID, model.DebtStatusCompleted, today).
		Count(&summary.OverdueCount).Error
	if err != nil {
		return nil, err
	}

	err = r.db.WithContext(ctx).
		Model(&model.Debt{}).
		Where("user_id = ? AND status <> ? AND (due_date IS NULL OR due_date >= CAST(? AS date))", userID, model.DebtStatusCompleted, today).
		Count(&summary.PendingCount).Error
	if err != nil {
		return nil, err
	}

	return &summary, nil
}
