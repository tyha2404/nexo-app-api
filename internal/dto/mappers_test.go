package dto_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

func ptr[T any](v T) *T { return &v }

func TestNewUserResponse(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	user := &model.User{
		ID:        uuid.New(),
		Username:  "tyha",
		Email:     "tyha@example.com",
		Password:  "$2a$10$hash",
		Role:      "admin",
		CreatedAt: created,
		UpdatedAt: created.Add(time.Hour),
	}

	resp := dto.NewUserResponse(user)

	assert.Equal(t, dto.UserResponse{
		ID:        user.ID.String(),
		Username:  "tyha",
		Email:     "tyha@example.com",
		CreatedAt: "2026-01-02T03:04:05Z",
		UpdatedAt: "2026-01-02T04:04:05Z",
	}, resp, "password and role must never be exposed")

	var empty dto.UserResponse
	empty.FromUser(nil)
	assert.Equal(t, dto.UserResponse{}, empty)
}

func TestToAlertResponse(t *testing.T) {
	at := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	a := &model.Alert{
		ID: uuid.New(), UserID: uuid.New(), BudgetID: uuid.New(),
		AlertType: "over_limit", Message: "Vượt ngân sách", TriggeredAt: at, CreatedAt: at,
	}

	resp := dto.ToAlertResponse(a)

	assert.Equal(t, a.ID, resp.ID)
	assert.Equal(t, a.UserID, resp.UserID)
	assert.Equal(t, a.BudgetID, resp.BudgetID)
	assert.Equal(t, "over_limit", resp.AlertType)
	assert.Equal(t, "Vượt ngân sách", resp.Message)
	assert.Equal(t, "2026-03-01T08:00:00Z", resp.TriggeredAt)
	assert.Equal(t, "2026-03-01T08:00:00Z", resp.CreatedAt)
}

func TestToBudgetResponse(t *testing.T) {
	b := &model.Budget{
		ID: uuid.New(), UserID: uuid.New(), CategoryID: uuid.New(),
		Amount: 5_000_000, PeriodType: "MONTHLY",
		PeriodStart: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		CreatedAt:   time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC),
	}

	resp := dto.ToBudgetResponse(b)
	assert.Equal(t, "", resp.CategoryName, "no preloaded category")
	assert.Equal(t, "2026-11-01", resp.PeriodStart)
	assert.Equal(t, "2026-11-01T09:30:00Z", resp.CreatedAt)
	assert.Equal(t, 5_000_000.0, resp.Amount)

	b.Category.Name = "Ăn uống"
	assert.Equal(t, "Ăn uống", dto.ToBudgetResponse(b).CategoryName)
}

func TestToPresetResponse(t *testing.T) {
	p := &model.Preset{
		ID: uuid.New(), UserID: uuid.New(), CategoryID: uuid.New(),
		Name: "Cà phê", Amount: 35000, Type: model.TransactionTypeExpense,
		Description: "Sáng", Icon: "☕", SortOrder: 2,
	}
	p.Category.Name = "Ăn uống"

	resp := dto.ToPresetResponse(p)

	assert.Equal(t, &dto.PresetResponse{
		ID: p.ID, UserID: p.UserID, CategoryID: p.CategoryID, CategoryName: "Ăn uống",
		Name: "Cà phê", Amount: 35000, Type: "EXPENSE", Description: "Sáng", Icon: "☕", SortOrder: 2,
	}, resp)
}

func TestToWalletResponse_NonCreditHasNoCreditFields(t *testing.T) {
	w := &model.Wallet{ID: uuid.New(), Name: "Tiền mặt", Type: model.WalletTypeCash, Balance: 1_000_000, Currency: "VND"}

	resp := dto.ToWalletResponse(w)

	assert.Equal(t, 1_000_000.0, resp.Balance)
	assert.Nil(t, resp.OutstandingDebt)
	assert.Nil(t, resp.AvailableCredit)
	assert.Nil(t, resp.StatementBalance)
	assert.Nil(t, resp.MinimumPayment)
}

func TestToWalletResponse_CreditCard(t *testing.T) {
	tests := []struct {
		name          string
		balance       float64
		limit         *float64
		statement     *float64
		minimum       *float64
		wantDebt      float64
		wantAvailable *float64
		wantStatement *float64
		wantMinimum   *float64
	}{
		{
			name: "spent part of the limit: 5% minimum", balance: -2_000_000, limit: ptr(10_000_000.0),
			wantDebt: 2_000_000, wantAvailable: ptr(8_000_000.0), wantStatement: ptr(2_000_000.0), wantMinimum: ptr(100_000.0),
		},
		{
			name: "small statement: minimum floored at 50k", balance: -600_000, limit: ptr(10_000_000.0),
			wantDebt: 600_000, wantAvailable: ptr(9_400_000.0), wantStatement: ptr(600_000.0), wantMinimum: ptr(50_000.0),
		},
		{
			name: "statement under 50k: minimum is 5%, not the floor", balance: -40_000,
			wantDebt: 40_000, wantStatement: ptr(40_000.0), wantMinimum: ptr(2_000.0),
		},
		{
			name: "over the limit: available credit never negative", balance: -12_000_000, limit: ptr(10_000_000.0),
			wantDebt: 12_000_000, wantAvailable: ptr(0.0), wantStatement: ptr(12_000_000.0), wantMinimum: ptr(600_000.0),
		},
		{
			name: "configured statement and minimum are kept", balance: -3_000_000, limit: ptr(10_000_000.0),
			statement: ptr(1_500_000.0), minimum: ptr(200_000.0),
			wantDebt: 3_000_000, wantAvailable: ptr(7_000_000.0), wantStatement: ptr(1_500_000.0), wantMinimum: ptr(200_000.0),
		},
		{
			name: "nothing owed", balance: 0, limit: ptr(10_000_000.0),
			wantDebt: 0, wantAvailable: ptr(10_000_000.0),
		},
		{
			name: "no limit configured: no available credit", balance: -100_000, limit: ptr(0.0),
			wantDebt: 100_000, wantStatement: ptr(100_000.0), wantMinimum: ptr(50_000.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &model.Wallet{
				Type: model.WalletTypeCredit, Balance: tt.balance, CreditLimit: tt.limit,
				StatementBalance: tt.statement, MinimumPayment: tt.minimum,
			}

			resp := dto.ToWalletResponse(w)

			require.NotNil(t, resp.OutstandingDebt)
			assert.Equal(t, tt.wantDebt, *resp.OutstandingDebt)
			assert.Equal(t, tt.wantAvailable, resp.AvailableCredit)
			assert.Equal(t, tt.wantStatement, resp.StatementBalance)
			if tt.wantMinimum == nil {
				assert.Nil(t, resp.MinimumPayment)
			} else {
				require.NotNil(t, resp.MinimumPayment)
				assert.InDelta(t, *tt.wantMinimum, *resp.MinimumPayment, 0.001)
			}
		})
	}
}
