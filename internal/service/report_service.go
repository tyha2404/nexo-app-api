package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

type ReportService interface {
	GetSummary(ctx context.Context, userID uuid.UUID, startDate, endDate string) (*dto.SummaryReport, error)
	GetCategoryBreakdown(ctx context.Context, userID uuid.UUID, startDate, endDate string) (*dto.CategoryBreakdownReport, error)
	GetMonthlyTrend(ctx context.Context, userID uuid.UUID, months int) (*dto.MonthlyTrendReport, error)
	// GetNetWorthTrend returns month-end net worth for the last `months`
	// calendar months (1–36, default 12) in the app timezone.
	GetNetWorthTrend(ctx context.Context, userID uuid.UUID, months int) (*dto.NetWorthReport, error)
	// GetDailyReport returns per-day income/expense for a month ("YYYY-MM";
	// empty means the current month in the app timezone). An invalid month
	// is constant.ErrInvalidInput.
	GetDailyReport(ctx context.Context, userID uuid.UUID, month string) (*dto.DailyReport, error)
}

// errReportRepositoryUnavailable is returned when the service was built
// without a SQL-capable report repository.
var errReportRepositoryUnavailable = errors.New("report repository is not configured")

type reportService struct {
	reportRepo repository.ReportRepository
}

// NewReportService keeps the constructor used by router/container.go. All
// totals are aggregated in SQL through a ReportRepository derived from the
// DB-backed targetRepo (repository.NewTargetRepository implements it).
func NewReportService(transactionRepo repository.TransactionRepository, targetRepo repository.TargetRepository) ReportService {
	reportRepo, ok := repository.AsReportRepository(targetRepo)
	if !ok {
		reportRepo, _ = repository.AsReportRepository(transactionRepo)
	}
	return &reportService{reportRepo: reportRepo}
}

// NewReportServiceWithRepository builds the service from an explicit
// ReportRepository (preferred for new wiring and tests).
func NewReportServiceWithRepository(reportRepo repository.ReportRepository) ReportService {
	return &reportService{reportRepo: reportRepo}
}

func (s *reportService) GetSummary(ctx context.Context, userID uuid.UUID, startDate, endDate string) (*dto.SummaryReport, error) {
	if s.reportRepo == nil {
		return nil, errReportRepositoryUnavailable
	}
	totals, err := s.reportRepo.GetTransactionTotals(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	return &dto.SummaryReport{
		TotalIncome:     totals.Income,
		TotalExpense:    totals.Expense,
		TotalInvestment: totals.ActiveInvestment,
		RealizedPnL:     totals.RealizedPnL,
	}, nil
}

func (s *reportService) GetCategoryBreakdown(ctx context.Context, userID uuid.UUID, startDate, endDate string) (*dto.CategoryBreakdownReport, error) {
	if s.reportRepo == nil {
		return nil, errReportRepositoryUnavailable
	}
	rows, err := s.reportRepo.GetExpenseByCategory(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	var totalExpense float64
	for _, row := range rows {
		totalExpense += row.Total
	}

	items := make([]dto.CategoryBreakdownItem, 0, len(rows))
	for _, row := range rows {
		percentage := 0.0
		if totalExpense > 0 {
			percentage = (row.Total / totalExpense) * 100.0
		}
		items = append(items, dto.CategoryBreakdownItem{
			CategoryID:   row.CategoryID,
			CategoryName: row.CategoryName,
			TotalAmount:  row.Total,
			Percentage:   percentage,
		})
	}

	return &dto.CategoryBreakdownReport{
		Items:        items,
		TotalExpense: totalExpense,
	}, nil
}

func (s *reportService) GetMonthlyTrend(ctx context.Context, userID uuid.UUID, months int) (*dto.MonthlyTrendReport, error) {
	if s.reportRepo == nil {
		return nil, errReportRepositoryUnavailable
	}
	if months <= 0 || months > 36 {
		months = 12
	}

	// Months are calendar months in the app timezone (Vietnam), so the
	// current month is right even when the server runs in UTC.
	currentMonthStart, nextMonthStart := util.MonthRange(util.Now())
	startMonth := currentMonthStart.AddDate(0, -(months - 1), 0)
	endDate := nextMonthStart.AddDate(0, 0, -1)

	monthly, err := s.reportRepo.GetMonthlyIncomeExpense(ctx, userID, startMonth.Format("2006-01-02"), endDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	byMonth := make(map[string]repository.MonthlyTotal, len(monthly))
	for _, m := range monthly {
		byMonth[m.Month] = m
	}

	// One query for all months' expense targets.
	targets, err := s.reportRepo.ListTargetsInRange(ctx, userID, model.TargetTypeExpense, startMonth, currentMonthStart)
	if err != nil {
		return nil, err
	}
	targetByMonth := make(map[string]float64, len(targets))
	for _, t := range targets {
		targetByMonth[fmt.Sprintf("%04d-%02d", t.Year, t.Month)] = t.TargetAmount
	}

	items := make([]dto.MonthlyTrendItem, 0, months)
	var totalExpenseSum float64
	for i := 0; i < months; i++ {
		cur := startMonth.AddDate(0, i, 0)
		mKey := cur.Format("2006-01")
		totals := byMonth[mKey]
		totalExpenseSum += totals.Expense

		items = append(items, dto.MonthlyTrendItem{
			Month:   mKey,
			Label:   fmt.Sprintf("T%02d/%s", int(cur.Month()), cur.Format("06")),
			Expense: totals.Expense,
			Income:  totals.Income,
			Target:  targetByMonth[mKey],
		})
	}

	return &dto.MonthlyTrendReport{
		AverageExpense: totalExpenseSum / float64(months),
		Items:          items,
	}, nil
}

func (s *reportService) GetNetWorthTrend(ctx context.Context, userID uuid.UUID, months int) (*dto.NetWorthReport, error) {
	if s.reportRepo == nil {
		return nil, errReportRepositoryUnavailable
	}
	if months <= 0 || months > 36 {
		months = 12
	}

	currentMonthStart, nextMonthStart := util.MonthRange(util.Now())
	startMonth := currentMonthStart.AddDate(0, -(months - 1), 0)
	endDate := nextMonthStart.AddDate(0, 0, -1)

	// Everything before the window, as one opening balance.
	before, err := s.reportRepo.GetTransactionTotals(ctx, userID, "", startMonth.AddDate(0, 0, -1).Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	opening := before.Income - before.Expense + before.RealizedPnL

	monthly, err := s.reportRepo.GetMonthlyIncomeExpense(ctx, userID, startMonth.Format("2006-01-02"), endDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	byMonth := make(map[string]repository.MonthlyTotal, len(monthly))
	for _, m := range monthly {
		byMonth[m.Month] = m
	}

	items := make([]dto.NetWorthPoint, 0, months)
	running := opening
	for i := 0; i < months; i++ {
		cur := startMonth.AddDate(0, i, 0)
		key := cur.Format("2006-01")
		m := byMonth[key]
		change := m.Income - m.Expense + m.RealizedPnL
		running += change
		items = append(items, dto.NetWorthPoint{
			Month:       key,
			Label:       fmt.Sprintf("T%02d/%s", int(cur.Month()), cur.Format("06")),
			Income:      m.Income,
			Expense:     m.Expense,
			RealizedPnL: m.RealizedPnL,
			Change:      change,
			NetWorth:    running,
		})
	}

	// A new user's chart starts at their first transaction, not with months of zeros.
	if opening == 0 {
		first := 0
		for first < len(items)-2 && items[first].Income == 0 && items[first].Expense == 0 && items[first].RealizedPnL == 0 {
			first++
		}
		items = items[first:]
	}

	current := opening
	if len(items) > 0 {
		current = items[len(items)-1].NetWorth
	}
	return &dto.NetWorthReport{
		Items:        items,
		Opening:      opening,
		Current:      current,
		PeriodChange: current - opening,
	}, nil
}

func (s *reportService) GetDailyReport(ctx context.Context, userID uuid.UUID, month string) (*dto.DailyReport, error) {
	if s.reportRepo == nil {
		return nil, errReportRepositoryUnavailable
	}
	var start, next time.Time
	if month == "" {
		start, next = util.MonthRange(util.Now())
	} else {
		parsed, err := time.Parse("2006-01", month)
		if err != nil {
			return nil, fmt.Errorf("month must be YYYY-MM: %w", constant.ErrInvalidInput)
		}
		start, next = util.MonthRangeOf(parsed.Year(), parsed.Month())
	}
	end := next.AddDate(0, 0, -1)

	rows, err := s.reportRepo.GetDailyIncomeExpense(ctx, userID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	byDate := make(map[string]repository.DailyTotal, len(rows))
	for _, row := range rows {
		byDate[row.Date] = row
	}

	report := &dto.DailyReport{Month: start.Format("2006-01"), Items: make([]dto.DailyReportItem, 0, end.Day())}
	for d := start; d.Before(next); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		row := byDate[date]
		report.Items = append(report.Items, dto.DailyReportItem{Date: date, Income: row.Income, Expense: row.Expense, Investment: row.Investment, Count: row.Count})
		report.TotalIncome += row.Income
		report.TotalExpense += row.Expense
		report.TotalInvestment += row.Investment
		if row.Expense > report.MaxExpense {
			report.MaxExpense = row.Expense
		}
	}
	return report, nil
}
