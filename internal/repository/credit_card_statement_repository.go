package repository

import (
	"errors"
	"fmt"
	"math"

	"context"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tyha2404/nexo-app-api/internal/constant"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CreditCardStatementRepository interface {
	Create(ctx context.Context, s *model.CreditCardStatement) error
	GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.CreditCardStatement, error)
	GetByWalletID(ctx context.Context, walletID uuid.UUID, userID uuid.UUID) ([]model.CreditCardStatement, error)
	List(ctx context.Context, userID uuid.UUID, walletID *uuid.UUID, year *int, month *int) ([]model.CreditCardStatement, error)
	Update(ctx context.Context, s *model.CreditCardStatement) error
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	PayStatement(ctx context.Context, userID, id uuid.UUID, amount float64, sourceWalletID *uuid.UUID) (*model.CreditCardStatement, error)
}

var (
	// ErrStatementAlreadyPaid is returned when paying a statement whose status is PAID.
	ErrStatementAlreadyPaid = errors.New("kỳ sao kê đã được thanh toán đủ")
	// ErrStatementInvalidPayment is returned for a non-positive amount or one
	// larger than the remaining statement balance.
	ErrStatementInvalidPayment = errors.New("số tiền thanh toán phải lớn hơn 0 và không vượt số dư còn lại của sao kê")
)

type creditCardStatementRepository struct {
	db *gorm.DB
}

func NewCreditCardStatementRepository(db *gorm.DB) CreditCardStatementRepository {
	return &creditCardStatementRepository{db: db}
}

func (r *creditCardStatementRepository) Create(ctx context.Context, s *model.CreditCardStatement) error {
	err := r.db.WithContext(ctx).Create(s).Error
	// 23505 = unique_violation: a statement for this card and period already exists
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%w: credit card statement for this period", constant.ErrAlreadyExists)
	}
	return err
}

func (r *creditCardStatementRepository) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.CreditCardStatement, error) {
	var s model.CreditCardStatement
	err := r.db.WithContext(ctx).
		Preload("Wallet", withDeleted).
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
		Preload("Wallet", withDeleted).
		Where("wallet_id = ? AND user_id = ?", walletID, userID).
		Order("statement_year DESC, statement_month DESC, statement_date DESC").
		Find(&items).Error
	return items, err
}

func (r *creditCardStatementRepository) List(ctx context.Context, userID uuid.UUID, walletID *uuid.UUID, year *int, month *int) ([]model.CreditCardStatement, error) {
	var items []model.CreditCardStatement
	query := r.db.WithContext(ctx).
		Preload("Wallet", withDeleted).
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

// PayStatement records a payment atomically in one DB transaction:
//  1. lock the statement row (FOR UPDATE) so concurrent payments serialise;
//  2. reject PAID statements and payments <= 0 or above the remaining balance;
//  3. if a source wallet is given it must belong to the user (else ErrNotFound)
//     and is debited with `balance = balance - amount`;
//  4. the card wallet is credited with `balance = balance + amount` (its balance
//     is negative debt) and its statement_balance snapshot set to the remainder;
//  5. the statement's paid_amount/status are updated.
//
// Any error rolls back every step. Returns (nil, nil) if the statement does not
// exist for this user (same convention as GetByID).
func (r *creditCardStatementRepository) PayStatement(ctx context.Context, userID, id uuid.UUID, amount float64, sourceWalletID *uuid.UUID) (*model.CreditCardStatement, error) {
	var stmt model.CreditCardStatement
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", id, userID).
			First(&stmt).Error; err != nil {
			return err
		}
		if stmt.Status == model.StatementStatusPaid {
			return ErrStatementAlreadyPaid
		}
		newPaid, newStatus, err := ApplyStatementPayment(stmt.StatementBalance, stmt.PaidAmount, amount)
		if err != nil {
			return err
		}

		if sourceWalletID != nil && *sourceWalletID != uuid.Nil {
			if *sourceWalletID == stmt.WalletID {
				return fmt.Errorf("source wallet must differ from the card wallet: %w", ErrStatementInvalidPayment)
			}
			res := tx.Model(&model.Wallet{}).
				Where("id = ? AND user_id = ?", *sourceWalletID, userID).
				Update("balance", gorm.Expr("balance - ?", amount))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("source wallet %w", constant.ErrNotFound)
			}
		}

		remaining := roundMoney(stmt.StatementBalance - newPaid)
		res := tx.Model(&model.Wallet{}).
			Where("id = ? AND user_id = ?", stmt.WalletID, userID).
			Updates(map[string]interface{}{
				"balance":           gorm.Expr("balance + ?", amount),
				"statement_balance": remaining,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("card wallet %w", constant.ErrNotFound)
		}

		if err := tx.Model(&stmt).Updates(map[string]interface{}{
			"paid_amount": newPaid,
			"status":      newStatus,
		}).Error; err != nil {
			return err
		}
		stmt.PaidAmount = newPaid
		stmt.Status = newStatus
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &stmt, nil
}

// ApplyStatementPayment validates a payment against a statement and returns the
// new paid amount and status. Amounts are compared at 2-decimal precision.
func ApplyStatementPayment(statementBalance, paidAmount, amount float64) (float64, model.StatementStatus, error) {
	if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, "", ErrStatementInvalidPayment
	}
	remaining := roundMoney(statementBalance - paidAmount)
	if roundMoney(amount) > remaining {
		return 0, "", ErrStatementInvalidPayment
	}
	newPaid := roundMoney(paidAmount + amount)
	if newPaid >= roundMoney(statementBalance) {
		return newPaid, model.StatementStatusPaid, nil
	}
	return newPaid, model.StatementStatusPartiallyPaid, nil
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}
