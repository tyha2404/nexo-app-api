package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
)

func day(t *testing.T, s string) *dto.CustomTime {
	t.Helper()
	var ct dto.CustomTime
	require.NoError(t, json.Unmarshal([]byte(`"`+s+`"`), &ct))
	return &ct
}

// The web sends the picked day as YYYY-MM-DD (like transaction dates); it is
// stored and returned as that same day.
func TestCreateDebt_KeepsPickedCalendarDay(t *testing.T) {
	repo := new(MockDebtRepo)
	svc := service.NewDebtService(repo)

	var saved *model.Debt
	repo.On("CreateWithWallet", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(a mock.Arguments) { saved = a.Get(1).(*model.Debt) }).Return(nil)

	res, err := svc.CreateDebt(context.Background(), uuid.New(), dto.CreateDebtRequest{
		Type: model.DebtTypePayable, Title: "A", TotalAmount: 100,
		StartDate: day(t, "2026-11-11"), DueDate: day(t, "2099-12-01"),
	})
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC), *saved.StartDate)
	assert.Equal(t, time.Date(2099, 12, 1, 0, 0, 0, 0, time.UTC), *saved.DueDate)
	assert.Equal(t, "2026-11-11", *res.StartDate)
	assert.Equal(t, "2099-12-01", *res.DueDate)
}

// Cached PWA builds still send local midnight with an offset; keep that day too.
func TestCreateDebt_AcceptsLocalMidnightWithOffset(t *testing.T) {
	repo := new(MockDebtRepo)
	repo.On("CreateWithWallet", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	res, err := service.NewDebtService(repo).CreateDebt(context.Background(), uuid.New(), dto.CreateDebtRequest{
		Type: model.DebtTypePayable, Title: "A", TotalAmount: 100, StartDate: day(t, "2026-11-11T00:00:00+07:00"),
	})
	require.NoError(t, err)
	assert.Equal(t, "2026-11-11", *res.StartDate)
}

// Rows saved before the fix hold local midnight shifted to 17:00 UTC of the
// previous day; the API must still return the day the user picked.
func TestGetDebts_RecoversShiftedLegacyDates(t *testing.T) {
	repo := new(MockDebtRepo)
	svc := service.NewDebtService(repo)
	legacy := time.Date(2026, 11, 10, 17, 0, 0, 0, time.UTC)
	repo.On("FindByUserID", mock.Anything, mock.Anything, model.DebtType(""), model.DebtStatus("")).
		Return([]model.Debt{{ID: uuid.New(), TotalAmount: 1, StartDate: &legacy}}, nil)

	res, err := svc.GetDebts(context.Background(), uuid.New(), "", "")
	require.NoError(t, err)
	assert.Equal(t, "2026-11-11", *res[0].StartDate)
	assert.Nil(t, res[0].DueDate)
}

func TestUpdateDebt(t *testing.T) {
	userID, debtID, walletID := uuid.New(), uuid.New(), uuid.New()
	stored := func() *model.Debt {
		return &model.Debt{ID: debtID, UserID: userID, Type: model.DebtTypePayable, Title: "Old",
			TotalAmount: 1000, PaidAmount: 400, Status: model.DebtStatusPending}
	}

	t.Run("replaces fields, keeps the picked day and recomputes status", func(t *testing.T) {
		repo := new(MockDebtRepo)
		repo.On("UpdateWithWallet", mock.Anything, debtID, userID).Return(stored(), nil)
		res, err := service.NewDebtService(repo).UpdateDebt(context.Background(), userID, debtID, dto.UpdateDebtRequest{
			Title: "New", TotalAmount: 400, StartDate: day(t, "2026-11-11"), Notes: "n", WalletID: &walletID,
		})
		require.NoError(t, err)
		assert.Equal(t, "New", res.Title)
		assert.Equal(t, 400.0, res.TotalAmount)
		assert.Equal(t, model.DebtStatusCompleted, res.Status)
		assert.Equal(t, "2026-11-11", *res.StartDate)
		assert.Nil(t, res.DueDate)
		assert.Equal(t, &walletID, res.WalletID)
	})

	t.Run("rejects a total below what was repaid", func(t *testing.T) {
		repo := new(MockDebtRepo)
		repo.On("UpdateWithWallet", mock.Anything, debtID, userID).Return(stored(), nil)
		_, err := service.NewDebtService(repo).UpdateDebt(context.Background(), userID, debtID, dto.UpdateDebtRequest{
			Title: "New", TotalAmount: 300,
		})
		assert.True(t, errors.Is(err, service.ErrInvalidDebtUpdate))
	})

	t.Run("rejects a due date before the start date", func(t *testing.T) {
		_, err := service.NewDebtService(new(MockDebtRepo)).UpdateDebt(context.Background(), userID, debtID, dto.UpdateDebtRequest{
			Title: "New", TotalAmount: 1000, StartDate: day(t, "2026-11-11"), DueDate: day(t, "2026-11-10"),
		})
		assert.True(t, errors.Is(err, service.ErrInvalidDebtUpdate))
	})

	t.Run("maps a missing debt to ErrDebtNotFound", func(t *testing.T) {
		repo := new(MockDebtRepo)
		repo.On("UpdateWithWallet", mock.Anything, debtID, userID).Return(nil, nil)
		_, err := service.NewDebtService(repo).UpdateDebt(context.Background(), userID, debtID, dto.UpdateDebtRequest{
			Title: "New", TotalAmount: 1000,
		})
		assert.True(t, errors.Is(err, service.ErrDebtNotFound))
	})
}
