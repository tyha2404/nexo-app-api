package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type TransactionService interface {
	CreateTransaction(ctx context.Context, userID uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error)
	GetTransaction(ctx context.Context, userID, id uuid.UUID) (*dto.TransactionResponse, error)
	ListTransactions(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.TransactionResponse, int64, error)
	UpdateTransaction(ctx context.Context, userID, id uuid.UUID, req dto.UpdateTransactionRequest) (*dto.TransactionResponse, error)
	DeleteTransaction(ctx context.Context, userID, id uuid.UUID) error
}

type transactionService struct {
	transactionRepo repository.TransactionRepository
	categoryRepo    repository.CategoryRepo
}

func NewTransactionService(transactionRepo repository.TransactionRepository, categoryRepo repository.CategoryRepo) TransactionService {
	return &transactionService{
		transactionRepo: transactionRepo,
		categoryRepo:    categoryRepo,
	}
}

func (s *transactionService) CreateTransaction(ctx context.Context, userID uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
	// Verify category exists and belongs to user
	category, err := s.categoryRepo.GetByID(ctx, req.CategoryID)
	if err != nil {
		return nil, errors.New("category not found")
	}
	if category.UserID != userID {
		return nil, errors.New("unauthorized access to category")
	}

	transaction := &model.Transaction{
		UserID:          userID,
		CategoryID:      req.CategoryID,
		Amount:          req.Amount,
		Type:            model.TransactionType(req.Type),
		Description:     req.Description,
		TransactionDate: req.TransactionDate,
	}

	if err := s.transactionRepo.Create(ctx, transaction); err != nil {
		return nil, err
	}

	// Reload to get associations if needed (though we already have category)
	transaction.Category = category

	return s.toResponse(transaction), nil
}

func (s *transactionService) GetTransaction(ctx context.Context, userID, id uuid.UUID) (*dto.TransactionResponse, error) {
	transaction, err := s.transactionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if transaction.UserID != userID {
		return nil, errors.New("transaction not found")
	}

	return s.toResponse(transaction), nil
}

func (s *transactionService) ListTransactions(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.TransactionResponse, int64, error) {
	offset := (page - 1) * limit
	transactions, total, err := s.transactionRepo.ListByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	var responses []dto.TransactionResponse
	for _, t := range transactions {
		responses = append(responses, *s.toResponse(&t))
	}

	return responses, total, nil
}

func (s *transactionService) UpdateTransaction(ctx context.Context, userID, id uuid.UUID, req dto.UpdateTransactionRequest) (*dto.TransactionResponse, error) {
	transaction, err := s.transactionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if transaction.UserID != userID {
		return nil, errors.New("transaction not found")
	}

	if req.CategoryID != nil {
		category, err := s.categoryRepo.GetByID(ctx, *req.CategoryID)
		if err != nil {
			return nil, errors.New("category not found")
		}
		if category.UserID != userID {
			return nil, errors.New("unauthorized access to category")
		}
		transaction.CategoryID = *req.CategoryID
		transaction.Category = category
	}

	if req.Amount != nil {
		transaction.Amount = *req.Amount
	}
	if req.Type != nil {
		transaction.Type = model.TransactionType(*req.Type)
	}
	if req.Description != nil {
		transaction.Description = req.Description
	}
	if req.TransactionDate != nil {
		transaction.TransactionDate = *req.TransactionDate
	}

	if err := s.transactionRepo.Update(ctx, transaction); err != nil {
		return nil, err
	}

	return s.toResponse(transaction), nil
}

func (s *transactionService) DeleteTransaction(ctx context.Context, userID, id uuid.UUID) error {
	transaction, err := s.transactionRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if transaction.UserID != userID {
		return errors.New("transaction not found")
	}

	return s.transactionRepo.Delete(ctx, id)
}

func (s *transactionService) toResponse(t *model.Transaction) *dto.TransactionResponse {
	var categoryName string
	if t.Category != nil {
		categoryName = t.Category.Name
	}

	var deletedAt *string
	if t.DeletedAt != nil {
		formatted := (*t.DeletedAt).Format(time.RFC3339)
		deletedAt = &formatted
	}

	return &dto.TransactionResponse{
		ID:              t.ID,
		UserID:          t.UserID,
		CategoryID:      t.CategoryID,
		CategoryName:    categoryName,
		Amount:          t.Amount,
		Type:            string(t.Type),
		Description:     t.Description,
		TransactionDate: t.TransactionDate.Format("2006-01-02"),
		CreatedAt:       t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       t.UpdatedAt.Format(time.RFC3339),
		DeletedAt:       deletedAt,
	}
}
