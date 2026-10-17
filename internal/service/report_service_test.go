package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
}

func (f *fakeReportRepo) GetTransactionTotals(ctx context.Context, userID uuid.UUID, startDate, endDate string) (repository.TransactionTotals, error) {
	return f.totals, nil
}
func (f *fakeReportRepo) GetExpenseByCategory(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]repository.CategoryTotal, error) {
	return f.categories, nil
}
func (f *fakeReportRepo) GetMonthlyIncomeExpense(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]repository.MonthlyTotal, error) {
	f.monthlyStart, f.monthlyEnd = startDate, endDate
	return f.monthly, nil
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
			{Month: curKey, Income: 25_000_000, Expense: 3_000_000},
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
	assert.Equal(t, prevKey, res.Items[1].Month)
	assert.Equal(t, 0.0, res.Items[0].Expense)
	assert.InDelta(t, 4_000_000.0, res.AverageExpense, 0.001)

	// Range covers whole calendar months in the app timezone.
	assert.Equal(t, "01", repo.monthlyStart[8:])
	monthStart, next := util.MonthRange(now)
	assert.Equal(t, next.AddDate(0, 0, -1).Format("2006-01-02"), repo.monthlyEnd)
	assert.Equal(t, monthStart, repo.targetTo)
}

func TestReportService_DerivesSQLRepoFromTargetRepo(t *testing.T) {
	// The concrete DB-backed target repository doubles as a ReportRepository
	// so container.go wiring does not need to change.
	_, ok := repository.AsReportRepository(repository.NewTargetRepository(nil))
	assert.True(t, ok)

	svc := NewReportService(nil, nil)
	_, err := svc.GetSummary(context.Background(), uuid.New(), "", "")
	assert.ErrorIs(t, err, errReportRepositoryUnavailable)
}
