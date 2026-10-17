package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/service"
)

type budgetCreateStub struct {
	repository.BudgetRepository
	created int
}

func (r *budgetCreateStub) Create(_ context.Context, _ *model.Budget) error {
	r.created++
	return nil
}

type presetCreateStub struct {
	repository.PresetRepository
	created int
}

func (r *presetCreateStub) Create(_ context.Context, _ *model.Preset) error {
	r.created++
	return nil
}

func TestBudgetAndPresetRejectForeignCategory(t *testing.T) {
	userID, otherUser, categoryID := uuid.New(), uuid.New(), uuid.New()
	foreign := &categoryStub{category: &model.Category{ID: categoryID, UserID: otherUser}}
	mine := &categoryStub{category: &model.Category{ID: categoryID, UserID: userID}}

	budgets := &budgetCreateStub{}
	_, err := service.NewBudgetService(budgets, foreign).CreateBudget(context.Background(), userID, dto.CreateBudgetRequest{CategoryID: categoryID, Amount: 1, PeriodType: "monthly"})
	require.ErrorIs(t, err, constant.ErrNotFound)
	require.Zero(t, budgets.created)
	_, err = service.NewBudgetService(budgets, mine).CreateBudget(context.Background(), userID, dto.CreateBudgetRequest{CategoryID: categoryID, Amount: 1, PeriodType: "monthly"})
	require.NoError(t, err)
	require.Equal(t, 1, budgets.created)

	presets := &presetCreateStub{}
	_, err = service.NewPresetService(presets, foreign).CreatePreset(context.Background(), userID, dto.CreatePresetRequest{CategoryID: categoryID, Name: "Cafe", Amount: 1, Type: "EXPENSE"})
	require.ErrorIs(t, err, constant.ErrNotFound)
	require.Zero(t, presets.created)
}
