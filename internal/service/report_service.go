package service

import (
	"context"
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

type reportService struct {
	reportRepo repository.ReportRepository
}

// NewReportServiceWithRepository builds the service on a ReportRepository;
// all totals are aggregated in SQL.
func NewReportServiceWithRepository(reportRepo repository.ReportRepository) ReportService {
	return &reportService{reportRepo: reportRepo}
}

func (s *reportService) GetSummary(ctx context.Context, userID uuid.UUID, startDate, endDate string) (*dto.SummaryReport, error) {
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

// Trend windows: 1 to maxTrendMonths calendar months, defaultTrendMonths
// when the request is out of range.
const (
	defaultTrendMonths = 12
	maxTrendMonths     = 36
	monthKeyLayout     = "2006-01"
)

// monthWindow clamps months and returns the window of the last `months`
// calendar months in the app timezone (so the current month is right even
// when the server runs in UTC): the first day of the current month, the
// first day of the window and its last day (end of the current month).
func monthWindow(months int) (n int, currentMonthStart, startMonth, endDate time.Time) {
	if months <= 0 || months > maxTrendMonths {
		months = defaultTrendMonths
	}
	currentMonthStart, nextMonthStart := util.MonthRange(util.Now())
	startMonth = currentMonthStart.AddDate(0, -(months - 1), 0)
	return months, currentMonthStart, startMonth, nextMonthStart.AddDate(0, 0, -1)
}

// monthLabel is the chart label of a month, e.g. "T03/26".
func monthLabel(month time.Time) string {
	return fmt.Sprintf("T%02d/%s", int(month.Month()), month.Format("06"))
}

func (s *reportService) GetMonthlyTrend(ctx context.Context, userID uuid.UUID, months int) (*dto.MonthlyTrendReport, error) {
	months, currentMonthStart, startMonth, endDate := monthWindow(months)

	monthly, err := s.reportRepo.GetMonthlyIncomeExpense(ctx, userID, util.DateString(startMonth), util.DateString(endDate))
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
		mKey := cur.Format(monthKeyLayout)
		totals := byMonth[mKey]
		totalExpenseSum += totals.Expense

		items = append(items, dto.MonthlyTrendItem{
			Month:   mKey,
			Label:   monthLabel(cur),
			Expense: totals.Expense,
			Income:  totals.Income,
			Target:  targetByMonth[mKey],

			Investment: totals.ActiveInvestment,
		})
	}

	return &dto.MonthlyTrendReport{
		AverageExpense: totalExpenseSum / float64(months),
		Items:          items,
	}, nil
}

func (s *reportService) GetNetWorthTrend(ctx context.Context, userID uuid.UUID, months int) (*dto.NetWorthReport, error) {
	months, _, startMonth, endDate := monthWindow(months)

	// Everything before the window, as one opening balance.
	before, err := s.reportRepo.GetTransactionTotals(ctx, userID, "", util.DateString(startMonth.AddDate(0, 0, -1)))
	if err != nil {
		return nil, err
	}
	opening := before.Income - before.Expense + before.RealizedPnL

	monthly, err := s.reportRepo.GetMonthlyIncomeExpense(ctx, userID, util.DateString(startMonth), util.DateString(endDate))
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
		key := cur.Format(monthKeyLayout)
		m := byMonth[key]
		change := m.Income - m.Expense + m.RealizedPnL
		running += change
		items = append(items, dto.NetWorthPoint{
			Month:       key,
			Label:       monthLabel(cur),
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
	var start, next time.Time
	if month == "" {
		start, next = util.MonthRange(util.Now())
	} else {
		parsed, err := time.Parse(monthKeyLayout, month)
		if err != nil {
			return nil, fmt.Errorf("month must be YYYY-MM: %w", constant.ErrInvalidInput)
		}
		start, next = util.MonthRangeOf(parsed.Year(), parsed.Month())
	}
	end := next.AddDate(0, 0, -1)

	rows, err := s.reportRepo.GetDailyIncomeExpense(ctx, userID, start.Format(time.DateOnly), end.Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	byDate := make(map[string]repository.DailyTotal, len(rows))
	for _, row := range rows {
		byDate[row.Date] = row
	}

	report := &dto.DailyReport{Month: start.Format("2006-01"), Items: make([]dto.DailyReportItem, 0, end.Day())}
	for d := start; d.Before(next); d = d.AddDate(0, 0, 1) {
		date := d.Format(time.DateOnly)
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
