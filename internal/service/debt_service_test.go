package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
)

type MockDebtRepo struct {
	mock.Mock
}

func (m *MockDebtRepo) Create(ctx context.Context, debt *model.Debt) error {
	args := m.Called(ctx, debt)
	return args.Error(0)
}

func (m *MockDebtRepo) CreateWithWallet(ctx context.Context, debt *model.Debt, walletID *uuid.UUID, balanceDelta float64) error {
	args := m.Called(ctx, debt, walletID, balanceDelta)
	return args.Error(0)
}

func (m *MockDebtRepo) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Debt, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Debt), args.Error(1)
}

func (m *MockDebtRepo) FindByUserID(ctx context.Context, userID uuid.UUID, debtType model.DebtType, status model.DebtStatus) ([]model.Debt, error) {
	args := m.Called(ctx, userID, debtType, status)
	return args.Get(0).([]model.Debt), args.Error(1)
}

func (m *MockDebtRepo) AddRepayment(ctx context.Context, debt *model.Debt, repayment *model.Repayment) error {
	args := m.Called(ctx, debt, repayment)
	return args.Error(0)
}

func (m *MockDebtRepo) AddRepaymentWithWallet(ctx context.Context, debt *model.Debt, repayment *model.Repayment, walletID *uuid.UUID, balanceDelta float64) error {
	args := m.Called(ctx, debt, repayment, walletID, balanceDelta)
	return args.Error(0)
}

func (m *MockDebtRepo) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *MockDebtRepo) GetSummaryByUserID(ctx context.Context, userID uuid.UUID) (*dto.DebtSummaryResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.DebtSummaryResponse), args.Error(1)
}

func TestCreateDebt_WithWallet_Receivable(t *testing.T) {
	mockRepo := new(MockDebtRepo)
	svc := service.NewDebtService(mockRepo)

	userID := uuid.New()
	walletID := uuid.New()
	req := dto.CreateDebtRequest{
		Type:        model.DebtTypeReceivable, // Cho vay -> trừ tiền ví
		Title:       "Cho bạn mượn",
		TotalAmount: 2000000,
		WalletID:    &walletID,
	}

	mockRepo.On("CreateWithWallet", mock.Anything, mock.MatchedBy(func(d *model.Debt) bool {
		return d.Title == "Cho bạn mượn" && d.TotalAmount == 2000000 && d.WalletID == &walletID
	}), &walletID, -2000000.0).Return(nil)

	res, err := svc.CreateDebt(context.Background(), userID, req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, float64(2000000), res.TotalAmount)
	assert.Equal(t, &walletID, res.WalletID)
	mockRepo.AssertExpectations(t)
}

func TestCreateDebt_WithWallet_Payable(t *testing.T) {
	mockRepo := new(MockDebtRepo)
	svc := service.NewDebtService(mockRepo)

	userID := uuid.New()
	walletID := uuid.New()
	req := dto.CreateDebtRequest{
		Type:        model.DebtTypePayable, // Đi vay -> cộng tiền ví
		Title:       "Vay ngân hàng",
		TotalAmount: 50000000,
		WalletID:    &walletID,
	}

	mockRepo.On("CreateWithWallet", mock.Anything, mock.MatchedBy(func(d *model.Debt) bool {
		return d.Title == "Vay ngân hàng" && d.TotalAmount == 50000000 && d.WalletID == &walletID
	}), &walletID, 50000000.0).Return(nil)

	res, err := svc.CreateDebt(context.Background(), userID, req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, float64(50000000), res.TotalAmount)
	assert.Equal(t, &walletID, res.WalletID)
	mockRepo.AssertExpectations(t)
}

func TestAddRepayment_WithWallet_Receivable(t *testing.T) {
	mockRepo := new(MockDebtRepo)
	svc := service.NewDebtService(mockRepo)

	userID := uuid.New()
	debtID := uuid.New()
	walletID := uuid.New()

	existingDebt := &model.Debt{
		ID:          debtID,
		UserID:      userID,
		Type:        model.DebtTypeReceivable, // Thu hồi nợ -> cộng tiền ví
		Title:       "Cho bạn mượn",
		TotalAmount: 2000000,
		PaidAmount:  0,
		Status:      model.DebtStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockRepo.On("FindByID", mock.Anything, debtID, userID).Return(existingDebt, nil)
	mockRepo.On("AddRepaymentWithWallet", mock.Anything, existingDebt, mock.MatchedBy(func(r *model.Repayment) bool {
		return r.Amount == 1000000 && r.WalletID == &walletID
	}), &walletID, 1000000.0).Return(nil)

	req := dto.AddRepaymentRequest{
		Amount:   1000000,
		WalletID: &walletID,
		Notes:    "Trả đợt 1",
	}

	res, err := svc.AddRepayment(context.Background(), userID, debtID, req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, float64(1000000), res.PaidAmount)
	assert.Equal(t, float64(1000000), res.Remaining)
	mockRepo.AssertExpectations(t)
}

func TestAddRepayment_WithWallet_Payable(t *testing.T) {
	mockRepo := new(MockDebtRepo)
	svc := service.NewDebtService(mockRepo)

	userID := uuid.New()
	debtID := uuid.New()
	walletID := uuid.New()

	existingDebt := &model.Debt{
		ID:          debtID,
		UserID:      userID,
		Type:        model.DebtTypePayable, // Trả nợ -> trừ tiền ví
		Title:       "Vay ngân hàng",
		TotalAmount: 50000000,
		PaidAmount:  0,
		Status:      model.DebtStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockRepo.On("FindByID", mock.Anything, debtID, userID).Return(existingDebt, nil)
	mockRepo.On("AddRepaymentWithWallet", mock.Anything, existingDebt, mock.MatchedBy(func(r *model.Repayment) bool {
		return r.Amount == 10000000 && r.WalletID == &walletID
	}), &walletID, -10000000.0).Return(nil)

	req := dto.AddRepaymentRequest{
		Amount:   10000000,
		WalletID: &walletID,
		Notes:    "Trả kỳ 1",
	}

	res, err := svc.AddRepayment(context.Background(), userID, debtID, req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, float64(10000000), res.PaidAmount)
	assert.Equal(t, float64(40000000), res.Remaining)
	mockRepo.AssertExpectations(t)
}
