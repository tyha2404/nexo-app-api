package service_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

func ptr[T any](v T) *T { return &v }

// ----------------------------- wallets -------------------------------------

func TestWalletService_GetWalletsDelegatesWithUser(t *testing.T) {
	repo := new(MockWalletRepo)
	userID := uuid.New()
	wallets := []model.Wallet{
		{ID: uuid.New(), UserID: userID, Name: "Tiền mặt", Balance: 1_000_000, Currency: "VND", IsIncludedInTotal: true},
		{ID: uuid.New(), UserID: userID, Name: "Thẻ", Balance: 500_000, IsIncludedInTotal: true},
	}
	repo.On("ListByUserID", mock.Anything, userID).Return(wallets, nil)

	got, err := service.NewWalletService(repo).GetWallets(context.Background(), userID)
	require.NoError(t, err)
	assert.Equal(t, 1_500_000.0, got.TotalBalance)
	assert.Equal(t, "VND", got.BaseCurrency)
	assert.Equal(t, 2, got.TotalWallets)
	require.Len(t, got.Wallets, 2)
	assert.Equal(t, "Tiền mặt", got.Wallets[0].Name)

	other := uuid.New()
	repo.On("ListByUserID", mock.Anything, other).Return([]model.Wallet(nil), errRepoDown)
	_, err = service.NewWalletService(repo).GetWallets(context.Background(), other)
	require.ErrorIs(t, err, errRepoDown)
}

func TestWalletService_GetWalletByIDNotOwnedIsNotFound(t *testing.T) {
	repo := new(MockWalletRepo)
	userID, walletID := uuid.New(), uuid.New()
	// The repository is user scoped: another user's wallet comes back as nil.
	repo.On("GetByIDForUser", mock.Anything, walletID, userID).Return(nil, repository.ErrWalletNotFound)

	_, err := service.NewWalletService(repo).GetWalletByID(context.Background(), userID, walletID)
	require.ErrorIs(t, err, service.ErrWalletNotFound)
}

func TestWalletService_UpdateWalletAppliesAllFields(t *testing.T) {
	repo := new(MockWalletRepo)
	svc := service.NewWalletService(repo)
	userID, walletID := uuid.New(), uuid.New()
	wallet := &model.Wallet{ID: walletID, UserID: userID, Name: "Old", Type: model.WalletTypeCash, Balance: 10, Currency: "VND"}
	repo.On("GetByIDForUser", mock.Anything, walletID, userID).Return(wallet, nil)
	repo.On("Update", mock.Anything, wallet, true).Return(nil)

	req := dto.UpdateWalletRequest{
		Name:              ptr("Thẻ VIB"),
		Type:              ptr(model.WalletTypeCredit),
		Balance:           ptr(-2_000_000.0),
		Currency:          ptr("VND"),
		Icon:              ptr("💳"),
		JarCategory:       ptr("NEC"),
		AllocationPercent: ptr(0.0),
		IsIncludedInTotal: ptr(false),
		CreditLimit:       ptr(20_000_000.0),
		StatementDay:      ptr(20),
		DueDay:            ptr(5),
		StatementBalance:  ptr(2_000_000.0),
		MinimumPayment:    ptr(100_000.0),
		PreviousBalance:   ptr(0.0),
	}
	res, err := svc.UpdateWallet(context.Background(), userID, walletID, req)
	require.NoError(t, err)
	require.Equal(t, "Thẻ VIB", res.Name)
	require.Equal(t, model.WalletTypeCredit, res.Type)
	require.Equal(t, -2_000_000.0, res.Balance)
	require.Equal(t, "💳", res.Icon)
	require.False(t, res.IsIncludedInTotal)
	require.Equal(t, 20_000_000.0, *res.CreditLimit)
	require.Equal(t, 18_000_000.0, *res.AvailableCredit)
	require.Equal(t, 20, *wallet.StatementDay)
	require.Equal(t, 5, *wallet.DueDay)
	require.Equal(t, "NEC", *wallet.JarCategory)
	require.Equal(t, 100_000.0, *wallet.MinimumPayment)
	require.Equal(t, userID, wallet.UserID, "owner never changes")
	// Allocation 0 never needs the sum of the other wallets.
	repo.AssertNotCalled(t, "SumAllocationPercent", mock.Anything, mock.Anything, mock.Anything)
	// Balance was sent, so it is written.
	repo.AssertCalled(t, "Update", mock.Anything, wallet, true)
}

func TestWalletService_UpdateWalletErrors(t *testing.T) {
	userID, walletID := uuid.New(), uuid.New()

	t.Run("not found or not owned", func(t *testing.T) {
		repo := new(MockWalletRepo)
		repo.On("GetByIDForUser", mock.Anything, walletID, userID).Return(nil, repository.ErrWalletNotFound)
		_, err := service.NewWalletService(repo).UpdateWallet(context.Background(), userID, walletID, dto.UpdateWalletRequest{Name: ptr("x")})
		require.ErrorIs(t, err, service.ErrWalletNotFound)
		repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("lookup error", func(t *testing.T) {
		repo := new(MockWalletRepo)
		repo.On("GetByIDForUser", mock.Anything, walletID, userID).Return(nil, errRepoDown)
		_, err := service.NewWalletService(repo).UpdateWallet(context.Background(), userID, walletID, dto.UpdateWalletRequest{})
		require.ErrorIs(t, err, errRepoDown)
	})

	for _, pct := range []float64{-1, 100.5, math.NaN()} {
		t.Run("invalid allocation percent", func(t *testing.T) {
			repo := new(MockWalletRepo)
			repo.On("GetByIDForUser", mock.Anything, walletID, userID).Return(&model.Wallet{ID: walletID, UserID: userID}, nil)
			_, err := service.NewWalletService(repo).UpdateWallet(context.Background(), userID, walletID, dto.UpdateWalletRequest{AllocationPercent: ptr(pct)})
			require.ErrorIs(t, err, service.ErrInvalidAllocationPercent)
			require.ErrorIs(t, err, constant.ErrInvalidInput)
			repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
		})
	}

	t.Run("update error", func(t *testing.T) {
		repo := new(MockWalletRepo)
		w := &model.Wallet{ID: walletID, UserID: userID}
		repo.On("GetByIDForUser", mock.Anything, walletID, userID).Return(w, nil)
		repo.On("Update", mock.Anything, w, false).Return(errRepoDown)
		_, err := service.NewWalletService(repo).UpdateWallet(context.Background(), userID, walletID, dto.UpdateWalletRequest{Name: ptr("x")})
		require.ErrorIs(t, err, errRepoDown)
	})
}

func TestWalletService_DeleteWallet(t *testing.T) {
	userID, walletID := uuid.New(), uuid.New()

	t.Run("not owned is not found and nothing is deleted", func(t *testing.T) {
		repo := new(MockWalletRepo)
		repo.On("GetByIDForUser", mock.Anything, walletID, userID).Return(nil, repository.ErrWalletNotFound)
		err := service.NewWalletService(repo).DeleteWallet(context.Background(), userID, walletID)
		require.ErrorIs(t, err, service.ErrWalletNotFound)
		repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("lookup error", func(t *testing.T) {
		repo := new(MockWalletRepo)
		repo.On("GetByIDForUser", mock.Anything, walletID, userID).Return(nil, errRepoDown)
		require.ErrorIs(t, service.NewWalletService(repo).DeleteWallet(context.Background(), userID, walletID), errRepoDown)
		repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("owner deletes with user scope", func(t *testing.T) {
		repo := new(MockWalletRepo)
		repo.On("GetByIDForUser", mock.Anything, walletID, userID).Return(&model.Wallet{ID: walletID, UserID: userID}, nil)
		repo.On("Delete", mock.Anything, walletID, userID).Return(nil)
		require.NoError(t, service.NewWalletService(repo).DeleteWallet(context.Background(), userID, walletID))
		repo.AssertCalled(t, "Delete", mock.Anything, walletID, userID)
	})
}

func TestWalletService_AutoAllocateIncomePassesUserAndRequest(t *testing.T) {
	repo := new(MockWalletRepo)
	userID := uuid.New()
	req := dto.AutoAllocateRequest{SourceWalletID: uuid.New(), IncomeAmount: 20_000_000, Preset: "50_30_20"}
	target := uuid.New()
	alloc := &repository.AutoAllocation{TotalAllocated: 20_000_000, Allocations: []repository.WalletAllocation{
		{WalletID: target, WalletName: "Tiết kiệm", AllocationPercent: 100, AllocatedAmount: 20_000_000, NewBalance: 25_000_000},
	}}
	repo.On("AutoAllocateIncome", mock.Anything, userID, req.SourceWalletID, req.IncomeAmount).Return(alloc, nil)

	got, err := service.NewWalletService(repo).AutoAllocateIncome(context.Background(), userID, req)
	require.NoError(t, err)
	assert.Equal(t, &dto.AutoAllocateResponse{
		SourceWalletID: &req.SourceWalletID, IncomeAmount: 20_000_000, TotalAllocated: 20_000_000,
		Allocations: []dto.AllocatedWalletDetail{{WalletID: target, WalletName: "Tiết kiệm", AllocationPercent: 100, AllocatedAmount: 20_000_000, NewBalance: 25_000_000}},
	}, got)
}

func TestWalletService_TransferMoneyValidationAndDate(t *testing.T) {
	userID, a, b := uuid.New(), uuid.New(), uuid.New()

	invalid := []struct {
		name string
		req  dto.TransferMoneyRequest
		want error
	}{
		{"same wallet", dto.TransferMoneyRequest{FromWalletID: a, ToWalletID: a, Amount: 1}, service.ErrSameWalletTransfer},
		{"zero amount", dto.TransferMoneyRequest{FromWalletID: a, ToWalletID: b, Amount: 0}, service.ErrInvalidTransfer},
		{"negative amount", dto.TransferMoneyRequest{FromWalletID: a, ToWalletID: b, Amount: -10}, service.ErrInvalidTransfer},
		{"NaN amount", dto.TransferMoneyRequest{FromWalletID: a, ToWalletID: b, Amount: math.NaN()}, service.ErrInvalidTransfer},
		{"NaN fee", dto.TransferMoneyRequest{FromWalletID: a, ToWalletID: b, Amount: 10, Fee: math.NaN()}, service.ErrInvalidTransfer},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockWalletRepo)
			_, err := service.NewWalletService(repo).TransferMoney(context.Background(), userID, tc.req)
			require.ErrorIs(t, err, tc.want)
			repo.AssertNotCalled(t, "TransferMoney", mock.Anything, mock.Anything)
		})
	}

	t.Run("explicit date and owner", func(t *testing.T) {
		repo := new(MockWalletRepo)
		date := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
		repo.On("TransferMoney", mock.Anything, mock.MatchedBy(func(wt *model.WalletTransfer) bool {
			return wt.UserID == userID && wt.TransferDate.Equal(date) && wt.Fee == 3_300
		})).Return(nil)
		res, err := service.NewWalletService(repo).TransferMoney(context.Background(), userID,
			dto.TransferMoneyRequest{FromWalletID: a, ToWalletID: b, Amount: 1_000_000, Fee: 3_300, TransferDate: &dto.CustomTime{Time: date}})
		require.NoError(t, err)
		require.Equal(t, "2026-10-31", res.TransferDate)
		require.Equal(t, 3_300.0, res.Fee)
	})

	t.Run("default date is today in the app timezone", func(t *testing.T) {
		repo := new(MockWalletRepo)
		var got time.Time
		repo.On("TransferMoney", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			got = args.Get(1).(*model.WalletTransfer).TransferDate
		}).Return(nil)
		_, err := service.NewWalletService(repo).TransferMoney(context.Background(), userID, dto.TransferMoneyRequest{FromWalletID: a, ToWalletID: b, Amount: 1})
		require.NoError(t, err)
		require.Equal(t, util.AppLocation(), got.Location())
	})

	t.Run("repository error is returned", func(t *testing.T) {
		repo := new(MockWalletRepo)
		repo.On("TransferMoney", mock.Anything, mock.Anything).Return(constant.ErrInsufficientBalance)
		_, err := service.NewWalletService(repo).TransferMoney(context.Background(), userID, dto.TransferMoneyRequest{FromWalletID: a, ToWalletID: b, Amount: 1})
		require.ErrorIs(t, err, service.ErrInsufficientBalance)
	})
}

// ------------------------------- debts -------------------------------------

func TestDebtService_GetDebtsMarksOverdue(t *testing.T) {
	repo := new(MockDebtRepo)
	userID := uuid.New()
	past := util.Now().AddDate(0, 0, -3)
	future := util.Now().AddDate(0, 1, 0)
	debts := []model.Debt{
		{ID: uuid.New(), UserID: userID, Title: "quá hạn", TotalAmount: 100, PaidAmount: 40, Status: model.DebtStatusPending, DueDate: &past},
		{ID: uuid.New(), UserID: userID, Title: "còn hạn", TotalAmount: 100, Status: model.DebtStatusPending, DueDate: &future},
		{ID: uuid.New(), UserID: userID, Title: "xong", TotalAmount: 100, PaidAmount: 100, Status: model.DebtStatusCompleted, DueDate: &past},
		{ID: uuid.New(), UserID: userID, Title: "không hạn", TotalAmount: 100, PaidAmount: 120, Status: model.DebtStatusPending},
	}
	repo.On("FindByUserID", mock.Anything, userID, model.DebtTypePayable, model.DebtStatus("")).Return(debts, nil)

	res, err := service.NewDebtService(repo).GetDebts(context.Background(), userID, model.DebtTypePayable, "")
	require.NoError(t, err)
	require.Len(t, res, 4)
	assert.Equal(t, model.DebtStatusOverdue, res[0].Status)
	assert.Equal(t, 60.0, res[0].Remaining)
	assert.Equal(t, model.DebtStatusPending, res[1].Status)
	assert.Equal(t, model.DebtStatusCompleted, res[2].Status, "a completed debt is never overdue")
	assert.Equal(t, model.DebtStatusPending, res[3].Status)
	assert.Equal(t, 0.0, res[3].Remaining, "remaining never goes negative")
	assert.Equal(t, model.DebtStatusPending, debts[0].Status, "repository rows are not mutated")
}

func TestDebtService_GetDebtsAndSummaryErrors(t *testing.T) {
	repo := new(MockDebtRepo)
	userID := uuid.New()
	repo.On("FindByUserID", mock.Anything, userID, model.DebtType(""), model.DebtStatus("")).Return([]model.Debt(nil), errRepoDown)
	_, err := service.NewDebtService(repo).GetDebts(context.Background(), userID, "", "")
	require.ErrorIs(t, err, errRepoDown)

	repo.On("GetSummaryByUserID", mock.Anything, userID).Return(&repository.DebtSummary{TotalPayable: 5, TotalReceivable: 7, OverdueCount: 1, PendingCount: 2}, nil)
	got, err := service.NewDebtService(repo).GetDebtSummary(context.Background(), userID)
	require.NoError(t, err)
	assert.Equal(t, &dto.DebtSummaryResponse{TotalPayable: 5, TotalReceivable: 7, OverdueCount: 1, PendingCount: 2}, got)
}

func TestDebtService_AddRepaymentNotFoundAndRepoError(t *testing.T) {
	userID, debtID := uuid.New(), uuid.New()

	repo := new(MockDebtRepo)
	repo.On("GetByIDForUser", mock.Anything, debtID, userID).Return(nil, repository.ErrDebtNotFound)
	_, err := service.NewDebtService(repo).AddRepayment(context.Background(), userID, debtID, dto.AddRepaymentRequest{Amount: 1})
	require.ErrorIs(t, err, service.ErrDebtNotFound)

	repo = new(MockDebtRepo)
	debt := &model.Debt{ID: debtID, UserID: userID, Type: model.DebtTypePayable, TotalAmount: 1000, PaidAmount: 0, Status: model.DebtStatusPending}
	repo.On("GetByIDForUser", mock.Anything, debtID, userID).Return(debt, nil)
	repo.On("AddRepaymentWithWallet", mock.Anything, mock.Anything, mock.Anything, (*uuid.UUID)(nil), 0.0).Return(errRepoDown)
	_, err = service.NewDebtService(repo).AddRepayment(context.Background(), userID, debtID, dto.AddRepaymentRequest{Amount: 400})
	require.ErrorIs(t, err, errRepoDown)
	require.Equal(t, 0.0, debt.PaidAmount, "a failed write must not change the loaded debt")
	require.Equal(t, model.DebtStatusPending, debt.Status)
}

func TestDebtService_CreateDebtPastDueIsOverdueAndWithoutWalletHasNoDelta(t *testing.T) {
	repo := new(MockDebtRepo)
	userID := uuid.New()
	past := util.Now().AddDate(0, 0, -1)
	repo.On("CreateWithWallet", mock.Anything, mock.MatchedBy(func(d *model.Debt) bool {
		return d.UserID == userID && d.Status == model.DebtStatusOverdue && d.StartDate != nil
	}), (*uuid.UUID)(nil), 0.0).Return(nil)

	res, err := service.NewDebtService(repo).CreateDebt(context.Background(), userID, dto.CreateDebtRequest{
		Type: model.DebtTypePayable, Title: "Vay", TotalAmount: 1_000_000, DueDate: &dto.CustomTime{Time: past},
	})
	require.NoError(t, err)
	require.Equal(t, model.DebtStatusOverdue, res.Status)
	require.Equal(t, 1_000_000.0, res.Remaining)

	repo2 := new(MockDebtRepo)
	repo2.On("CreateWithWallet", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errRepoDown)
	_, err = service.NewDebtService(repo2).CreateDebt(context.Background(), userID, dto.CreateDebtRequest{Type: model.DebtTypePayable, Title: "x", TotalAmount: 1})
	require.ErrorIs(t, err, errRepoDown)
}

// --------------------------- credit card statements ------------------------

func TestCreditCardStatement_CreateValidatesWalletAndDates(t *testing.T) {
	userID, walletID := uuid.New(), uuid.New()
	future := util.Now().AddDate(0, 1, 0).Format("2006-01-02")
	base := dto.CreateStatementRequest{WalletID: walletID, StatementMonth: 10, StatementYear: 2026, StatementDate: "2026-10-20", DueDate: future, StatementBalance: 1_000_000}

	t.Run("wallet of another user", func(t *testing.T) {
		stmts, wallets := new(MockStatementRepo), new(MockWalletRepo)
		wallets.On("GetByIDForUser", mock.Anything, walletID, userID).Return(nil, repository.ErrWalletNotFound)
		_, err := service.NewCreditCardStatementService(stmts, wallets).CreateStatement(context.Background(), userID, base)
		require.ErrorIs(t, err, service.ErrWalletNotFound)
		stmts.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("wallet lookup error", func(t *testing.T) {
		stmts, wallets := new(MockStatementRepo), new(MockWalletRepo)
		wallets.On("GetByIDForUser", mock.Anything, walletID, userID).Return(nil, errRepoDown)
		_, err := service.NewCreditCardStatementService(stmts, wallets).CreateStatement(context.Background(), userID, base)
		require.ErrorIs(t, err, errRepoDown)
	})

	for _, tc := range []struct {
		name          string
		stmtDate, due string
	}{
		{"bad statement date", "20/10/2026", future},
		{"bad due date", "2026-10-20", "2026-13-01"},
		{"empty due date", "2026-10-20", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stmts, wallets := new(MockStatementRepo), new(MockWalletRepo)
			wallets.On("GetByIDForUser", mock.Anything, walletID, userID).Return(&model.Wallet{ID: walletID, UserID: userID}, nil)
			req := base
			req.StatementDate, req.DueDate = tc.stmtDate, tc.due
			_, err := service.NewCreditCardStatementService(stmts, wallets).CreateStatement(context.Background(), userID, req)
			require.ErrorIs(t, err, service.ErrInvalidDate)
			stmts.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
			wallets.AssertNotCalled(t, "UpdateStatementSnapshot", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestCreditCardStatement_CreateMinimumPaymentAndStatus(t *testing.T) {
	userID, walletID := uuid.New(), uuid.New()
	futureRFC := util.Now().AddDate(0, 1, 0).UTC().Format(time.RFC3339)

	cases := []struct {
		name       string
		balance    float64
		minPay     float64
		paid       float64
		wantMinPay float64
		wantStatus model.StatementStatus
	}{
		{"5% of balance", 5_000_000, 0, 0, 250_000, model.StatementStatusUnpaid},
		{"floor of 50k when balance >= 50k", 600_000, 0, 0, 50_000, model.StatementStatusUnpaid},
		{"small balance keeps 5%", 40_000, 0, 0, 2_000, model.StatementStatusUnpaid},
		{"explicit minimum kept", 5_000_000, 300_000, 0, 300_000, model.StatementStatusUnpaid},
		{"partially paid", 1_000_000, 0, 200_000, 50_000, model.StatementStatusPartiallyPaid},
		{"fully paid", 1_000_000, 0, 1_000_000, 50_000, model.StatementStatusPaid},
		{"zero balance", 0, 0, 0, 0, model.StatementStatusUnpaid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stmts, wallets := new(MockStatementRepo), new(MockWalletRepo)
			wallet := &model.Wallet{ID: walletID, UserID: userID, Name: "Thẻ ACB", Icon: "💳"}
			wallets.On("GetByIDForUser", mock.Anything, walletID, userID).Return(wallet, nil)
			wallets.On("UpdateStatementSnapshot", mock.Anything, walletID, userID, tc.balance, tc.wantMinPay, 0.0).Return(nil)
			stmts.On("Create", mock.Anything, mock.MatchedBy(func(s *model.CreditCardStatement) bool {
				return s.UserID == userID && s.WalletID == walletID && s.Status == tc.wantStatus
			})).Return(nil)

			res, err := service.NewCreditCardStatementService(stmts, wallets).CreateStatement(context.Background(), userID, dto.CreateStatementRequest{
				WalletID: walletID, StatementMonth: 10, StatementYear: 2026, StatementDate: "2026-10-20T00:00:00Z", DueDate: futureRFC,
				StatementBalance: tc.balance, MinimumPayment: tc.minPay, PaidAmount: tc.paid, Note: ptr("ghi chú"),
			})
			require.NoError(t, err)
			require.InDelta(t, tc.wantMinPay, res.MinimumPayment, 0.001)
			require.Equal(t, tc.wantStatus, res.Status)
			require.Equal(t, "ghi chú", res.Note)
			require.Equal(t, "Thẻ ACB", res.WalletName)
			require.Equal(t, tc.wantMinPay, *wallet.MinimumPayment, "wallet snapshot synced")
		})
	}
}

func TestCreditCardStatement_CreateSnapshotErrorIsReturned(t *testing.T) {
	userID, walletID := uuid.New(), uuid.New()
	stmts, wallets := new(MockStatementRepo), new(MockWalletRepo)
	wallets.On("GetByIDForUser", mock.Anything, walletID, userID).Return(&model.Wallet{ID: walletID, UserID: userID}, nil)
	stmts.On("Create", mock.Anything, mock.Anything).Return(nil)
	wallets.On("UpdateStatementSnapshot", mock.Anything, walletID, userID, mock.Anything, mock.Anything, mock.Anything).Return(errRepoDown)

	_, err := service.NewCreditCardStatementService(stmts, wallets).CreateStatement(context.Background(), userID, dto.CreateStatementRequest{
		WalletID: walletID, StatementDate: "2026-10-20", DueDate: "2026-11-05", StatementBalance: 1,
	})
	require.ErrorIs(t, err, errRepoDown)

	stmts2 := new(MockStatementRepo)
	stmts2.On("Create", mock.Anything, mock.Anything).Return(errRepoDown)
	wallets2 := new(MockWalletRepo)
	wallets2.On("GetByIDForUser", mock.Anything, walletID, userID).Return(&model.Wallet{ID: walletID, UserID: userID}, nil)
	_, err = service.NewCreditCardStatementService(stmts2, wallets2).CreateStatement(context.Background(), userID, dto.CreateStatementRequest{
		WalletID: walletID, StatementDate: "2026-10-20", DueDate: "2026-11-05", StatementBalance: 1,
	})
	require.ErrorIs(t, err, errRepoDown)
	wallets2.AssertNotCalled(t, "UpdateStatementSnapshot", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCreditCardStatement_GetByID(t *testing.T) {
	userID, id := uuid.New(), uuid.New()
	stmts := new(MockStatementRepo)
	svc := service.NewCreditCardStatementService(stmts, new(MockWalletRepo))

	stmts.On("GetByIDForUser", mock.Anything, id, userID).Return(nil, repository.ErrStatementNotFound).Once()
	_, err := svc.GetStatementByID(context.Background(), userID, id)
	require.ErrorIs(t, err, service.ErrStatementNotFound)

	stmts.On("GetByIDForUser", mock.Anything, id, userID).Return(nil, errRepoDown).Once()
	_, err = svc.GetStatementByID(context.Background(), userID, id)
	require.ErrorIs(t, err, errRepoDown)

	due := util.Now().AddDate(0, 0, 10)
	stmts.On("GetByIDForUser", mock.Anything, id, userID).Return(&model.CreditCardStatement{
		ID: id, UserID: userID, StatementBalance: 1_000, PaidAmount: 300, Status: model.StatementStatusPartiallyPaid, DueDate: due,
	}, nil).Once()
	res, err := svc.GetStatementByID(context.Background(), userID, id)
	require.NoError(t, err)
	require.Equal(t, 700.0, res.RemainingAmount)
	require.Equal(t, model.StatementStatusPartiallyPaid, res.Status)
}

func TestCreditCardStatement_ListComputesTotals(t *testing.T) {
	userID, walletID := uuid.New(), uuid.New()
	year, month := 2026, 10
	due := util.Now().AddDate(0, 0, 10)
	stmts := new(MockStatementRepo)
	stmts.On("List", mock.Anything, userID, &walletID, &year, &month).Return([]model.CreditCardStatement{
		{ID: uuid.New(), UserID: userID, StatementBalance: 1_000_000, PaidAmount: 400_000, Status: model.StatementStatusPartiallyPaid, DueDate: due},
		{ID: uuid.New(), UserID: userID, StatementBalance: 500_000, PaidAmount: 600_000, Status: model.StatementStatusPaid, DueDate: due},
	}, nil)
	svc := service.NewCreditCardStatementService(stmts, new(MockWalletRepo))

	res, err := svc.ListStatements(context.Background(), userID, &walletID, &year, &month)
	require.NoError(t, err)
	require.Equal(t, int64(2), res.Total)
	require.Equal(t, 1_500_000.0, res.TotalBalance)
	require.Equal(t, 1_000_000.0, res.TotalPaid)
	require.Equal(t, 600_000.0, res.TotalRemaining, "overpaid statement contributes 0 remaining")

	other := uuid.New()
	stmts.On("List", mock.Anything, other, (*uuid.UUID)(nil), (*int)(nil), (*int)(nil)).Return([]model.CreditCardStatement{}, nil)
	res, err = svc.ListStatements(context.Background(), other, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, res.Items)
	require.Zero(t, res.Total)

	bad := uuid.New()
	stmts.On("List", mock.Anything, bad, (*uuid.UUID)(nil), (*int)(nil), (*int)(nil)).Return([]model.CreditCardStatement(nil), errRepoDown)
	_, err = svc.ListStatements(context.Background(), bad, nil, nil, nil)
	require.ErrorIs(t, err, errRepoDown)
}

func TestCreditCardStatement_UpdateRecomputesStatus(t *testing.T) {
	userID, id := uuid.New(), uuid.New()
	origStmtDate := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	origDue := util.Now().AddDate(0, 1, 0)
	newDue := util.Now().AddDate(0, 2, 0).Format("2006-01-02")
	paidStatus := model.StatementStatusPaid

	cases := []struct {
		name       string
		req        dto.UpdateStatementRequest
		wantStatus model.StatementStatus
		check      func(t *testing.T, s *model.CreditCardStatement)
	}{
		{
			name:       "status in the request is ignored when nothing was paid",
			req:        dto.UpdateStatementRequest{Status: &paidStatus},
			wantStatus: model.StatementStatusUnpaid,
		},
		{
			name:       "partial payment",
			req:        dto.UpdateStatementRequest{PaidAmount: ptr(100_000.0), Note: ptr("đã trả một phần")},
			wantStatus: model.StatementStatusPartiallyPaid,
			check: func(t *testing.T, s *model.CreditCardStatement) {
				require.Equal(t, "đã trả một phần", s.Note)
			},
		},
		{
			name: "full payment and all fields",
			req: dto.UpdateStatementRequest{
				StatementMonth: ptr(11), StatementYear: ptr(2027), StatementDate: ptr("2026-11-20"), DueDate: ptr(newDue),
				StatementBalance: ptr(2_000_000.0), MinimumPayment: ptr(100_000.0), PreviousBalance: ptr(50_000.0), PaidAmount: ptr(2_000_000.0),
			},
			wantStatus: model.StatementStatusPaid,
			check: func(t *testing.T, s *model.CreditCardStatement) {
				require.Equal(t, 11, s.StatementMonth)
				require.Equal(t, 2027, s.StatementYear)
				require.Equal(t, "2026-11-20", s.StatementDate.Format("2006-01-02"))
				require.Equal(t, newDue, s.DueDate.Format("2006-01-02"))
				require.Equal(t, 100_000.0, s.MinimumPayment)
				require.Equal(t, 50_000.0, s.PreviousBalance)
			},
		},
		{
			name: "paid amount left out keeps the locked (concurrently paid) value",
			req:  dto.UpdateStatementRequest{Note: ptr("ghi chú")},
			// stored.PaidAmount is set below to what a concurrent PayStatement committed.
			wantStatus: model.StatementStatusPartiallyPaid,
			check: func(t *testing.T, s *model.CreditCardStatement) {
				require.Equal(t, 300_000.0, s.PaidAmount)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stored := &model.CreditCardStatement{ID: id, UserID: userID, StatementBalance: 1_000_000, Status: model.StatementStatusUnpaid,
				StatementDate: origStmtDate, DueDate: origDue}
			if tc.req.PaidAmount == nil && tc.req.Note != nil {
				stored.PaidAmount = 300_000
			}
			stmts := new(MockStatementRepo)
			stmts.On("Update", mock.Anything, userID, id).Return(stored, nil)

			res, err := service.NewCreditCardStatementService(stmts, new(MockWalletRepo)).UpdateStatement(context.Background(), userID, id, tc.req)
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, res.Status)
			require.Equal(t, tc.wantStatus, stored.Status)
			require.Equal(t, userID, stored.UserID)
			if tc.check != nil {
				tc.check(t, stored)
			}
		})
	}
}

func TestCreditCardStatement_UpdateAndDeleteErrors(t *testing.T) {
	userID, id := uuid.New(), uuid.New()

	stmts := new(MockStatementRepo)
	stmts.On("Update", mock.Anything, userID, id).Return(nil, constant.ErrNotFound)
	_, err := service.NewCreditCardStatementService(stmts, new(MockWalletRepo)).UpdateStatement(context.Background(), userID, id, dto.UpdateStatementRequest{})
	require.ErrorIs(t, err, service.ErrStatementNotFound)

	// Invalid dates are rejected (like CreateStatement) before anything is locked or written.
	for _, req := range []dto.UpdateStatementRequest{
		{StatementDate: ptr("not-a-date")},
		{DueDate: ptr("31/12/2026")},
	} {
		stmts = new(MockStatementRepo)
		_, err = service.NewCreditCardStatementService(stmts, new(MockWalletRepo)).UpdateStatement(context.Background(), userID, id, req)
		require.ErrorIs(t, err, service.ErrInvalidDate)
		stmts.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
	}

	stmts = new(MockStatementRepo)
	stmts.On("Update", mock.Anything, userID, id).Return(nil, fmt.Errorf("%w: credit card statement for this period", constant.ErrAlreadyExists))
	_, err = service.NewCreditCardStatementService(stmts, new(MockWalletRepo)).UpdateStatement(context.Background(), userID, id, dto.UpdateStatementRequest{StatementMonth: ptr(10)})
	require.ErrorIs(t, err, constant.ErrAlreadyExists)

	stmts = new(MockStatementRepo)
	stmts.On("Update", mock.Anything, userID, id).Return(nil, errRepoDown)
	_, err = service.NewCreditCardStatementService(stmts, new(MockWalletRepo)).UpdateStatement(context.Background(), userID, id, dto.UpdateStatementRequest{})
	require.ErrorIs(t, err, errRepoDown)

	stmts = new(MockStatementRepo)
	stmts.On("Delete", mock.Anything, id, userID).Return(nil)
	require.NoError(t, service.NewCreditCardStatementService(stmts, new(MockWalletRepo)).DeleteStatement(context.Background(), userID, id))
	stmts.AssertCalled(t, "Delete", mock.Anything, id, userID)

	stmts = new(MockStatementRepo)
	stmts.On("Delete", mock.Anything, id, userID).Return(errors.New("statement not found"))
	require.Error(t, service.NewCreditCardStatementService(stmts, new(MockWalletRepo)).DeleteStatement(context.Background(), userID, id))
}

// ------------------------------- targets -----------------------------------

func TestTargetService_UpsertTarget(t *testing.T) {
	userID := uuid.New()

	invalid := []dto.UpsertTargetRequest{
		{TargetType: "SAVINGS", TargetAmount: 1, Month: 1, Year: 2026},
		{TargetType: "EXPENSE", TargetAmount: 0, Month: 1, Year: 2026},
		{TargetType: "EXPENSE", TargetAmount: -5, Month: 1, Year: 2026},
		{TargetType: "EXPENSE", TargetAmount: 10_000_000_000, Month: 1, Year: 2026},
		{TargetType: "EXPENSE", TargetAmount: 1, Month: 13, Year: 2026},
		{TargetType: "INVESTMENT", TargetAmount: 1, Month: 1, Year: 1999},
	}
	for _, req := range invalid {
		repo := new(MockTargetRepo)
		err := service.NewTargetService(repo).UpsertTarget(context.Background(), userID, &req)
		require.ErrorIs(t, err, constant.ErrInvalidInput, "%+v", req)
		repo.AssertNotCalled(t, "UpsertTarget", mock.Anything, mock.Anything)
	}

	repo := new(MockTargetRepo)
	repo.On("UpsertTarget", mock.Anything, mock.MatchedBy(func(m *model.MonthlyTarget) bool {
		return m.UserID == userID && m.TargetType == model.TargetTypeInvestment && m.TargetAmount == 5_000_000 && m.Month == 12 && m.Year == 2026
	})).Return(nil)
	require.NoError(t, service.NewTargetService(repo).UpsertTarget(context.Background(), userID,
		&dto.UpsertTargetRequest{TargetType: "INVESTMENT", TargetAmount: 5_000_000, Month: 12, Year: 2026}))

	repo = new(MockTargetRepo)
	repo.On("UpsertTarget", mock.Anything, mock.Anything).Return(errRepoDown)
	require.ErrorIs(t, service.NewTargetService(repo).UpsertTarget(context.Background(), userID,
		&dto.UpsertTargetRequest{TargetType: "EXPENSE", TargetAmount: 1, Month: 1, Year: 2026}), errRepoDown)
}
