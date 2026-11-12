package repository

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TransactionRepository interface {
	Create(ctx context.Context, transaction *model.Transaction) error
	// GetByIDForUser returns the user's transaction; "transaction not found"
	// (constant.ErrNotFound) when it is missing or belongs to someone else.
	GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Transaction, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Transaction, int64, error)
	GetSummaryByUserID(ctx context.Context, userID uuid.UUID, filters map[string]interface{}) (TransactionSummary, error)
	SearchByUserID(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Transaction, int64, error)
	Update(ctx context.Context, transaction *model.Transaction) error
	// Delete soft-deletes the user's transaction and reverses its wallet
	// effect; "transaction not found" (constant.ErrNotFound) when no live
	// transaction with that id belongs to the user.
	Delete(ctx context.Context, userID, id uuid.UUID) error
	// Restore undoes the soft delete of one of the user's transactions and
	// re-applies its wallet effect. Returns ErrNotFound when no deleted
	// transaction with that id belongs to the user.
	Restore(ctx context.Context, userID, id uuid.UUID) error
	// ListTags returns every tag on the user's live transactions with usage
	// totals, most recently used first.
	ListTags(ctx context.Context, userID uuid.UUID) ([]TagUsage, error)
}

// TagUsage is one tag with how often and how much it was used.
type TagUsage struct {
	Name         string
	Count        int64
	TotalExpense float64
	TotalIncome  float64
	LastUsed     time.Time
}

// withTagFilter keeps rows carrying the "tag" filter (exact match). It uses
// `tags @> ARRAY[tag]`, which the GIN index on transactions.tags serves.
func withTagFilter(q *gorm.DB, column string, filters map[string]interface{}) *gorm.DB {
	if tag, ok := filters["tag"].(string); ok && strings.TrimSpace(tag) != "" {
		return q.Where(column+" @> ?", model.StringArray{strings.TrimSpace(tag)})
	}
	return q
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
	if !hasWallet(walletID) || (delta == 0 && !required) {
		return nil
	}
	found, err := applyWalletEffect(tx, walletID, userID, delta)
	if err != nil {
		return err
	}
	if !found && required {
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

func (r *transactionRepository) GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Transaction, error) {
	var transaction model.Transaction
	err := r.db.WithContext(ctx).
		Preload("Category", withDeleted).
		Where("id = ? AND user_id = ?", id, userID).
		First(&transaction).Error
	if err != nil {
		return nil, notFoundAs("transaction", err)
	}
	return &transaction, nil
}

func (r *transactionRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Transaction, int64, error) {
	var transactions []model.Transaction
	var total int64

	// Filter before counting/pagination so all matching rows are visible.
	query := applyTransactionFilters(r.db.WithContext(ctx).Model(&model.Transaction{}).Where("user_id = ?", userID), filters)
	if st, ok := filters["status"].(string); ok {
		query = withStatusFilter(query, st)
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

// TransactionSummary aggregates the transactions matching a list's filters.
type TransactionSummary struct {
	SumAmount float64
	// SumAmountForAverage excludes categories flagged exclude_from_average_daily.
	SumAmountForAverage float64
	Total               int64
	HoldingAmount       float64
	HoldingCount        int64
	RealizedPnL         float64
}

func (r *transactionRepository) GetSummaryByUserID(ctx context.Context, userID uuid.UUID, filters map[string]interface{}) (TransactionSummary, error) {
	var sum TransactionSummary
	buildBaseQuery := func() *gorm.DB {
		return applyTransactionFilters(r.db.WithContext(ctx).Model(&model.Transaction{}).Where("transactions.user_id = ?", userID), filters)
	}

	st, filterStatus := filters["status"].(string)

	// 1. Calculate filtered total and sumAmount
	listQuery := withStatusFilter(buildBaseQuery(), st)

	if err := listQuery.Count(&sum.Total).Error; err != nil {
		return TransactionSummary{}, err
	}

	row := listQuery.Select("COALESCE(SUM(amount), 0)").Row()
	if err := row.Scan(&sum.SumAmount); err != nil {
		return TransactionSummary{}, err
	}

	// 1b. Calculate sum excluding categories flagged as "exclude from average daily spending".
	// Raw Joins are not soft-delete filtered; intentionally no c.deleted_at filter so
	// spending in a since-deleted category still counts (transactions are kept).
	avgQuery := buildBaseQuery().Joins("JOIN categories c ON c.id = transactions.category_id AND c.user_id = transactions.user_id AND c.exclude_from_average_daily = false")
	avgRow := avgQuery.Select("COALESCE(SUM(transactions.amount), 0)").Row()
	if err := avgRow.Scan(&sum.SumAmountForAverage); err != nil {
		return TransactionSummary{}, err
	}

	// 2. Calculate Holding metrics:
	if filterStatus && st != "" && st != string(model.InvestmentStatusHolding) {
		sum.HoldingAmount = 0
		sum.HoldingCount = 0
	} else {
		holdingQuery := buildBaseQuery().Where("status IS NULL OR status = ?", model.InvestmentStatusHolding)
		if err := holdingQuery.Count(&sum.HoldingCount).Error; err != nil {
			return TransactionSummary{}, err
		}
		holdingRow := holdingQuery.Select("COALESCE(SUM(amount), 0)").Row()
		if err := holdingRow.Scan(&sum.HoldingAmount); err != nil {
			return TransactionSummary{}, err
		}
	}

	// 3. Calculate Realized PnL:
	// Sum realized_pnl for all transactions (including SOLD, MATURED, CANCELLED, or any status with realized_pnl)
	pnlRow := buildBaseQuery().Where("(status IS NOT NULL AND status != ?) OR realized_pnl != 0", model.InvestmentStatusHolding).Select("COALESCE(SUM(realized_pnl), 0)").Row()
	if err := pnlRow.Scan(&sum.RealizedPnL); err != nil {
		return TransactionSummary{}, err
	}

	return sum, nil
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

// SearchByUserID is a read-only search used by the AI financial tools: the
// shared filters of applyTransactionFilters (type "ALL" means any type) plus
// wallet and amount range. All conditions are ANDed and always scoped to userID.
func (r *transactionRepository) SearchByUserID(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Transaction, int64, error) {
	var transactions []model.Transaction
	var total int64

	if t, _ := filters["type"].(string); t == "ALL" {
		filters = maps.Clone(filters)
		delete(filters, "type")
	}
	query := applyTransactionFilters(r.db.WithContext(ctx).Model(&model.Transaction{}).Where("user_id = ?", userID), filters)

	if w, ok := filters["walletId"].(string); ok && w != "" {
		if walletID, err := uuid.Parse(w); err == nil {
			query = query.Where("wallet_id = ?", walletID)
		}
	}

	if minA, ok := filters["minAmount"].(float64); ok && minA > 0 {
		query = query.Where("amount >= ?", minA)
	}

	if maxA, ok := filters["maxAmount"].(float64); ok && maxA > 0 {
		query = query.Where("amount <= ?", maxA)
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
func (r *transactionRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.Transaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", id, userID).First(&old).Error; err != nil {
			return notFoundAs("transaction", err)
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

// Restore clears deleted_at and puts back the wallet effect that Delete
// reversed. The effect is re-applied only when the row had it applied
// (balance_applied) and the wallet is still live; otherwise balance_applied is
// cleared so a later edit or delete never reverses an effect that isn't there.
func (r *transactionRepository) Restore(ctx context.Context, userID, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.Transaction
		err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ? AND deleted_at IS NOT NULL", id, userID).
			First(&old).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("transaction %w", constant.ErrNotFound)
		}
		if err != nil {
			return err
		}
		applied := false
		if old.BalanceApplied {
			if applied, err = applyWalletEffect(tx, old.WalletID, old.UserID, old.WalletBalanceEffect()); err != nil {
				return err
			}
		}
		return tx.Unscoped().Model(&model.Transaction{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{"deleted_at": nil, "balance_applied": applied}).Error
	})
}

func (r *transactionRepository) ListTags(ctx context.Context, userID uuid.UUID) ([]TagUsage, error) {
	var rows []struct {
		Name         string
		Count        int64
		TotalExpense float64
		TotalIncome  float64
		LastUsed     time.Time
	}
	err := r.db.WithContext(ctx).
		Table("transactions, unnest(transactions.tags) AS tag").
		Where("transactions.user_id = ? AND transactions.deleted_at IS NULL", userID).
		Select("tag AS name, COUNT(*) AS count, " +
			sqlSumExpense + " AS total_expense, " +
			sqlSumIncome + " AS total_income, " +
			"MAX(transactions.transaction_date) AS last_used").
		Group("tag").
		Order("last_used DESC, count DESC, name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]TagUsage, 0, len(rows))
	for _, row := range rows {
		out = append(out, TagUsage(row))
	}
	return out, nil
}

// applyTransactionFilters adds the filters shared by list, summary and search:
// keyword (case-insensitive substring of the description, LIKE wildcards
// escaped), tag, type, categoryId and the inclusive startDate/endDate
// (YYYY-MM-DD). Malformed ids and dates are ignored. Columns are qualified so
// the query may join other tables.
func applyTransactionFilters(q *gorm.DB, filters map[string]interface{}) *gorm.DB {
	if keyword, ok := filters["keyword"].(string); ok && strings.TrimSpace(keyword) != "" {
		q = q.Where("transactions.description ILIKE ?", "%"+escapeLike(strings.TrimSpace(keyword))+"%")
	}
	q = withTagFilter(q, "transactions.tags", filters)
	if t, ok := filters["type"].(string); ok && t != "" {
		q = q.Where("transactions.type = ?", t)
	}
	if c, ok := filters["categoryId"].(string); ok && c != "" {
		if catID, err := uuid.Parse(c); err == nil {
			q = q.Where("transactions.category_id = ?", catID)
		}
	}
	if s, ok := filters["startDate"].(string); ok && s != "" {
		if t, err := util.ParseDate(s); err == nil {
			q = q.Where("transactions.transaction_date >= ?", t)
		}
	}
	if e, ok := filters["endDate"].(string); ok && e != "" {
		if t, err := util.ParseDate(e); err == nil {
			q = q.Where("transactions.transaction_date <= ?", t)
		}
	}
	return q
}

// withStatusFilter keeps rows with the given investment status; HOLDING also
// matches rows without a status. An empty status adds no condition.
func withStatusFilter(q *gorm.DB, status string) *gorm.DB {
	switch status {
	case "":
		return q
	case string(model.InvestmentStatusHolding):
		return q.Where("status IS NULL OR status = ?", status)
	default:
		return q.Where("status = ?", status)
	}
}

// notFoundAs turns gorm.ErrRecordNotFound into "<entity> not found" wrapping
// constant.ErrNotFound; other errors are returned unchanged.
func notFoundAs(entity string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%s %w", entity, constant.ErrNotFound)
	}
	return err
}

// deleteForUser soft-deletes the row (a model pointer) with that id when it belongs
// to userID; "<entity> not found" (constant.ErrNotFound) when nothing matched.
func deleteForUser(db *gorm.DB, row interface{}, entity string, userID, id uuid.UUID) error {
	res := db.Where("id = ? AND user_id = ?", id, userID).Delete(row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("%s %w", entity, constant.ErrNotFound)
	}
	return nil
}

// Escape SQL LIKE wildcards so the search field matches literal text.
func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

// withDeleted preloads an association including soft-deleted rows, so e.g. a
// transaction still shows the name of a category that was deleted later.
func withDeleted(db *gorm.DB) *gorm.DB { return db.Unscoped() }
