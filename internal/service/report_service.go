package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type ReportService interface {
	GetSummary(ctx context.Context, userID uuid.UUID, startDate, endDate string) (*dto.SummaryReport, error)
	GetCategoryBreakdown(ctx context.Context, userID uuid.UUID, startDate, endDate string) (*dto.CategoryBreakdownReport, error)
}

type reportService struct {
	transactionRepo repository.TransactionRepository
}

func NewReportService(transactionRepo repository.TransactionRepository) ReportService {
	return &reportService{
		transactionRepo: transactionRepo,
	}
}

func (s *reportService) GetSummary(ctx context.Context, userID uuid.UUID, startDate, endDate string) (*dto.SummaryReport, error) {
	// 1. Fetch Income transactions
	incomeFilters := map[string]interface{}{
		"type":      string(model.TransactionTypeIncome),
		"startDate": startDate,
		"endDate":   endDate,
	}
	incomes, _, err := s.transactionRepo.ListByUserID(ctx, userID, 10000, 0, incomeFilters)
	if err != nil {
		return nil, err
	}

	var totalIncome float64
	for _, inc := range incomes {
		totalIncome += inc.Amount
	}

	// 2. Fetch Expense transactions
	expenseFilters := map[string]interface{}{
		"type":      string(model.TransactionTypeExpense),
		"startDate": startDate,
		"endDate":   endDate,
	}
	expenses, _, err := s.transactionRepo.ListByUserID(ctx, userID, 10000, 0, expenseFilters)
	if err != nil {
		return nil, err
	}

	var totalExpense float64
	for _, exp := range expenses {
		totalExpense += exp.Amount
	}

	// 3. Fetch Investment transactions
	investmentFilters := map[string]interface{}{
		"type":      string(model.TransactionTypeInvestment),
		"startDate": startDate,
		"endDate":   endDate,
	}
	investments, _, err := s.transactionRepo.ListByUserID(ctx, userID, 10000, 0, investmentFilters)
	if err != nil {
		return nil, err
	}

	var totalInvestment float64
	for _, inv := range investments {
		// Only count HOLDING (or empty/legacy) as currently active invested money
		if inv.Status == nil || *inv.Status == model.InvestmentStatusHolding {
			totalInvestment += inv.Amount
		}
	}

	return &dto.SummaryReport{
		TotalIncome:     totalIncome,
		TotalExpense:    totalExpense,
		TotalInvestment: totalInvestment,
		NetBalance:      totalIncome - totalExpense - totalInvestment,
	}, nil
}

func (s *reportService) GetCategoryBreakdown(ctx context.Context, userID uuid.UUID, startDate, endDate string) (*dto.CategoryBreakdownReport, error) {
	filters := map[string]interface{}{
		"type":      string(model.TransactionTypeExpense),
		"startDate": startDate,
		"endDate":   endDate,
	}
	expenses, _, err := s.transactionRepo.ListByUserID(ctx, userID, 10000, 0, filters)
	if err != nil {
		return nil, err
	}

	var totalExpense float64
	categorySum := make(map[uuid.UUID]float64)
	categoryNames := make(map[uuid.UUID]string)

	for _, exp := range expenses {
		totalExpense += exp.Amount
		categorySum[exp.CategoryID] += exp.Amount
		if exp.Category != nil {
			categoryNames[exp.CategoryID] = exp.Category.Name
		}
	}

	var items []dto.CategoryBreakdownItem
	for catID, sum := range categorySum {
		percentage := 0.0
		if totalExpense > 0 {
			percentage = (sum / totalExpense) * 100.0
		}
		items = append(items, dto.CategoryBreakdownItem{
			CategoryID:   catID,
			CategoryName: categoryNames[catID],
			TotalAmount:  sum,
			Percentage:   percentage,
		})
	}

	return &dto.CategoryBreakdownReport{
		Items:        items,
		TotalExpense: totalExpense,
	}, nil
}
