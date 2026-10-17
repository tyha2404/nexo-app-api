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

type MockStatementRepo struct {
	mock.Mock
}

func (m *MockStatementRepo) Create(ctx context.Context, s *model.CreditCardStatement) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}

func (m *MockStatementRepo) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.CreditCardStatement, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.CreditCardStatement), args.Error(1)
}

func (m *MockStatementRepo) GetByWalletID(ctx context.Context, walletID uuid.UUID, userID uuid.UUID) ([]model.CreditCardStatement, error) {
	args := m.Called(ctx, walletID, userID)
	return args.Get(0).([]model.CreditCardStatement), args.Error(1)
}

func (m *MockStatementRepo) List(ctx context.Context, userID uuid.UUID, walletID *uuid.UUID, year *int, month *int) ([]model.CreditCardStatement, error) {
	args := m.Called(ctx, userID, walletID, year, month)
	return args.Get(0).([]model.CreditCardStatement), args.Error(1)
}

func (m *MockStatementRepo) Update(ctx context.Context, s *model.CreditCardStatement) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}

func (m *MockStatementRepo) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *MockStatementRepo) PayStatement(ctx context.Context, userID, id uuid.UUID, amount float64, sourceWalletID *uuid.UUID) (*model.CreditCardStatement, error) {
	args := m.Called(ctx, userID, id, amount, sourceWalletID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.CreditCardStatement), args.Error(1)
}

func TestCreditCardStatementService_CreateAndPayStatement(t *testing.T) {
	stmtRepo := new(MockStatementRepo)
	walletRepo := new(MockWalletRepo)
	svc := service.NewCreditCardStatementService(stmtRepo, walletRepo)

	userID := uuid.New()
	walletID := uuid.New()

	wallet := &model.Wallet{
		ID:       walletID,
		UserID:   userID,
		Name:     "Thẻ tín dụng ACB",
		Type:     model.WalletTypeCredit,
		Balance:  -5000000.0,
		Currency: "VND",
	}

	walletRepo.On("GetByID", mock.Anything, walletID, userID).Return(wallet, nil)
	walletRepo.On("UpdateStatementSnapshot", mock.Anything, walletID, userID, 5000000.0, 250000.0, 0.0).Return(nil)
	stmtRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	req := dto.CreateStatementRequest{
		WalletID:         walletID,
		StatementMonth:   8,
		StatementYear:    2026,
		StatementDate:    "2026-08-20",
		DueDate:          "2026-12-05",
		StatementBalance: 5000000.0,
		MinimumPayment:   250000.0,
		PreviousBalance:  0.0,
		PaidAmount:       0.0,
	}

	res, err := svc.CreateStatement(context.Background(), userID, req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 5000000.0, res.StatementBalance)
	assert.Equal(t, 250000.0, res.MinimumPayment)
	assert.Equal(t, model.StatementStatusUnpaid, res.Status)

	// Test Payment
	stmtID := res.ID
	stmtObj := &model.CreditCardStatement{
		ID:               stmtID,
		UserID:           userID,
		WalletID:         walletID,
		StatementMonth:   8,
		StatementYear:    2026,
		StatementDate:    time.Now(),
		DueDate:          time.Now().AddDate(0, 0, 15),
		StatementBalance: 5000000.0,
		MinimumPayment:   250000.0,
		PaidAmount:       0.0,
		Status:           model.StatementStatusUnpaid,
	}

	paid := *stmtObj
	paid.PaidAmount = 5000000.0
	paid.Status = model.StatementStatusPaid
	stmtRepo.On("PayStatement", mock.Anything, userID, stmtID, 5000000.0, (*uuid.UUID)(nil)).Return(&paid, nil)

	payReq := dto.PayStatementRequest{
		Amount: 5000000.0,
	}

	payRes, err := svc.PayStatement(context.Background(), userID, stmtID, payReq)
	assert.NoError(t, err)
	assert.NotNil(t, payRes)
	assert.Equal(t, 5000000.0, payRes.PaidAmount)
	assert.Equal(t, 0.0, payRes.RemainingAmount)
	assert.Equal(t, model.StatementStatusPaid, payRes.Status)
}

func TestCreditCardStatementService_PayStatement_PropagatesErrors(t *testing.T) {
	userID, stmtID, srcID := uuid.New(), uuid.New(), uuid.New()
	cases := []struct {
		name    string
		repoRes *model.CreditCardStatement
		repoErr error
		want    error
	}{
		{"not found", nil, nil, service.ErrStatementNotFound},
		{"already paid", nil, service.ErrStatementAlreadyPaid, service.ErrStatementAlreadyPaid},
		{"overpayment", nil, service.ErrStatementInvalidPayment, service.ErrStatementInvalidPayment},
		{"source wallet missing", nil, errors.New("source wallet not found"), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stmtRepo := new(MockStatementRepo)
			walletRepo := new(MockWalletRepo)
			svc := service.NewCreditCardStatementService(stmtRepo, walletRepo)
			stmtRepo.On("PayStatement", mock.Anything, userID, stmtID, 100.0, &srcID).Return(tc.repoRes, tc.repoErr)

			res, err := svc.PayStatement(context.Background(), userID, stmtID, dto.PayStatementRequest{Amount: 100, SourceWalletID: &srcID})
			assert.Nil(t, res)
			assert.Error(t, err)
			if tc.want != nil {
				assert.True(t, errors.Is(err, tc.want))
			}
			// No direct wallet writes outside the repository transaction.
			walletRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}
