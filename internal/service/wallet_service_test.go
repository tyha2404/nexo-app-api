package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
)

type MockWalletRepo struct {
	mock.Mock
}

func (m *MockWalletRepo) Create(ctx context.Context, wallet *model.Wallet) error {
	args := m.Called(ctx, wallet)
	return args.Error(0)
}

func (m *MockWalletRepo) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Wallet, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Wallet), args.Error(1)
}

func (m *MockWalletRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]model.Wallet), args.Error(1)
}

func (m *MockWalletRepo) Update(ctx context.Context, wallet *model.Wallet, includeBalance bool) error {
	args := m.Called(ctx, wallet, includeBalance)
	return args.Error(0)
}

func (m *MockWalletRepo) UpdateStatementSnapshot(ctx context.Context, walletID, userID uuid.UUID, statementBalance, minimumPayment, previousBalance float64) error {
	args := m.Called(ctx, walletID, userID, statementBalance, minimumPayment, previousBalance)
	return args.Error(0)
}

func (m *MockWalletRepo) SumAllocationPercent(ctx context.Context, userID uuid.UUID, excludeWalletID *uuid.UUID) (float64, error) {
	args := m.Called(ctx, userID, excludeWalletID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockWalletRepo) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *MockWalletRepo) TransferMoney(ctx context.Context, transfer *model.WalletTransfer) error {
	args := m.Called(ctx, transfer)
	return args.Error(0)
}

func (m *MockWalletRepo) AutoAllocateIncome(ctx context.Context, userID uuid.UUID, req *dto.AutoAllocateRequest) (*dto.AutoAllocateResponse, error) {
	args := m.Called(ctx, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.AutoAllocateResponse), args.Error(1)
}

func (m *MockWalletRepo) GetSummaryByUserID(ctx context.Context, userID uuid.UUID) (*dto.WalletSummaryResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.WalletSummaryResponse), args.Error(1)
}

func TestWalletService_CreateCreditCard(t *testing.T) {
	mockRepo := new(MockWalletRepo)
	svc := service.NewWalletService(mockRepo)

	userID := uuid.New()
	creditLimit := 30000000.0
	statementDay := 20
	dueDay := 5
	isIncluded := false

	req := dto.CreateWalletRequest{
		Name:              "Thẻ tín dụng ACB",
		Type:              model.WalletTypeCredit,
		Balance:           0,
		Currency:          "VND",
		Icon:              "💳",
		CreditLimit:       &creditLimit,
		StatementDay:      &statementDay,
		DueDay:            &dueDay,
		IsIncludedInTotal: &isIncluded,
	}

	mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(w *model.Wallet) bool {
		return w.Name == "Thẻ tín dụng ACB" &&
			w.Type == model.WalletTypeCredit &&
			w.CreditLimit != nil && *w.CreditLimit == 30000000.0 &&
			w.StatementDay != nil && *w.StatementDay == 20 &&
			w.DueDay != nil && *w.DueDay == 5 &&
			!w.IsIncludedInTotal
	})).Return(nil)

	resp, err := svc.CreateWallet(context.Background(), userID, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Thẻ tín dụng ACB", resp.Name)
	assert.Equal(t, model.WalletTypeCredit, resp.Type)
	assert.NotNil(t, resp.CreditLimit)
	assert.Equal(t, 30000000.0, *resp.CreditLimit)
	assert.NotNil(t, resp.AvailableCredit)
	assert.Equal(t, 30000000.0, *resp.AvailableCredit)
	assert.NotNil(t, resp.OutstandingDebt)
	assert.Equal(t, 0.0, *resp.OutstandingDebt)
}

func TestWalletService_CreditCardOutstandingCalculations(t *testing.T) {
	mockRepo := new(MockWalletRepo)
	svc := service.NewWalletService(mockRepo)

	userID := uuid.New()
	cardID := uuid.New()
	creditLimit := 30000000.0
	statementDay := 20
	dueDay := 5

	// Spent 5,000,000 VND -> balance is -5,000,000
	wallet := &model.Wallet{
		ID:                cardID,
		UserID:            userID,
		Name:              "Thẻ tín dụng ACB",
		Type:              model.WalletTypeCredit,
		Balance:           -5000000.0,
		Currency:          "VND",
		Icon:              "💳",
		CreditLimit:       &creditLimit,
		StatementDay:      &statementDay,
		DueDay:            &dueDay,
		IsIncludedInTotal: false,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	mockRepo.On("GetByID", mock.Anything, cardID, userID).Return(wallet, nil)

	resp, err := svc.GetWalletByID(context.Background(), userID, cardID)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, -5000000.0, resp.Balance)
	assert.NotNil(t, resp.OutstandingDebt)
	assert.Equal(t, 5000000.0, *resp.OutstandingDebt)
	assert.NotNil(t, resp.AvailableCredit)
	assert.Equal(t, 25000000.0, *resp.AvailableCredit)
	assert.NotNil(t, resp.StatementBalance)
	assert.Equal(t, 5000000.0, *resp.StatementBalance)
	assert.NotNil(t, resp.MinimumPayment)
	assert.Equal(t, 250000.0, *resp.MinimumPayment) // 5% of 5,000,000
}

func TestWalletService_TransferMoney(t *testing.T) {
	mockRepo := new(MockWalletRepo)
	svc := service.NewWalletService(mockRepo)

	userID := uuid.New()
	fromID := uuid.New()
	toID := uuid.New()

	req := dto.TransferMoneyRequest{
		FromWalletID: fromID,
		ToWalletID:   toID,
		Amount:       5000000.0,
		Fee:          0,
	}

	mockRepo.On("TransferMoney", mock.Anything, mock.MatchedBy(func(wt *model.WalletTransfer) bool {
		return wt.FromWalletID == fromID && wt.ToWalletID == toID && wt.Amount == 5000000.0
	})).Return(nil)

	resp, err := svc.TransferMoney(context.Background(), userID, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 5000000.0, resp.Amount)
}

func TestWalletService_TransferMoney_RejectsNegativeFee(t *testing.T) {
	mockRepo := new(MockWalletRepo)
	svc := service.NewWalletService(mockRepo)

	_, err := svc.TransferMoney(context.Background(), uuid.New(), dto.TransferMoneyRequest{
		FromWalletID: uuid.New(), ToWalletID: uuid.New(), Amount: 100000, Fee: -5000,
	})
	assert.True(t, errors.Is(err, service.ErrInvalidTransfer))
	mockRepo.AssertNotCalled(t, "TransferMoney", mock.Anything, mock.Anything)
}

func TestWalletService_CreateWallet_RejectsAllocationOver100(t *testing.T) {
	mockRepo := new(MockWalletRepo)
	svc := service.NewWalletService(mockRepo)
	userID := uuid.New()

	mockRepo.On("SumAllocationPercent", mock.Anything, userID, (*uuid.UUID)(nil)).Return(80.0, nil)

	_, err := svc.CreateWallet(context.Background(), userID, dto.CreateWalletRequest{Name: "Jar", Type: model.WalletTypeJar, AllocationPercent: 30})
	assert.True(t, errors.Is(err, service.ErrAllocationPercentExceeded))
	mockRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestWalletService_CreateWallet_AllowsAllocationUpTo100(t *testing.T) {
	mockRepo := new(MockWalletRepo)
	svc := service.NewWalletService(mockRepo)
	userID := uuid.New()

	mockRepo.On("SumAllocationPercent", mock.Anything, userID, (*uuid.UUID)(nil)).Return(70.0, nil)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	_, err := svc.CreateWallet(context.Background(), userID, dto.CreateWalletRequest{Name: "Jar", Type: model.WalletTypeJar, AllocationPercent: 30})
	assert.NoError(t, err)
}

func TestWalletService_UpdateWallet_AllocationExcludesSelfAndKeepsBalance(t *testing.T) {
	mockRepo := new(MockWalletRepo)
	svc := service.NewWalletService(mockRepo)
	userID, walletID := uuid.New(), uuid.New()
	wallet := &model.Wallet{ID: walletID, UserID: userID, AllocationPercent: 50, Balance: 1000}

	mockRepo.On("GetByID", mock.Anything, walletID, userID).Return(wallet, nil)
	mockRepo.On("SumAllocationPercent", mock.Anything, userID, mock.MatchedBy(func(id *uuid.UUID) bool { return id != nil && *id == walletID })).Return(40.0, nil)
	// Balance not sent -> includeBalance=false so a stale balance is never written.
	mockRepo.On("Update", mock.Anything, wallet, false).Return(nil)

	pct := 60.0
	_, err := svc.UpdateWallet(context.Background(), userID, walletID, dto.UpdateWalletRequest{AllocationPercent: &pct})
	assert.NoError(t, err)

	pct = 61.0
	_, err = svc.UpdateWallet(context.Background(), userID, walletID, dto.UpdateWalletRequest{AllocationPercent: &pct})
	assert.True(t, errors.Is(err, service.ErrAllocationPercentExceeded))
	mockRepo.AssertNumberOfCalls(t, "Update", 1)
}
