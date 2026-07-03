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

func (r *transactionRepository) Update(ctx context.Context, transaction *model.Transaction) error {
	return r.db.WithContext(ctx).Save(transaction).Error
}

func (r *transactionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Transaction{}, id).Error
}
