package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
)

type MockRolloverRepo struct {
	mock.Mock
}

func (m *MockRolloverRepo) GetAllActiveUserIDs(ctx context.Context) ([]uuid.UUID, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

func (m *MockRolloverRepo) GetTargetForMonth(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error) {
	args := m.Called(ctx, userID, targetType, month, year)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.MonthlyTarget), args.Error(1)
}

func (m *MockRolloverRepo) GetLatestTargetBefore(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error) {
	args := m.Called(ctx, userID, targetType, month, year)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.MonthlyTarget), args.Error(1)
}

func (m *MockRolloverRepo) GetMonthlyBudgetsForPeriod(ctx context.Context, userID uuid.UUID, periodStart time.Time) ([]model.Budget, error) {
	args := m.Called(ctx, userID, periodStart)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Budget), args.Error(1)
}

func (m *MockRolloverRepo) GetLatestMonthlyBudgetsBefore(ctx context.Context, userID uuid.UUID, periodStart time.Time) ([]model.Budget, error) {
	args := m.Called(ctx, userID, periodStart)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Budget), args.Error(1)
}

func (m *MockRolloverRepo) CreateMonthlyTargets(ctx context.Context, targets []model.MonthlyTarget) error {
	args := m.Called(ctx, targets)
	return args.Error(0)
}

func (m *MockRolloverRepo) CreateBudgets(ctx context.Context, budgets []model.Budget) error {
	args := m.Called(ctx, budgets)
	return args.Error(0)
}

func TestRolloverService_ProcessRolloverForUser_CopiesBothTargetsAndBudgets(t *testing.T) {
	mockRepo := new(MockRolloverRepo)
	logger := zap.NewNop()
	svc := service.NewRolloverService(mockRepo, logger)

	userID := uuid.New()
	targetMonth := 9
	targetYear := 2026
	targetPeriodStart := time.Date(targetYear, time.Month(targetMonth), 1, 0, 0, 0, 0, time.UTC)
	catID1 := uuid.New()
	catID2 := uuid.New()

	// Target check: neither exists for 09/2026
	mockRepo.On("GetTargetForMonth", mock.Anything, userID, model.TargetTypeExpense, targetMonth, targetYear).
		Return(nil, nil)
	mockRepo.On("GetTargetForMonth", mock.Anything, userID, model.TargetTypeInvestment, targetMonth, targetYear).
		Return(nil, nil)

	// Latest targets found from previous month (08/2026)
	mockRepo.On("GetLatestTargetBefore", mock.Anything, userID, model.TargetTypeExpense, targetMonth, targetYear).
		Return(&model.MonthlyTarget{TargetAmount: 15000000}, nil)
	mockRepo.On("GetLatestTargetBefore", mock.Anything, userID, model.TargetTypeInvestment, targetMonth, targetYear).
		Return(&model.MonthlyTarget{TargetAmount: 5000000}, nil)

	// Budgets check: none exists for 09/2026
	mockRepo.On("GetMonthlyBudgetsForPeriod", mock.Anything, userID, targetPeriodStart).
		Return([]model.Budget{}, nil)

	// Latest budgets found from previous period
	latestBudgets := []model.Budget{
		{CategoryID: catID1, Amount: 3000000},
		{CategoryID: catID2, Amount: 2000000},
	}
	mockRepo.On("GetLatestMonthlyBudgetsBefore", mock.Anything, userID, targetPeriodStart).
		Return(latestBudgets, nil)

	// Expect create calls
	mockRepo.On("CreateMonthlyTargets", mock.Anything, mock.MatchedBy(func(targets []model.MonthlyTarget) bool {
		if len(targets) != 2 {
			return false
		}
		return targets[0].TargetAmount == 15000000 && targets[1].TargetAmount == 5000000
	})).Return(nil)

	mockRepo.On("CreateBudgets", mock.Anything, mock.MatchedBy(func(budgets []model.Budget) bool {
		if len(budgets) != 2 {
			return false
		}
		return budgets[0].Amount == 3000000 && budgets[1].Amount == 2000000 && budgets[0].PeriodStart.Equal(targetPeriodStart)
	})).Return(nil)

	err := svc.ProcessRolloverForUser(context.Background(), userID, targetMonth, targetYear)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestRolloverService_ProcessRolloverForUser_SkipsExisting(t *testing.T) {
	mockRepo := new(MockRolloverRepo)
	logger := zap.NewNop()
	svc := service.NewRolloverService(mockRepo, logger)

	userID := uuid.New()
	targetMonth := 9
	targetYear := 2026
	targetPeriodStart := time.Date(targetYear, time.Month(targetMonth), 1, 0, 0, 0, 0, time.UTC)
	catID := uuid.New()

	// Targets already exist for 09/2026
	mockRepo.On("GetTargetForMonth", mock.Anything, userID, model.TargetTypeExpense, targetMonth, targetYear).
		Return(&model.MonthlyTarget{TargetAmount: 20000000}, nil)
	mockRepo.On("GetTargetForMonth", mock.Anything, userID, model.TargetTypeInvestment, targetMonth, targetYear).
		Return(&model.MonthlyTarget{TargetAmount: 8000000}, nil)

	// Budget already exists for 09/2026
	mockRepo.On("GetMonthlyBudgetsForPeriod", mock.Anything, userID, targetPeriodStart).
		Return([]model.Budget{
			{CategoryID: catID, Amount: 5000000},
		}, nil)

	mockRepo.On("GetLatestMonthlyBudgetsBefore", mock.Anything, userID, targetPeriodStart).
		Return([]model.Budget{
			{CategoryID: catID, Amount: 4000000},
		}, nil)

	// Should not call create targets or budgets because everything is already configured
	mockRepo.On("CreateMonthlyTargets", mock.Anything, []model.MonthlyTarget{}).Return(nil).Maybe()
	mockRepo.On("CreateBudgets", mock.Anything, []model.Budget{}).Return(nil).Maybe()

	err := svc.ProcessRolloverForUser(context.Background(), userID, targetMonth, targetYear)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestRolloverService_ProcessRolloverForMonth_ProcessesAllUsers(t *testing.T) {
	mockRepo := new(MockRolloverRepo)
	logger := zap.NewNop()
	svc := service.NewRolloverService(mockRepo, logger)

	user1 := uuid.New()
	user2 := uuid.New()
	targetMonth := 9
	targetYear := 2026
	targetPeriodStart := time.Date(targetYear, time.Month(targetMonth), 1, 0, 0, 0, 0, time.UTC)

	mockRepo.On("GetAllActiveUserIDs", mock.Anything).Return([]uuid.UUID{user1, user2}, nil)

	// For user1: has no targets/budgets in new month, has history
	mockRepo.On("GetTargetForMonth", mock.Anything, user1, model.TargetTypeExpense, targetMonth, targetYear).Return(nil, nil)
	mockRepo.On("GetTargetForMonth", mock.Anything, user1, model.TargetTypeInvestment, targetMonth, targetYear).Return(nil, nil)
	mockRepo.On("GetLatestTargetBefore", mock.Anything, user1, model.TargetTypeExpense, targetMonth, targetYear).
		Return(&model.MonthlyTarget{TargetAmount: 10000000}, nil)
	mockRepo.On("GetLatestTargetBefore", mock.Anything, user1, model.TargetTypeInvestment, targetMonth, targetYear).
		Return(nil, nil) // no investment history
	mockRepo.On("GetMonthlyBudgetsForPeriod", mock.Anything, user1, targetPeriodStart).Return([]model.Budget{}, nil)
	mockRepo.On("GetLatestMonthlyBudgetsBefore", mock.Anything, user1, targetPeriodStart).Return([]model.Budget{}, nil)

	mockRepo.On("CreateMonthlyTargets", mock.Anything, mock.MatchedBy(func(t []model.MonthlyTarget) bool {
		return len(t) == 1 && t[0].TargetAmount == 10000000
	})).Return(nil)

	// For user2: no history at all
	mockRepo.On("GetTargetForMonth", mock.Anything, user2, model.TargetTypeExpense, targetMonth, targetYear).Return(nil, nil)
	mockRepo.On("GetTargetForMonth", mock.Anything, user2, model.TargetTypeInvestment, targetMonth, targetYear).Return(nil, nil)
	mockRepo.On("GetLatestTargetBefore", mock.Anything, user2, model.TargetTypeExpense, targetMonth, targetYear).Return(nil, nil)
	mockRepo.On("GetLatestTargetBefore", mock.Anything, user2, model.TargetTypeInvestment, targetMonth, targetYear).Return(nil, nil)
	mockRepo.On("GetMonthlyBudgetsForPeriod", mock.Anything, user2, targetPeriodStart).Return([]model.Budget{}, nil)
	mockRepo.On("GetLatestMonthlyBudgetsBefore", mock.Anything, user2, targetPeriodStart).Return([]model.Budget{}, nil)

	err := svc.ProcessRolloverForMonth(context.Background(), targetMonth, targetYear)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}
