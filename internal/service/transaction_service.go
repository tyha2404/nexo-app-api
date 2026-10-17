package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
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
	walletRepo      repository.WalletRepository
	pushService     PushService
}

// TransactionServiceOption configures optional collaborators.
type TransactionServiceOption func(*transactionService)

// WithBudgetAlertPush sends a Web Push notification whenever a new budget
// alert is inserted (deduplicated alerts are not re-notified).
func WithBudgetAlertPush(push PushService) TransactionServiceOption {
	return func(s *transactionService) { s.pushService = push }
}

func NewTransactionService(
	transactionRepo repository.TransactionRepository,
	categoryRepo repository.CategoryRepo,
	budgetRepo repository.BudgetRepository,
	alertRepo repository.AlertRepository,
	walletRepo repository.WalletRepository,
	opts ...TransactionServiceOption,
) TransactionService {
	s := &transactionService{
		transactionRepo: transactionRepo,
		categoryRepo:    categoryRepo,
		budgetRepo:      budgetRepo,
		alertRepo:       alertRepo,
		walletRepo:      walletRepo,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ensureWalletOwned verifies the wallet exists and belongs to the user. The
// repository re-checks ownership inside the DB transaction that moves the balance.
func (s *transactionService) ensureWalletOwned(ctx context.Context, userID uuid.UUID, walletID *uuid.UUID) error {
	if walletID == nil || *walletID == uuid.Nil {
		return nil
	}
	wallet, err := s.walletRepo.GetByID(ctx, *walletID, userID)
	if err != nil {
		return err
	}
	if wallet == nil {
		return fmt.Errorf("wallet %w", constant.ErrNotFound)
	}
	return nil
}

func (s *transactionService) CreateTransaction(ctx context.Context, userID uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
	// Verify category exists and belongs to user
	category, err := s.categoryRepo.GetByIDForUser(ctx, userID, req.CategoryID)
	if err != nil {
		return nil, categoryLookupError(err)
	}

	if string(category.Type) != string(req.Type) {
		return nil, fmt.Errorf("transaction type does not match category type: %w", constant.ErrInvalidInput)
	}

	if err := s.ensureWalletOwned(ctx, userID, req.WalletID); err != nil {
		return nil, err
	}

	transaction := &model.Transaction{
		UserID:          userID,
		CategoryID:      req.CategoryID,
		WalletID:        req.WalletID,
		Amount:          req.Amount,
		Type:            model.TransactionType(req.Type),
		Description:     req.Description,
		ReceiptURL:      req.ReceiptURL,
		TransactionDate: req.TransactionDate.Time,
	}

	if transaction.Type == model.TransactionTypeInvestment {
		status := model.InvestmentStatusHolding
		if req.Status != nil {
			status = model.InvestmentStatus(*req.Status)
		}
		transaction.Status = &status
		transaction.RealizedPnL = req.RealizedPnL
		if status == model.InvestmentStatusHolding {
			zero := 0.0
			transaction.RealizedPnL = &zero
		}
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
		return nil, fmt.Errorf("transaction %w", constant.ErrNotFound)
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
		return nil, fmt.Errorf("transaction %w", constant.ErrNotFound)
	}

	if req.CategoryID != nil {
		category, err := s.categoryRepo.GetByIDForUser(ctx, userID, *req.CategoryID)
		if err != nil {
			return nil, categoryLookupError(err)
		}

		expectedType := transaction.Type
		if req.Type != nil {
			expectedType = model.TransactionType(*req.Type)
		}
		if string(category.Type) != string(expectedType) {
			return nil, fmt.Errorf("transaction type does not match category type: %w", constant.ErrInvalidInput)
		}

		transaction.CategoryID = *req.CategoryID
		transaction.Category = category
	} else if req.Type != nil {
		category, err := s.categoryRepo.GetByIDForUser(ctx, userID, transaction.CategoryID)
		if err == nil {
			if string(category.Type) != *req.Type {
				return nil, fmt.Errorf("transaction type does not match category type: %w", constant.ErrInvalidInput)
			}
		}
	}

	if req.WalletID != nil {
		if transaction.WalletID == nil || *transaction.WalletID != *req.WalletID {
			if err := s.ensureWalletOwned(ctx, userID, req.WalletID); err != nil {
				return nil, err
			}
		}
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
	if req.ReceiptURL != nil {
		transaction.ReceiptURL = req.ReceiptURL
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
		return fmt.Errorf("transaction %w", constant.ErrNotFound)
	}

	return s.transactionRepo.Delete(ctx, id)
}

func (s *transactionService) toResponse(t *model.Transaction) *dto.TransactionResponse {
	var categoryName string
	if t.Category != nil {
		categoryName = t.Category.Name
	}

	var deletedAt *string
	if t.DeletedAt.Valid {
		formatted := t.DeletedAt.Time.Format(time.RFC3339)
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
		ReceiptURL:      t.ReceiptURL,
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
	if budget.Amount <= 0 {
		return
	}
	start, end = budgetSpendWindow(budget, transactionDate)

	filters := map[string]interface{}{
		"type":       string(model.TransactionTypeExpense),
		"categoryId": categoryID.String(),
		"startDate":  start.Format("2006-01-02"),
		"endDate":    end.Format("2006-01-02"),
	}

	totalExpense, _, _, _, _, _, err := s.transactionRepo.GetSummaryByUserID(ctx, userID, filters)
	if err != nil {
		return
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
		created, err := s.alertRepo.Create(ctx, alert)
		if err == nil && created {
			s.notifyBudgetAlert(ctx, userID, budget, ratio)
		}
	}
}

// BudgetAlertPayload builds the push notification for a new budget alert.
func BudgetAlertPayload(categoryName string, ratio float64) dto.PushPayload {
	if categoryName == "" {
		categoryName = "Danh mục"
	}
	percent := int(math.Floor(ratio * 100))
	body := fmt.Sprintf("%s đã dùng %d%% ngân sách.", categoryName, percent)
	if ratio >= 1.0 {
		body = fmt.Sprintf("%s đã dùng %d%% ngân sách — vượt hạn mức!", categoryName, percent)
	}
	return dto.PushPayload{Title: "Cảnh báo ngân sách", Body: body, URL: "/#/planning/categories", Tag: "budget-alert"}
}

// notifyBudgetAlert pushes in the background; it never blocks or fails the
// transaction request.
func (s *transactionService) notifyBudgetAlert(ctx context.Context, userID uuid.UUID, budget *model.Budget, ratio float64) {
	if s.pushService == nil || !s.pushService.Enabled() {
		return
	}
	categoryName := ""
	if budget.Category.Name != "" {
		categoryName = budget.Category.Name
	} else if s.categoryRepo != nil {
		if cat, err := s.categoryRepo.GetByIDForUser(ctx, userID, budget.CategoryID); err == nil && cat != nil {
			categoryName = cat.Name
		}
	}
	s.pushService.NotifyUser(userID, BudgetAlertPayload(categoryName, ratio))
}

// categoryLookupError maps a failed user-scoped category lookup: a category that
// does not exist or belongs to another user is a 404 ("category not found"),
// anything else (DB failure) is returned as is.
func categoryLookupError(err error) error {
	if errors.Is(err, constant.ErrNotFound) {
		return fmt.Errorf("category %w", constant.ErrNotFound)
	}
	return err
}

// budgetSpendWindow returns the inclusive [start, end] dates of the budget period
// containing transactionDate. Month/year boundaries use the app timezone
// (util.AppLocation), never the server's or the parsed value's location.
func budgetSpendWindow(budget *model.Budget, transactionDate time.Time) (time.Time, time.Time) {
	switch budget.PeriodType {
	case "monthly":
		start, next := util.MonthRange(transactionDate)
		return start, next.Add(-time.Nanosecond)
	case "yearly":
		y := transactionDate.In(util.AppLocation()).Year()
		start := time.Date(y, 1, 1, 0, 0, 0, 0, util.AppLocation())
		return start, start.AddDate(1, 0, 0).Add(-time.Nanosecond)
	default:
		start := budget.PeriodStart
		return start, start.AddDate(0, 1, 0)
	}
}
