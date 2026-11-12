package repository

import (
	"errors"
	"fmt"
	"math"

	"context"

	"github.com/tyha2404/nexo-app-api/internal/constant"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CreditCardStatementRepository interface {
	Create(ctx context.Context, s *model.CreditCardStatement) error
	GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.CreditCardStatement, error)
	List(ctx context.Context, userID uuid.UUID, walletID *uuid.UUID, year *int, month *int) ([]model.CreditCardStatement, error)
	// Update locks the user's statement row, applies mutate to it and writes
	// back only the editable columns (see statementEditableColumns).
	Update(ctx context.Context, userID, id uuid.UUID, mutate func(s *model.CreditCardStatement) error) (*model.CreditCardStatement, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
	PayStatement(ctx context.Context, userID, id uuid.UUID, amount float64, sourceWalletID *uuid.UUID) (*model.CreditCardStatement, error)
}

var (
	// ErrStatementNotFound: no live statement with that id belongs to the user.
	ErrStatementNotFound = constant.NewKindError(constant.ErrNotFound, "không tìm thấy kỳ sao kê")
	// ErrStatementAlreadyPaid is returned when paying a statement whose status is PAID.
	ErrStatementAlreadyPaid = constant.NewKindError(constant.ErrInvalidInput, "kỳ sao kê đã được thanh toán đủ")
	// ErrStatementInvalidPayment is returned for a non-positive amount or one
	// larger than the remaining statement balance.
	ErrStatementInvalidPayment = constant.NewKindError(constant.ErrInvalidInput, "số tiền thanh toán phải lớn hơn 0 và không vượt số dư còn lại của sao kê")
)

type creditCardStatementRepository struct {
	db *gorm.DB
}

func NewCreditCardStatementRepository(db *gorm.DB) CreditCardStatementRepository {
	return &creditCardStatementRepository{db: db}
}

func (r *creditCardStatementRepository) Create(ctx context.Context, s *model.CreditCardStatement) error {
	return mapStatementUniqueViolation(r.db.WithContext(ctx).Create(s).Error)
}

// mapStatementUniqueViolation turns a unique_violation (23505: a statement for
// this card and period already exists) into constant.ErrAlreadyExists.
func mapStatementUniqueViolation(err error) error {
	if isUniqueViolation(err) {
		return fmt.Errorf("%w: credit card statement for this period", constant.ErrAlreadyExists)
	}
	return err
}

func (r *creditCardStatementRepository) GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.CreditCardStatement, error) {
	var s model.CreditCardStatement
	err := r.db.WithContext(ctx).
		Preload("Wallet", withDeleted).
		Where("id = ? AND user_id = ?", id, userID).
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStatementNotFound
		}
		return nil, err
	}
	return &s, nil
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

// statementEditableColumns are the columns UpdateStatement may change. id,
// user_id, wallet_id and created_at are never written by an edit.
var statementEditableColumns = []string{
	"statement_month", "statement_year", "statement_date", "due_date", "statement_balance",
	"minimum_payment", "previous_balance", "paid_amount", "status", "note", "updated_at",
}

// Update edits the user's statement in one DB transaction: the live row is
// locked (FOR UPDATE, like PayStatement) so mutate sees the committed
// paid_amount and a concurrent payment can no longer be overwritten by a stale
// read-modify-write. Only statementEditableColumns are written, scoped by id
// AND user_id (an explicit Select also writes zero values). Returns
// constant.ErrNotFound when no live statement with that id belongs to the
// user, constant.ErrAlreadyExists on a unique violation, or mutate's error.
func (r *creditCardStatementRepository) Update(ctx context.Context, userID, id uuid.UUID, mutate func(s *model.CreditCardStatement) error) (*model.CreditCardStatement, error) {
	var updated model.CreditCardStatement
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stmt model.CreditCardStatement
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", id, userID).
			First(&stmt).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStatementNotFound
			}
			return err
		}
		if err := mutate(&stmt); err != nil {
			return err
		}
		stmt.ID, stmt.UserID = id, userID // an edit never moves the row
		result := tx.Model(&stmt).Omit(clause.Associations).
			Where("id = ? AND user_id = ?", id, userID).
			Select(statementEditableColumns).
			Updates(&stmt)
		if result.Error != nil {
			return mapStatementUniqueViolation(result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrStatementNotFound
		}
		return tx.Preload("Wallet", withDeleted).
			Where("id = ? AND user_id = ?", id, userID).
			First(&updated).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// Delete soft-deletes the user's statement. constant.ErrNotFound when no live
// statement with that id belongs to the user.
func (r *creditCardStatementRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.CreditCardStatement{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return constant.ErrNotFound
	}
	return nil
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
// Any error rolls back every step. Returns ErrStatementNotFound if the
// statement does not exist for this user.
func (r *creditCardStatementRepository) PayStatement(ctx context.Context, userID, id uuid.UUID, amount float64, sourceWalletID *uuid.UUID) (*model.CreditCardStatement, error) {
	var stmt model.CreditCardStatement
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", id, userID).
			First(&stmt).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStatementNotFound
			}
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
			found, err := applyWalletEffect(tx, sourceWalletID, userID, -amount)
			if err != nil {
				return err
			}
			if !found {
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
	return newPaid, model.StatementStatusFor(roundMoney(statementBalance), newPaid), nil
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}
