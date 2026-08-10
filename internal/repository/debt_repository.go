package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type DebtRepository interface {
	Create(ctx context.Context, debt *model.Debt) error
	FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Debt, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, debtType model.DebtType, status model.DebtStatus) ([]model.Debt, error)
	AddRepayment(ctx context.Context, debt *model.Debt, repayment *model.Repayment) error
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
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("due_date ASC, created_at DESC").Find(&debts).Error
	return debts, err
}

func (r *debtRepository) AddRepayment(ctx context.Context, debt *model.Debt, repayment *model.Repayment) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(repayment).Error; err != nil {
			return err
		}
		return tx.Model(debt).Updates(map[string]interface{}{
			"paid_amount": debt.PaidAmount,
			"status":      debt.Status,
			"updated_at":  debt.UpdatedAt,
		}).Error
	})
}

func (r *debtRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.Debt{}).Error
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

	// Overdue count
	err = r.db.WithContext(ctx).
		Model(&model.Debt{}).
		Where("user_id = ? AND status = ?", userID, model.DebtStatusOverdue).
		Count(&summary.OverdueCount).Error
	if err != nil {
		return nil, err
	}

	// Pending count
	err = r.db.WithContext(ctx).
		Model(&model.Debt{}).
		Where("user_id = ? AND status = ?", userID, model.DebtStatusPending).
		Count(&summary.PendingCount).Error
	if err != nil {
		return nil, err
	}

	return &summary, nil
}
