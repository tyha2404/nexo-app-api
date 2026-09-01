package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type CreditCardStatementRepository interface {
	Create(ctx context.Context, s *model.CreditCardStatement) error
	GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.CreditCardStatement, error)
	GetByWalletID(ctx context.Context, walletID uuid.UUID, userID uuid.UUID) ([]model.CreditCardStatement, error)
	List(ctx context.Context, userID uuid.UUID, walletID *uuid.UUID, year *int, month *int) ([]model.CreditCardStatement, error)
	Update(ctx context.Context, s *model.CreditCardStatement) error
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}

type creditCardStatementRepository struct {
	db *gorm.DB
}

func NewCreditCardStatementRepository(db *gorm.DB) CreditCardStatementRepository {
	return &creditCardStatementRepository{db: db}
}

func (r *creditCardStatementRepository) Create(ctx context.Context, s *model.CreditCardStatement) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *creditCardStatementRepository) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.CreditCardStatement, error) {
	var s model.CreditCardStatement
	err := r.db.WithContext(ctx).
		Preload("Wallet").
		Where("id = ? AND user_id = ?", id, userID).
		First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *creditCardStatementRepository) GetByWalletID(ctx context.Context, walletID uuid.UUID, userID uuid.UUID) ([]model.CreditCardStatement, error) {
	var items []model.CreditCardStatement
	err := r.db.WithContext(ctx).
		Preload("Wallet").
		Where("wallet_id = ? AND user_id = ?", walletID, userID).
		Order("statement_year DESC, statement_month DESC, statement_date DESC").
		Find(&items).Error
	return items, err
}

func (r *creditCardStatementRepository) List(ctx context.Context, userID uuid.UUID, walletID *uuid.UUID, year *int, month *int) ([]model.CreditCardStatement, error) {
	var items []model.CreditCardStatement
	query := r.db.WithContext(ctx).
		Preload("Wallet").
		Where("user_id = ?", userID)

	if walletID != nil {
		query = query.Where("wallet_id = ?", *walletID)
	}
	if year != nil && *year > 0 {
		query = query.Where("statement_year = ?", *year)
	}
	if month != nil && *month > 0 {
		query = query.Where("statement_month = ?", *month)
	}

	err := query.Order("statement_year DESC, statement_month DESC, statement_date DESC").Find(&items).Error
	return items, err
}

func (r *creditCardStatementRepository) Update(ctx context.Context, s *model.CreditCardStatement) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *creditCardStatementRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.CreditCardStatement{}).Error
}
