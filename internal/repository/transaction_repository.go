package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TransactionRepository interface {
	Create(ctx context.Context, transaction *model.Transaction) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Transaction, int64, error)
	GetSummaryByUserID(ctx context.Context, userID uuid.UUID, filters map[string]interface{}) (sumAmount float64, sumAmountForAverage float64, total int64, holdingAmount float64, holdingCount int64, realizedPnL float64, err error)
	SearchByUserID(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Transaction, int64, error)
	Update(ctx context.Context, transaction *model.Transaction) error
	Delete(ctx context.Context, id uuid.UUID) error
	CountTransactionsInRange(ctx context.Context, userID uuid.UUID, start, end time.Time) (int64, error)
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

// Create inserts the transaction and, when it has a wallet, moves that wallet's
// balance by transaction.WalletBalanceEffect() in the same DB transaction.
// The wallet must belong to transaction.UserID, otherwise ErrNotFound.
func (r *transactionRepository) Create(ctx context.Context, transaction *model.Transaction) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		transaction.BalanceApplied = hasWallet(transaction.WalletID)
		if err := tx.Omit(clause.Associations).Create(transaction).Error; err != nil {
			return err
		}
		return adjustWalletBalance(tx, transaction.WalletID, transaction.UserID, transaction.WalletBalanceEffect(), true)
	})
}

func hasWallet(id *uuid.UUID) bool { return id != nil && *id != uuid.Nil }

// walletReversal returns the delta that undoes a stored transaction's wallet
// effect, and false when there is nothing to undo (no wallet, or a legacy row
// whose effect was never applied: balance_applied = false).
func walletReversal(t *model.Transaction) (float64, bool) {
	if !t.BalanceApplied || !hasWallet(t.WalletID) {
		return 0, false
	}
	return -t.WalletBalanceEffect(), true
}

// applyWalletEffect applies delta to the wallet and reports whether the wallet
// (live, owned by userID) was found. It always issues the UPDATE (even for a
// zero delta) so the result reliably tells whether the effect is "applied".
func applyWalletEffect(tx *gorm.DB, walletID *uuid.UUID, userID uuid.UUID, delta float64) (bool, error) {
	if !hasWallet(walletID) {
		return false, nil
	}
	res := tx.Model(&model.Wallet{}).
		Where("id = ? AND user_id = ?", *walletID, userID).
		Update("balance", gorm.Expr("balance + ?", delta))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// adjustWalletBalance atomically applies `balance = balance + delta` to a wallet
// owned by userID. A nil wallet is a no-op (transactions without wallet keep the
// legacy "no balance effect" behaviour). When required is true a missing (or
// soft-deleted / foreign) wallet is an ErrNotFound; when false it is tolerated,
// which is used to reverse effects on wallets that were deleted afterwards.
func adjustWalletBalance(tx *gorm.DB, walletID *uuid.UUID, userID uuid.UUID, delta float64, required bool) error {
	if walletID == nil || *walletID == uuid.Nil {
		return nil
	}
	if delta == 0 && !required {
		return nil
	}
	res := tx.Model(&model.Wallet{}).
		Where("id = ? AND user_id = ?", *walletID, userID).
		Update("balance", gorm.Expr("balance + ?", delta))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 && required {
		return fmt.Errorf("wallet %w", constant.ErrNotFound)
	}
	return nil
}

func sameWallet(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (r *transactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	var transaction model.Transaction
	err := r.db.WithContext(ctx).
		Preload("Category", withDeleted).
		First(&transaction, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (r *transactionRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Transaction, int64, error) {
	var transactions []model.Transaction
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Transaction{}).Where("user_id = ?", userID)

	// Search before counting/pagination so all matching rows are visible.
	if keyword, ok := filters["keyword"].(string); ok && strings.TrimSpace(keyword) != "" {
		query = query.Where("description ILIKE ?", "%"+escapeLike(strings.TrimSpace(keyword))+"%")
	}

	// Filter by Type
	if t, ok := filters["type"].(string); ok && t != "" {
		query = query.Where("type = ?", t)
	}

	// Filter by Status
	if st, ok := filters["status"].(string); ok && st != "" {
		if st == string(model.InvestmentStatusHolding) {
			query = query.Where("status IS NULL OR status = ?", st)
		} else {
			query = query.Where("status = ?", st)
		}
	}

	// Filter by CategoryID
	if c, ok := filters["categoryId"].(string); ok && c != "" {
		if catID, err := uuid.Parse(c); err == nil {
			query = query.Where("category_id = ?", catID)
		}
	}

	// Filter by StartDate
	if s, ok := filters["startDate"].(string); ok && s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			query = query.Where("transaction_date >= ?", t)
		}
	}

	// Filter by EndDate
	if e, ok := filters["endDate"].(string); ok && e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			query = query.Where("transaction_date <= ?", t)
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Preload("Category", withDeleted).
		Order("transaction_date desc, created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error

	return transactions, total, err
}

func (r *transactionRepository) GetSummaryByUserID(ctx context.Context, userID uuid.UUID, filters map[string]interface{}) (sumAmount float64, sumAmountForAverage float64, total int64, holdingAmount float64, holdingCount int64, realizedPnL float64, err error) {
	buildBaseQuery := func() *gorm.DB {
		q := r.db.WithContext(ctx).Model(&model.Transaction{}).Where("transactions.user_id = ?", userID)
		if keyword, ok := filters["keyword"].(string); ok && strings.TrimSpace(keyword) != "" {
			q = q.Where("transactions.description ILIKE ?", "%"+escapeLike(strings.TrimSpace(keyword))+"%")
		}
		if t, ok := filters["type"].(string); ok && t != "" {
			q = q.Where("transactions.type = ?", t)
		}
		if c, ok := filters["categoryId"].(string); ok && c != "" {
			if catID, err := uuid.Parse(c); err == nil {
				q = q.Where("transactions.category_id = ?", catID)
			}
		}
		if s, ok := filters["startDate"].(string); ok && s != "" {
			if t, err := time.Parse("2006-01-02", s); err == nil {
				q = q.Where("transactions.transaction_date >= ?", t)
			}
		}
		if e, ok := filters["endDate"].(string); ok && e != "" {
			if t, err := time.Parse("2006-01-02", e); err == nil {
				q = q.Where("transactions.transaction_date <= ?", t)
			}
		}
		return q
	}

	st, filterStatus := filters["status"].(string)

	// 1. Calculate filtered total and sumAmount
	listQuery := buildBaseQuery()
	if filterStatus && st != "" {
		if st == string(model.InvestmentStatusHolding) {
			listQuery = listQuery.Where("status IS NULL OR status = ?", st)
		} else {
			listQuery = listQuery.Where("status = ?", st)
		}
	}

	if err := listQuery.Count(&total).Error; err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	row := listQuery.Select("COALESCE(SUM(amount), 0)").Row()
	if err := row.Scan(&sumAmount); err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	// 1b. Calculate sum excluding categories flagged as "exclude from average daily spending".
	// Raw Joins are not soft-delete filtered; intentionally no c.deleted_at filter so
	// spending in a since-deleted category still counts (transactions are kept).
	avgQuery := buildBaseQuery().Joins("JOIN categories c ON c.id = transactions.category_id AND c.user_id = transactions.user_id AND c.exclude_from_average_daily = false")
	avgRow := avgQuery.Select("COALESCE(SUM(transactions.amount), 0)").Row()
	if err := avgRow.Scan(&sumAmountForAverage); err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	// 2. Calculate Holding metrics:
	if filterStatus && st != "" && st != string(model.InvestmentStatusHolding) {
		holdingAmount = 0
		holdingCount = 0
	} else {
		holdingQuery := buildBaseQuery().Where("status IS NULL OR status = ?", model.InvestmentStatusHolding)
		if err := holdingQuery.Count(&holdingCount).Error; err != nil {
			return 0, 0, 0, 0, 0, 0, err
		}
		holdingRow := holdingQuery.Select("COALESCE(SUM(amount), 0)").Row()
		if err := holdingRow.Scan(&holdingAmount); err != nil {
			return 0, 0, 0, 0, 0, 0, err
		}
	}

	// 3. Calculate Realized PnL:
	// Sum realized_pnl for all transactions (including SOLD, MATURED, CANCELLED, or any status with realized_pnl)
	pnlRow := buildBaseQuery().Where("(status IS NOT NULL AND status != ?) OR realized_pnl != 0", model.InvestmentStatusHolding).Select("COALESCE(SUM(realized_pnl), 0)").Row()
	if err := pnlRow.Scan(&realizedPnL); err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	return sumAmount, sumAmountForAverage, total, holdingAmount, holdingCount, realizedPnL, nil
}

// Update saves the transaction and re-balances wallets in one DB transaction:
// the stored row is locked, its old wallet effect is reversed (only when
// balance_applied) and the new effect applied, setting balance_applied
// (handles wallet, amount, type and investment status changes).
func (r *transactionRepository) Update(ctx context.Context, transaction *model.Transaction) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.Transaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", transaction.ID, transaction.UserID).
			First(&old).Error; err != nil {
			return err
		}
		// Reverse the old effect only if it was really applied (legacy rows are
		// false). A wallet deleted meanwhile is tolerated.
		if delta, ok := walletReversal(&old); ok {
			if err := adjustWalletBalance(tx, old.WalletID, old.UserID, delta, false); err != nil {
				return err
			}
		}
		transaction.BalanceApplied = false
		if hasWallet(transaction.WalletID) {
			applied, err := applyWalletEffect(tx, transaction.WalletID, transaction.UserID, transaction.WalletBalanceEffect())
			if err != nil {
				return err
			}
			// A newly chosen wallet must exist and be the user's; an unchanged
			// wallet that was deleted since is tolerated (effect not applied).
			if !applied && !sameWallet(old.WalletID, transaction.WalletID) {
				return fmt.Errorf("wallet %w", constant.ErrNotFound)
			}
			transaction.BalanceApplied = applied
		}
		return tx.Omit(clause.Associations).Save(transaction).Error
	})
}

// SearchByUserID is a read-only search with extended filters (wallet, amount range,
// keyword in description) used by the AI financial tools. All conditions are ANDed
// and always scoped to userID.
func (r *transactionRepository) SearchByUserID(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Transaction, int64, error) {
	var transactions []model.Transaction
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Transaction{}).Where("user_id = ?", userID)

	if t, ok := filters["type"].(string); ok && t != "" && t != "ALL" {
		query = query.Where("type = ?", t)
	}

	if c, ok := filters["categoryId"].(string); ok && c != "" {
		if catID, err := uuid.Parse(c); err == nil {
			query = query.Where("category_id = ?", catID)
		}
	}

	if w, ok := filters["walletId"].(string); ok && w != "" {
		if walletID, err := uuid.Parse(w); err == nil {
			query = query.Where("wallet_id = ?", walletID)
		}
	}

	if s, ok := filters["startDate"].(string); ok && s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			query = query.Where("transaction_date >= ?", t)
		}
	}

	if e, ok := filters["endDate"].(string); ok && e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			query = query.Where("transaction_date <= ?", t)
		}
	}

	if minA, ok := filters["minAmount"].(float64); ok && minA > 0 {
		query = query.Where("amount >= ?", minA)
	}

	if maxA, ok := filters["maxAmount"].(float64); ok && maxA > 0 {
		query = query.Where("amount <= ?", maxA)
	}

	if kw, ok := filters["keyword"].(string); ok && kw != "" {
		pattern := "%" + strings.ToLower(kw) + "%"
		query = query.Where("LOWER(description) LIKE ?", pattern)
	}

	sortBy := "transaction_date desc, created_at desc"
	switch sort, _ := filters["sortBy"].(string); sort {
	case "amount_asc":
		sortBy = "amount asc"
	case "amount_desc":
		sortBy = "amount desc"
	case "date_asc":
		sortBy = "transaction_date asc, created_at asc"
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Preload("Category", withDeleted).
		Order(sortBy).
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error

	return transactions, total, err
}

// Delete (soft) removes the transaction and reverses its wallet effect atomically.
func (r *transactionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.Transaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&old).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.Transaction{}, "id = ?", id).Error; err != nil {
			return err
		}
		delta, ok := walletReversal(&old)
		if !ok {
			return nil // legacy row: its effect was never applied, nothing to reverse
		}
		return adjustWalletBalance(tx, old.WalletID, old.UserID, delta, false)
	})
}

func (r *transactionRepository) CountTransactionsInRange(ctx context.Context, userID uuid.UUID, start, end time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Transaction{}).
		Where("user_id = ? AND transaction_date >= ? AND transaction_date <= ?", userID, start, end).
		Count(&count).Error
	return count, err
}

// Escape SQL LIKE wildcards so the search field matches literal text.
func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

// withDeleted preloads an association including soft-deleted rows, so e.g. a
// transaction still shows the name of a category that was deleted later.
func withDeleted(db *gorm.DB) *gorm.DB { return db.Unscoped() }
