package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
)

type MockTargetRepo struct {
	mock.Mock
}

func (m *MockTargetRepo) UpsertTarget(ctx context.Context, target *model.MonthlyTarget) error {
	args := m.Called(ctx, target)
	return args.Error(0)
}

func (m *MockTargetRepo) GetTarget(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error) {
	args := m.Called(ctx, userID, targetType, month, year)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.MonthlyTarget), args.Error(1)
}

func (m *MockTargetRepo) GetMonthlyTotalByCategoryType(ctx context.Context, userID uuid.UUID, catType model.CategoryType, startDate, endDate time.Time) (float64, error) {
	args := m.Called(ctx, userID, catType, startDate, endDate)
	return args.Get(0).(float64), args.Error(1)
}

func TestTargetService_GetSummary_NormalBudget(t *testing.T) {
	mockRepo := new(MockTargetRepo)
	svc := service.NewTargetService(mockRepo)

	userID := uuid.New()
	month := 8
	year := 2026

	// Mock Expense Target: 10,000,000
	mockRepo.On("GetTarget", mock.Anything, userID, model.TargetType("EXPENSE"), month, year).
		Return(&model.MonthlyTarget{TargetAmount: 10000000}, nil)

	// Mock Investment Target: 5,000,000
	mockRepo.On("GetTarget", mock.Anything, userID, model.TargetType("INVESTMENT"), month, year).
		Return(&model.MonthlyTarget{TargetAmount: 5000000}, nil)

	// Spent: 6,000,000 (Remaining = 4,000,000)
	mockRepo.On("GetMonthlyTotalByCategoryType", mock.Anything, userID, model.CategoryTypeExpense, mock.Anything, mock.Anything).
		Return(6000000.0, nil)

	// Invested: 3,500,000 (Remaining = 1,500,000)
	mockRepo.On("GetMonthlyTotalByCategoryType", mock.Anything, userID, model.CategoryTypeInvestment, mock.Anything, mock.Anything).
		Return(3500000.0, nil)

	res, err := svc.GetSummary(context.Background(), userID, month, year)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// Verify Expense Summary
	assert.Equal(t, 10000000.0, res.Expense.TargetAmount)
	assert.Equal(t, 6000000.0, res.Expense.SpentAmount)
	assert.Equal(t, 4000000.0, res.Expense.RemainingAmount)
	assert.False(t, res.Expense.IsOverBudget)
	assert.Equal(t, 0.0, res.Expense.OverspentAmount)
	assert.True(t, res.Expense.DailyAllowance > 0)

	// Verify Investment Summary
	assert.Equal(t, 5000000.0, res.Investment.TargetAmount)
	assert.Equal(t, 3500000.0, res.Investment.InvestedAmount)
	assert.Equal(t, 1500000.0, res.Investment.RemainingAmount)
	assert.False(t, res.Investment.IsTargetReached)
}

func TestTargetService_GetSummary_OverBudget(t *testing.T) {
	mockRepo := new(MockTargetRepo)
	svc := service.NewTargetService(mockRepo)

	userID := uuid.New()
	month := 8
	year := 2026

	// Mock Expense Target: 10,000,000
	mockRepo.On("GetTarget", mock.Anything, userID, model.TargetType("EXPENSE"), month, year).
		Return(&model.MonthlyTarget{TargetAmount: 10000000}, nil)

	// Mock Investment Target: 5,000,000
	mockRepo.On("GetTarget", mock.Anything, userID, model.TargetType("INVESTMENT"), month, year).
		Return(&model.MonthlyTarget{TargetAmount: 5000000}, nil)

	// Spent: 12,000,000 (Overbudget by 2,000,000)
	mockRepo.On("GetMonthlyTotalByCategoryType", mock.Anything, userID, model.CategoryTypeExpense, mock.Anything, mock.Anything).
		Return(12000000.0, nil)

	// Invested: 6,000,000 (Reached & Surplus = 1,000,000)
	mockRepo.On("GetMonthlyTotalByCategoryType", mock.Anything, userID, model.CategoryTypeInvestment, mock.Anything, mock.Anything).
		Return(6000000.0, nil)

	res, err := svc.GetSummary(context.Background(), userID, month, year)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// Verify Overbudget Expense
	assert.True(t, res.Expense.IsOverBudget)
	assert.Equal(t, 2000000.0, res.Expense.OverspentAmount)
	assert.Equal(t, 0.0, res.Expense.DailyAllowance)

	// Verify Reached Investment
	assert.True(t, res.Investment.IsTargetReached)
	assert.Equal(t, 1000000.0, res.Investment.SurplusAmount)
	assert.Equal(t, 0.0, res.Investment.RemainingAmount)
}
