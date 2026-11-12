package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

type activeBudgetRepo struct {
	repository.BudgetRepository
	rows   []model.Budget
	gotAt  time.Time
	gotUID uuid.UUID
}

func (r *activeBudgetRepo) ListActiveByUserID(_ context.Context, userID uuid.UUID, at time.Time) ([]model.Budget, error) {
	r.gotUID, r.gotAt = userID, at
	return r.rows, nil
}

func TestListActiveBudgets_OnePerCategory(t *testing.T) {
	food, coffee := uuid.New(), uuid.New()
	nov := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	// Ordered as the repository returns them: newest period first.
	repo := &activeBudgetRepo{rows: []model.Budget{
		{ID: uuid.New(), CategoryID: food, Amount: 3_000_000, PeriodType: model.BudgetPeriodMonthly, PeriodStart: nov},
		{ID: uuid.New(), CategoryID: coffee, Amount: 500_000, PeriodType: model.BudgetPeriodMonthly, PeriodStart: nov},
		{ID: uuid.New(), CategoryID: food, Amount: 30_000_000, PeriodType: model.BudgetPeriodYearly, PeriodStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	}}
	svc := NewBudgetService(repo, nil)
	uid := uuid.New()
	at := time.Date(2026, 11, 11, 9, 0, 0, 0, util.AppLocation())

	got, err := svc.ListActiveBudgets(context.Background(), uid, at)
	require.NoError(t, err)
	require.Len(t, got, 2, "one budget per category")
	assert.Equal(t, 3_000_000.0, got[0].Amount, "the most recent period wins")
	assert.Equal(t, coffee, got[1].CategoryID)
	assert.Equal(t, uid, repo.gotUID)
	assert.Equal(t, at, repo.gotAt)
}

func TestBudgetReferenceDate(t *testing.T) {
	now := util.Now()
	assert.WithinDuration(t, now, budgetReferenceDate(nil), time.Minute, "default: today")
	assert.WithinDuration(t, now, budgetReferenceDate(map[string]interface{}{"month": now.Format("2006-01")}), time.Minute, "current month: today")
	assert.WithinDuration(t, now, budgetReferenceDate(map[string]interface{}{"month": "tháng 10"}), time.Minute, "invalid month: today")

	got := budgetReferenceDate(map[string]interface{}{"month": "2025-02"})
	assert.Equal(t, "2025-02-28", got.Format("2006-01-02"), "another month: its last day")
	assert.Equal(t, util.AppLocation(), got.Location())
}

func TestToolBudgetStatus_UsesRequestedMonth(t *testing.T) {
	e := newToolEnv(t)
	e.report.breakdown = &dto.CategoryBreakdownReport{}

	e.call(t, "get_budget_status", map[string]interface{}{"month": "2025-10"})
	require.Len(t, e.budget.activeAt, 1)
	assert.Equal(t, "2025-10-31", e.budget.activeAt[0].Format("2006-01-02"))

	e.call(t, "get_budget_status", map[string]interface{}{})
	require.Len(t, e.budget.activeAt, 2)
	assert.Equal(t, util.Today(), e.budget.activeAt[1].Format("2006-01-02"), "default: the current month")

	e.call(t, "list_categories", nil)
	require.Len(t, e.budget.activeAt, 3)
	assert.Equal(t, util.Today(), e.budget.activeAt[2].Format("2006-01-02"))
	requireOnlyUser(t, e.userID, e.budget.users)
}
