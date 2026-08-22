package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type TransactionRepository interface {
	Create(ctx context.Context, transaction *model.Transaction) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Transaction, int64, error)
GetSummaryByUserID(ctx context.Context, userID uuid.UUID, filters map[string]interface{}) (sumAmount float64, sumAmountForAverage float64, total int64, holdingAmount float64, holdingCount int64, realizedPnL float64, err error)
	Update(ctx context.Context, transaction *model.Transaction) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(ctx context.Context, transaction *model.Transaction) error {
	return r.db.WithContext(ctx).Create(transaction).Error
}

func (r *transactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	var transaction model.Transaction
	err := r.db.WithContext(ctx).
		Preload("Category").
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
		Preload("Category").
		Order("transaction_date desc, created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error

	return transactions, total, err
}

func (r *transactionRepository) GetSummaryByUserID(ctx context.Context, userID uuid.UUID, filters map[string]interface{}) (sumAmount float64, sumAmountForAverage float64, total int64, holdingAmount float64, holdingCount int64, realizedPnL float64, err error) {
	buildBaseQuery := func() *gorm.DB {
		q := r.db.WithContext(ctx).Model(&model.Transaction{}).Where("transactions.user_id = ?", userID)
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

	// 1b. Calculate sum excluding categories flagged as "exclude from average daily spending"
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

func (r *transactionRepository) Update(ctx context.Context, transaction *model.Transaction) error {
	return r.db.WithContext(ctx).Save(transaction).Error
}

func (r *transactionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Transaction{}, id).Error
}
