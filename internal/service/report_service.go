package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type ReportService interface {
	GetSummary(ctx context.Context, userID uuid.UUID, startDate, endDate string) (*dto.SummaryReport, error)
	GetCategoryBreakdown(ctx context.Context, userID uuid.UUID, startDate, endDate string) (*dto.CategoryBreakdownReport, error)
	GetMonthlyTrend(ctx context.Context, userID uuid.UUID, months int) (*dto.MonthlyTrendReport, error)
}

type reportService struct {
	transactionRepo repository.TransactionRepository
	targetRepo      repository.TargetRepository
}

func NewReportService(transactionRepo repository.TransactionRepository, targetRepo repository.TargetRepository) ReportService {
	return &reportService{
		transactionRepo: transactionRepo,
		targetRepo:      targetRepo,
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

func (s *reportService) GetMonthlyTrend(ctx context.Context, userID uuid.UUID, months int) (*dto.MonthlyTrendReport, error) {
	if months <= 0 || months > 36 {
		months = 12
	}

	now := time.Now()
	// Calculate range: from the 1st of (now - months + 1) to end of current month
	startMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -(months - 1), 0)
	endMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0).Add(-time.Nanosecond)

	startDateStr := startMonth.Format("2006-01-02")
	endDateStr := endMonth.Format("2006-01-02")

	// 1. Fetch expenses in range
	expenseFilters := map[string]interface{}{
		"type":      string(model.TransactionTypeExpense),
		"startDate": startDateStr,
		"endDate":   endDateStr,
	}
	expenses, _, err := s.transactionRepo.ListByUserID(ctx, userID, 100000, 0, expenseFilters)
	if err != nil {
		return nil, err
	}

	// 2. Fetch incomes in range
	incomeFilters := map[string]interface{}{
		"type":      string(model.TransactionTypeIncome),
		"startDate": startDateStr,
		"endDate":   endDateStr,
	}
	incomes, _, err := s.transactionRepo.ListByUserID(ctx, userID, 100000, 0, incomeFilters)
	if err != nil {
		return nil, err
	}

	// Aggregate by month ("YYYY-MM")
	monthlyExpense := make(map[string]float64)
	monthlyIncome := make(map[string]float64)

	for _, exp := range expenses {
		mKey := exp.TransactionDate.Format("2006-01")
		monthlyExpense[mKey] += exp.Amount
	}

	for _, inc := range incomes {
		mKey := inc.TransactionDate.Format("2006-01")
		monthlyIncome[mKey] += inc.Amount
	}

	// Build monthly items chronologically
	var items []dto.MonthlyTrendItem
	var totalExpenseSum float64

	for i := 0; i < months; i++ {
		cur := startMonth.AddDate(0, i, 0)
		mKey := cur.Format("2006-01")
		label := fmt.Sprintf("T%02d/%s", int(cur.Month()), cur.Format("06"))

		exp := monthlyExpense[mKey]
		inc := monthlyIncome[mKey]

		var targetAmount float64
		if s.targetRepo != nil {
			target, err := s.targetRepo.GetTarget(ctx, userID, model.TargetTypeExpense, int(cur.Month()), cur.Year())
			if err == nil && target != nil {
				targetAmount = target.TargetAmount
			}
		}

		totalExpenseSum += exp

		items = append(items, dto.MonthlyTrendItem{
			Month:   mKey,
			Label:   label,
			Expense: exp,
			Income:  inc,
			Target:  targetAmount,
		})
	}

	avgExpense := 0.0
	if months > 0 {
		avgExpense = totalExpenseSum / float64(months)
	}

	return &dto.MonthlyTrendReport{
		AverageExpense: avgExpense,
		Items:          items,
	}, nil
}

