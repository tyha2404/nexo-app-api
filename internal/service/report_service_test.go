package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

type fakeReportRepo struct {
	totals       repository.TransactionTotals
	categories   []repository.CategoryTotal
	monthly      []repository.MonthlyTotal
	targets      []model.MonthlyTarget
	targetCalls  int
	monthlyStart string
	monthlyEnd   string
	targetFrom   time.Time
	targetTo     time.Time
	daily        []repository.DailyTotal
	dailyStart   string
	dailyEnd     string
	totalsStart  string
	totalsEnd    string
}

func (f *fakeReportRepo) GetTransactionTotals(ctx context.Context, userID uuid.UUID, startDate, endDate string) (repository.TransactionTotals, error) {
	f.totalsStart, f.totalsEnd = startDate, endDate
	return f.totals, nil
}
func (f *fakeReportRepo) GetExpenseByCategory(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]repository.CategoryTotal, error) {
	return f.categories, nil
}
func (f *fakeReportRepo) GetMonthlyIncomeExpense(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]repository.MonthlyTotal, error) {
	f.monthlyStart, f.monthlyEnd = startDate, endDate
	return f.monthly, nil
}
func (f *fakeReportRepo) GetDailyIncomeExpense(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]repository.DailyTotal, error) {
	f.dailyStart, f.dailyEnd = startDate, endDate
	return f.daily, nil
}
func (f *fakeReportRepo) ListTargetsInRange(ctx context.Context, userID uuid.UUID, targetType model.TargetType, from, to time.Time) ([]model.MonthlyTarget, error) {
	f.targetCalls++
	f.targetFrom, f.targetTo = from, to
	return f.targets, nil
}

func TestReportService_GetSummary(t *testing.T) {
	repo := &fakeReportRepo{totals: repository.TransactionTotals{Income: 30_000_000, Expense: 12_500_000, ActiveInvestment: 5_000_000, RealizedPnL: 250_000}}
	svc := NewReportServiceWithRepository(repo)

	res, err := svc.GetSummary(context.Background(), uuid.New(), "2026-10-01", "2026-10-31")
	require.NoError(t, err)
	assert.Equal(t, 30_000_000.0, res.TotalIncome)
	assert.Equal(t, 12_500_000.0, res.TotalExpense)
	assert.Equal(t, 5_000_000.0, res.TotalInvestment)
	assert.Equal(t, 250_000.0, res.RealizedPnL)
}

func TestReportService_GetCategoryBreakdown(t *testing.T) {
	food, rent := uuid.New(), uuid.New()
	repo := &fakeReportRepo{categories: []repository.CategoryTotal{
		{CategoryID: rent, CategoryName: "Tiền nhà", Total: 7_500_000},
		{CategoryID: food, CategoryName: "Ăn uống", Total: 2_500_000},
	}}
	res, err := NewReportServiceWithRepository(repo).GetCategoryBreakdown(context.Background(), uuid.New(), "", "")
	require.NoError(t, err)
	assert.Equal(t, 10_000_000.0, res.TotalExpense)
	require.Len(t, res.Items, 2)
	assert.Equal(t, rent, res.Items[0].CategoryID)
	assert.InDelta(t, 75.0, res.Items[0].Percentage, 0.001)
	assert.InDelta(t, 25.0, res.Items[1].Percentage, 0.001)
}

func TestReportService_GetMonthlyTrend_SingleTargetQueryAndAppTimezone(t *testing.T) {
	now := util.Now()
	curKey := now.Format("2006-01")
	prev, _ := util.MonthRange(now)
	prevKey := prev.AddDate(0, -1, 0).Format("2006-01")

	repo := &fakeReportRepo{
		monthly: []repository.MonthlyTotal{
			{Month: prevKey, Income: 20_000_000, Expense: 9_000_000},
			{Month: curKey, Income: 25_000_000, Expense: 3_000_000, ActiveInvestment: 1_200_000},
		},
		targets: []model.MonthlyTarget{{Month: int(now.Month()), Year: now.Year(), TargetAmount: 10_000_000}},
	}

	res, err := NewReportServiceWithRepository(repo).GetMonthlyTrend(context.Background(), uuid.New(), 3)
	require.NoError(t, err)
	require.Len(t, res.Items, 3)
	assert.Equal(t, 1, repo.targetCalls, "targets must be fetched in one query, not one per month")

	last := res.Items[2]
	assert.Equal(t, curKey, last.Month)
	assert.Equal(t, 3_000_000.0, last.Expense)
	assert.Equal(t, 10_000_000.0, last.Target)
	assert.Equal(t, 1_200_000.0, last.Investment, "active investment per month (used by the AI get_monthly_trend tool)")
	assert.Equal(t, prevKey, res.Items[1].Month)
	assert.Equal(t, 0.0, res.Items[0].Expense)
	assert.InDelta(t, 4_000_000.0, res.AverageExpense, 0.001)

	// Range covers whole calendar months in the app timezone.
	assert.Equal(t, "01", repo.monthlyStart[8:])
	monthStart, next := util.MonthRange(now)
	assert.Equal(t, next.AddDate(0, 0, -1).Format("2006-01-02"), repo.monthlyEnd)
	assert.Equal(t, monthStart, repo.targetTo)
}

func TestReportService_GetDailyReportFillsEveryDay(t *testing.T) {
	repo := &fakeReportRepo{daily: []repository.DailyTotal{
		{Date: "2026-02-03", Income: 10_000_000, Count: 1},
		{Date: "2026-02-14", Expense: 1_200_000, Count: 3},
		{Date: "2026-02-28", Expense: 300_000, Income: 50_000, Investment: 2_000_000, Count: 3},
	}}
	res, err := NewReportServiceWithRepository(repo).GetDailyReport(context.Background(), uuid.New(), "2026-02")
	require.NoError(t, err)

	assert.Equal(t, "2026-02-01", repo.dailyStart)
	assert.Equal(t, "2026-02-28", repo.dailyEnd)
	assert.Equal(t, "2026-02", res.Month)
	require.Len(t, res.Items, 28)
	assert.Equal(t, "2026-02-01", res.Items[0].Date)
	assert.Zero(t, res.Items[0].Count)
	assert.Equal(t, 1_200_000.0, res.Items[13].Expense)
	assert.EqualValues(t, 3, res.Items[13].Count)
	assert.Equal(t, 10_050_000.0, res.TotalIncome)
	assert.Equal(t, 1_500_000.0, res.TotalExpense)
	assert.Equal(t, 1_200_000.0, res.MaxExpense)
	assert.Equal(t, 2_000_000.0, res.Items[27].Investment)
	assert.Equal(t, 2_000_000.0, res.TotalInvestment)
}

func TestReportService_GetDailyReportRejectsBadMonth(t *testing.T) {
	svc := NewReportServiceWithRepository(&fakeReportRepo{})
	for _, month := range []string{"2026-13", "11/2026", "2026-1-01"} {
		_, err := svc.GetDailyReport(context.Background(), uuid.New(), month)
		require.ErrorIs(t, err, constant.ErrInvalidInput, month)
	}
}

func TestReportService_GetDailyReportDefaultsToCurrentMonth(t *testing.T) {
	repo := &fakeReportRepo{}
	res, err := NewReportServiceWithRepository(repo).GetDailyReport(context.Background(), uuid.New(), "")
	require.NoError(t, err)
	start, _ := util.MonthRange(util.Now())
	assert.Equal(t, start.Format("2006-01"), res.Month)
	assert.Equal(t, start.Format("2006-01-02"), repo.dailyStart)
}

func TestReportService_GetNetWorthTrend_CumulatesFromOpening(t *testing.T) {
	monthStart, next := util.MonthRange(util.Now())
	key := func(offset int) string { return monthStart.AddDate(0, offset, 0).Format("2006-01") }

	repo := &fakeReportRepo{
		// Before the window: 30M in, 21M out, 1M realized → opening 10M.
		totals: repository.TransactionTotals{Income: 30_000_000, Expense: 21_000_000, RealizedPnL: 1_000_000, ActiveInvestment: 5_000_000},
		monthly: []repository.MonthlyTotal{
			{Month: key(-1), Income: 20_000_000, Expense: 9_000_000, RealizedPnL: 1_000_000},
			{Month: key(0), Expense: 3_000_000},
		},
	}
	res, err := NewReportServiceWithRepository(repo).GetNetWorthTrend(context.Background(), uuid.New(), 3)
	require.NoError(t, err)

	require.Len(t, res.Items, 3)
	assert.Equal(t, key(-2), res.Items[0].Month)
	assert.Equal(t, 10_000_000.0, res.Items[0].NetWorth, "a quiet month keeps the opening balance")
	assert.Equal(t, 12_000_000.0, res.Items[1].Change)
	assert.Equal(t, 22_000_000.0, res.Items[1].NetWorth)
	assert.Equal(t, -3_000_000.0, res.Items[2].Change)
	assert.Equal(t, 19_000_000.0, res.Items[2].NetWorth)
	assert.Equal(t, 10_000_000.0, res.Opening)
	assert.Equal(t, 19_000_000.0, res.Current)
	assert.Equal(t, 9_000_000.0, res.PeriodChange)
	assert.Equal(t, fmt.Sprintf("T%02d/%s", int(monthStart.Month()), monthStart.Format("06")), res.Items[2].Label)

	// Opening = everything up to the day before the window; months cover the window exactly.
	windowStart := monthStart.AddDate(0, -2, 0)
	assert.Equal(t, "", repo.totalsStart)
	assert.Equal(t, windowStart.AddDate(0, 0, -1).Format("2006-01-02"), repo.totalsEnd)
	assert.Equal(t, windowStart.Format("2006-01-02"), repo.monthlyStart)
	assert.Equal(t, next.AddDate(0, 0, -1).Format("2006-01-02"), repo.monthlyEnd)
}

func TestReportService_GetNetWorthTrend_NewUserStartsAtFirstActivity(t *testing.T) {
	monthStart, _ := util.MonthRange(util.Now())
	key := func(offset int) string { return monthStart.AddDate(0, offset, 0).Format("2006-01") }

	repo := &fakeReportRepo{monthly: []repository.MonthlyTotal{
		{Month: key(-3), Income: 5_000_000},
		{Month: key(-1), Expense: 1_000_000},
	}}
	res, err := NewReportServiceWithRepository(repo).GetNetWorthTrend(context.Background(), uuid.New(), 12)
	require.NoError(t, err)
	require.Len(t, res.Items, 4, "months before the first transaction are dropped")
	assert.Equal(t, key(-3), res.Items[0].Month)
	assert.Equal(t, 4_000_000.0, res.Current)

	// Only this month has data: still two points, so the chart can draw a line.
	repo = &fakeReportRepo{monthly: []repository.MonthlyTotal{{Month: key(0), Income: 2_000_000}}}
	res, err = NewReportServiceWithRepository(repo).GetNetWorthTrend(context.Background(), uuid.New(), 12)
	require.NoError(t, err)
	require.Len(t, res.Items, 2)
	assert.Equal(t, 0.0, res.Items[0].NetWorth)
	assert.Equal(t, 2_000_000.0, res.Items[1].NetWorth)
}

func TestReportService_GetNetWorthTrend_ClampsMonths(t *testing.T) {
	repo := &fakeReportRepo{totals: repository.TransactionTotals{Income: 1}}
	for _, months := range []int{0, -4, 37} {
		res, err := NewReportServiceWithRepository(repo).GetNetWorthTrend(context.Background(), uuid.New(), months)
		require.NoError(t, err)
		assert.Len(t, res.Items, 12, "months=%d", months)
	}
}
