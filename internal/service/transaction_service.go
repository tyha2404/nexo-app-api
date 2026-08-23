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
	ListTransactions(ctx context.Context, userID uuid.UUID, page, limit int, filters map[string]interface{}) ([]dto.TransactionResponse, int64, *dto.TransactionSummaryDTO, error)
	UpdateTransaction(ctx context.Context, userID, id uuid.UUID, req dto.UpdateTransactionRequest) (*dto.TransactionResponse, error)
	DeleteTransaction(ctx context.Context, userID, id uuid.UUID) error
}

type transactionService struct {
	transactionRepo repository.TransactionRepository
	categoryRepo    repository.CategoryRepo
	budgetRepo      repository.BudgetRepository
	alertRepo       repository.AlertRepository
}

func NewTransactionService(
	transactionRepo repository.TransactionRepository,
	categoryRepo repository.CategoryRepo,
	budgetRepo repository.BudgetRepository,
	alertRepo repository.AlertRepository,
) TransactionService {
	return &transactionService{
		transactionRepo: transactionRepo,
		categoryRepo:    categoryRepo,
		budgetRepo:      budgetRepo,
		alertRepo:       alertRepo,
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

	if string(category.Type) != string(req.Type) {
		return nil, errors.New("transaction type does not match category type")
	}

	transaction := &model.Transaction{
		UserID:          userID,
		CategoryID:      req.CategoryID,
		WalletID:        req.WalletID,
		Amount:          req.Amount,
		Type:            model.TransactionType(req.Type),
		Description:     req.Description,
		TransactionDate: req.TransactionDate.Time,
	}

	if err := s.transactionRepo.Create(ctx, transaction); err != nil {
		return nil, err
	}

	transaction.Category = category

	// Auto check budget alert if transaction is Expense
	if transaction.Type == model.TransactionTypeExpense {
		s.checkBudgetLimit(ctx, userID, req.CategoryID, req.TransactionDate.Time)
	}

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

func (s *transactionService) ListTransactions(ctx context.Context, userID uuid.UUID, page, limit int, filters map[string]interface{}) ([]dto.TransactionResponse, int64, *dto.TransactionSummaryDTO, error) {
	offset := (page - 1) * limit
	transactions, total, err := s.transactionRepo.ListByUserID(ctx, userID, limit, offset, filters)
	if err != nil {
		return nil, 0, nil, err
	}

	sumAmount, sumAmountForAverage, count, holdingAmount, holdingCount, realizedPnL, err := s.transactionRepo.GetSummaryByUserID(ctx, userID, filters)
	if err != nil {
		return nil, 0, nil, err
	}

	var responses []dto.TransactionResponse
	for _, t := range transactions {
		responses = append(responses, *s.toResponse(&t))
	}

	summary := &dto.TransactionSummaryDTO{
		SumAmount:           sumAmount,
		SumAmountForAverage: sumAmountForAverage,
		Total:               count,
		HoldingAmount:       holdingAmount,
		HoldingCount:        holdingCount,
		RealizedPnL:         realizedPnL,
	}

	return responses, total, summary, nil
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

		expectedType := transaction.Type
		if req.Type != nil {
			expectedType = model.TransactionType(*req.Type)
		}
		if string(category.Type) != string(expectedType) {
			return nil, errors.New("transaction type does not match category type")
		}

		transaction.CategoryID = *req.CategoryID
		transaction.Category = category
	} else if req.Type != nil {
		category, err := s.categoryRepo.GetByID(ctx, transaction.CategoryID)
		if err == nil {
			if string(category.Type) != *req.Type {
				return nil, errors.New("transaction type does not match category type")
			}
		}
	}

	if req.WalletID != nil {
		transaction.WalletID = req.WalletID
	}
	if req.Amount != nil {
		transaction.Amount = *req.Amount
	}
	if req.Type != nil {
		transaction.Type = model.TransactionType(*req.Type)
	}
	if req.Status != nil {
		st := model.InvestmentStatus(*req.Status)
		transaction.Status = &st
	}
	if req.RealizedPnL != nil {
		transaction.RealizedPnL = req.RealizedPnL
	}
	if req.Description != nil {
		transaction.Description = req.Description
	}
	if req.TransactionDate != nil {
		transaction.TransactionDate = req.TransactionDate.Time
	}

	if err := s.transactionRepo.Update(ctx, transaction); err != nil {
		return nil, err
	}

	// Trigger budget limit verification if transaction is an Expense
	if transaction.Type == model.TransactionTypeExpense {
		s.checkBudgetLimit(ctx, userID, transaction.CategoryID, transaction.TransactionDate)
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

	var statusStr *string
	if t.Status != nil {
		st := string(*t.Status)
		statusStr = &st
	}

	return &dto.TransactionResponse{
		ID:              t.ID,
		UserID:          t.UserID,
		CategoryID:      t.CategoryID,
		CategoryName:    categoryName,
		WalletID:        t.WalletID,
		Amount:          t.Amount,
		Type:            string(t.Type),
		Status:          statusStr,
		RealizedPnL:     t.RealizedPnL,
		Description:     t.Description,
		TransactionDate: t.TransactionDate.Format("2006-01-02"),
		CreatedAt:       t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       t.UpdatedAt.Format(time.RFC3339),
		DeletedAt:       deletedAt,
	}
}

func (s *transactionService) checkBudgetLimit(ctx context.Context, userID, categoryID uuid.UUID, transactionDate time.Time) {
	budget, err := s.budgetRepo.GetByCategoryAndPeriod(ctx, userID, categoryID, transactionDate)
	if err != nil || budget == nil {
		return // No budget configured for this category/time
	}

	var start, end time.Time
	if budget.PeriodType == "monthly" {
		start = time.Date(transactionDate.Year(), transactionDate.Month(), 1, 0, 0, 0, 0, transactionDate.Location())
		end = start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	} else if budget.PeriodType == "yearly" {
		start = time.Date(transactionDate.Year(), 1, 1, 0, 0, 0, 0, transactionDate.Location())
		end = start.AddDate(1, 0, 0).Add(-time.Nanosecond)
	} else {
		start = budget.PeriodStart
		end = start.AddDate(0, 1, 0)
	}

	filters := map[string]interface{}{
		"type":       string(model.TransactionTypeExpense),
		"categoryId": categoryID.String(),
		"startDate":  start.Format("2006-01-02"),
		"endDate":    end.Format("2006-01-02"),
	}

	expenses, _, err := s.transactionRepo.ListByUserID(ctx, userID, 1000, 0, filters)
	if err != nil {
		return
	}

	var totalExpense float64
	for _, e := range expenses {
		totalExpense += e.Amount
	}

	var alertType string
	var message string
	ratio := totalExpense / budget.Amount
	if ratio >= 1.0 {
		alertType = "over_limit"
		message = "Bạn đã vượt hạn mức chi tiêu của danh mục này!"
	} else if ratio >= 0.8 {
		alertType = "approaching_limit"
		message = "Chi tiêu của bạn đã đạt trên 80% ngân sách danh mục này."
	}

	if alertType != "" {
		alert := &model.Alert{
			UserID:    userID,
			BudgetID:  budget.ID,
			AlertType: alertType,
			Message:   message,
		}
		_ = s.alertRepo.Create(ctx, alert)
	}
}
