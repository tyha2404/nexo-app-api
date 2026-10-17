package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/service"
)

type transactionStub struct {
	repository.TransactionRepository
	stored       *model.Transaction
	summary      float64
	summaryCalls int
}

func (r *transactionStub) Create(_ context.Context, tx *model.Transaction) error {
	r.stored = tx
	tx.ID = uuid.New()
	return nil
}
func (r *transactionStub) GetByID(_ context.Context, _ uuid.UUID) (*model.Transaction, error) {
	return r.stored, nil
}
func (r *transactionStub) Update(_ context.Context, tx *model.Transaction) error {
	r.stored = tx
	return nil
}
func (r *transactionStub) GetSummaryByUserID(_ context.Context, _ uuid.UUID, _ map[string]interface{}) (float64, float64, int64, float64, int64, float64, error) {
	r.summaryCalls++
	return r.summary, r.summary, 1500, 0, 0, 0, nil
}

type categoryStub struct {
	repository.CategoryRepo
	category *model.Category
}

// GetByIDForUser emulates the user-scoped SQL (WHERE id = ? AND user_id = ?).
func (r *categoryStub) GetByIDForUser(_ context.Context, userID, id uuid.UUID) (*model.Category, error) {
	if r.category == nil || r.category.ID != id || r.category.UserID != userID {
		return nil, constant.ErrNotFound
	}
	return r.category, nil
}

type budgetStub struct {
	repository.BudgetRepository
	budget *model.Budget
}

func (r *budgetStub) GetByCategoryAndPeriod(_ context.Context, _, _ uuid.UUID, _ time.Time) (*model.Budget, error) {
	return r.budget, nil
}

type alertStub struct {
	repository.AlertRepository
	alerts []*model.Alert
}

func (r *alertStub) Create(_ context.Context, a *model.Alert) (bool, error) {
	r.alerts = append(r.alerts, a)
	return true, nil
}

func TestCreateInvestmentPreservesStatusAndPnL(t *testing.T) {
	for _, status := range []string{"HOLDING", "SOLD", "MATURED", "CANCELLED"} {
		t.Run(status, func(t *testing.T) {
			userID, categoryID := uuid.New(), uuid.New()
			tx := &transactionStub{}
			cat := &categoryStub{category: &model.Category{ID: categoryID, UserID: userID, Type: model.CategoryType("INVESTMENT")}}
			pnl := -500.0
			result, err := service.NewTransactionService(tx, cat, nil, nil, nil).CreateTransaction(context.Background(), userID, dto.CreateTransactionRequest{CategoryID: categoryID, Amount: 1000, Type: "INVESTMENT", Status: &status, RealizedPnL: &pnl})
			require.NoError(t, err)
			require.Equal(t, status, *result.Status)
			expected := pnl
			if status == "HOLDING" {
				expected = 0
			}
			require.Equal(t, expected, *result.RealizedPnL)
		})
	}
}
func TestUpdateReceiptCanClearAndOmissionPreservesIt(t *testing.T) {
	userID := uuid.New()
	url := "https://example.com/receipt.jpg"
	repo := &transactionStub{stored: &model.Transaction{ID: uuid.New(), UserID: userID, Type: model.TransactionTypeIncome, ReceiptURL: &url}}
	svc := service.NewTransactionService(repo, nil, nil, nil, nil)
	result, err := svc.UpdateTransaction(context.Background(), userID, repo.stored.ID, dto.UpdateTransactionRequest{})
	require.NoError(t, err)
	require.Equal(t, url, *result.ReceiptURL)
	empty := ""
	result, err = svc.UpdateTransaction(context.Background(), userID, repo.stored.ID, dto.UpdateTransactionRequest{ReceiptURL: &empty})
	require.NoError(t, err)
	require.Empty(t, *result.ReceiptURL)
}
func TestBudgetAlertUsesCompleteAggregate(t *testing.T) {
	userID, categoryID := uuid.New(), uuid.New()
	repo := &transactionStub{summary: 1500}
	cat := &categoryStub{category: &model.Category{ID: categoryID, UserID: userID, Type: model.CategoryType("EXPENSE")}}
	budgets := &budgetStub{budget: &model.Budget{ID: uuid.New(), Amount: 1200, PeriodType: "monthly"}}
	alerts := &alertStub{}
	_, err := service.NewTransactionService(repo, cat, budgets, alerts, nil).CreateTransaction(context.Background(), userID, dto.CreateTransactionRequest{CategoryID: categoryID, Type: "EXPENSE", Amount: 1})
	require.NoError(t, err)
	require.Equal(t, 1, repo.summaryCalls)
	require.Len(t, alerts.alerts, 1)
	require.Equal(t, "over_limit", alerts.alerts[0].AlertType)
}

type walletOwnerStub struct {
	repository.WalletRepository
	owned map[uuid.UUID]uuid.UUID // walletID -> userID
	calls int
}

func (r *walletOwnerStub) GetByID(_ context.Context, id, userID uuid.UUID) (*model.Wallet, error) {
	r.calls++
	if owner, ok := r.owned[id]; ok && owner == userID {
		return &model.Wallet{ID: id, UserID: userID}, nil
	}
	return nil, nil
}

func TestCreateTransactionRejectsForeignWallet(t *testing.T) {
	userID, otherUser, categoryID, walletID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &transactionStub{}
	cat := &categoryStub{category: &model.Category{ID: categoryID, UserID: userID, Type: model.CategoryType("INCOME")}}
	wallets := &walletOwnerStub{owned: map[uuid.UUID]uuid.UUID{walletID: otherUser}}

	_, err := service.NewTransactionService(repo, cat, nil, nil, wallets).CreateTransaction(context.Background(), userID,
		dto.CreateTransactionRequest{CategoryID: categoryID, WalletID: &walletID, Type: "INCOME", Amount: 100})
	require.ErrorIs(t, err, constant.ErrNotFound)
	require.Nil(t, repo.stored, "transaction must not be written when the wallet is not the user's")
}

func TestCreateTransactionWithOwnWalletAndWithoutWallet(t *testing.T) {
	userID, categoryID, walletID := uuid.New(), uuid.New(), uuid.New()
	cat := &categoryStub{category: &model.Category{ID: categoryID, UserID: userID, Type: model.CategoryType("INCOME")}}
	wallets := &walletOwnerStub{owned: map[uuid.UUID]uuid.UUID{walletID: userID}}

	repo := &transactionStub{}
	res, err := service.NewTransactionService(repo, cat, nil, nil, wallets).CreateTransaction(context.Background(), userID,
		dto.CreateTransactionRequest{CategoryID: categoryID, WalletID: &walletID, Type: "INCOME", Amount: 100})
	require.NoError(t, err)
	require.Equal(t, walletID, *res.WalletID)

	// No wallet: backward compatible, wallet repo never consulted.
	repo = &transactionStub{}
	wallets.calls = 0
	_, err = service.NewTransactionService(repo, cat, nil, nil, wallets).CreateTransaction(context.Background(), userID,
		dto.CreateTransactionRequest{CategoryID: categoryID, Type: "INCOME", Amount: 100})
	require.NoError(t, err)
	require.Zero(t, wallets.calls)
}

func TestUpdateTransactionChecksOnlyChangedWallet(t *testing.T) {
	userID, walletA, walletB := uuid.New(), uuid.New(), uuid.New()
	repo := &transactionStub{stored: &model.Transaction{ID: uuid.New(), UserID: userID, Type: model.TransactionTypeIncome, Amount: 10, WalletID: &walletA}}
	wallets := &walletOwnerStub{owned: map[uuid.UUID]uuid.UUID{walletA: userID}}
	svc := service.NewTransactionService(repo, nil, nil, nil, wallets)

	same := walletA
	_, err := svc.UpdateTransaction(context.Background(), userID, repo.stored.ID, dto.UpdateTransactionRequest{WalletID: &same})
	require.NoError(t, err)
	require.Zero(t, wallets.calls)

	_, err = svc.UpdateTransaction(context.Background(), userID, repo.stored.ID, dto.UpdateTransactionRequest{WalletID: &walletB})
	require.ErrorIs(t, err, constant.ErrNotFound)
	require.Equal(t, walletA, *repo.stored.WalletID)
}

func TestTransactionRejectsForeignCategory(t *testing.T) {
	userID, otherUser, categoryID := uuid.New(), uuid.New(), uuid.New()
	cat := &categoryStub{category: &model.Category{ID: categoryID, UserID: otherUser, Type: model.CategoryType("EXPENSE")}}
	repo := &transactionStub{}
	svc := service.NewTransactionService(repo, cat, nil, nil, nil)

	_, err := svc.CreateTransaction(context.Background(), userID, dto.CreateTransactionRequest{CategoryID: categoryID, Type: "EXPENSE", Amount: 1})
	require.ErrorIs(t, err, constant.ErrNotFound)
	require.Nil(t, repo.stored)

	repo.stored = &model.Transaction{ID: uuid.New(), UserID: userID, Type: model.TransactionTypeExpense, CategoryID: uuid.New()}
	before := repo.stored.CategoryID
	_, err = svc.UpdateTransaction(context.Background(), userID, repo.stored.ID, dto.UpdateTransactionRequest{CategoryID: &categoryID})
	require.ErrorIs(t, err, constant.ErrNotFound)
	require.Equal(t, before, repo.stored.CategoryID)
}
