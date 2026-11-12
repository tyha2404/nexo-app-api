package service_test

import (
	"context"
	"fmt"
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

// memTxnRepo is an in-memory TransactionRepository. GetByID is not user
// scoped (like the real one), so the service must check the owner.
type memTxnRepo struct {
	repository.TransactionRepository
	rows        map[uuid.UUID]*model.Transaction
	deleted     []uuid.UUID
	updates     int
	lastLimit   int
	lastOffset  int
	lastFilters map[string]interface{}
	summary     dto.TransactionSummaryDTO
	tags        []repository.TagUsage
	listErr     error
	summaryErr  error
	tagsErr     error
	updateErr   error
}

func newMemTxnRepo(txns ...*model.Transaction) *memTxnRepo {
	r := &memTxnRepo{rows: map[uuid.UUID]*model.Transaction{}}
	for _, tx := range txns {
		r.rows[tx.ID] = tx
	}
	return r
}

func (r *memTxnRepo) GetByIDForUser(_ context.Context, userID, id uuid.UUID) (*model.Transaction, error) {
	tx, ok := r.rows[id]
	if !ok || tx.UserID != userID {
		return nil, fmt.Errorf("transaction %w", constant.ErrNotFound)
	}
	cp := *tx
	return &cp, nil
}

func (r *memTxnRepo) ListByUserID(_ context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Transaction, int64, error) {
	r.lastLimit, r.lastOffset, r.lastFilters = limit, offset, filters
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	var out []model.Transaction
	for _, tx := range r.rows {
		if tx.UserID == userID {
			out = append(out, *tx)
		}
	}
	return out, int64(len(out)), nil
}

func (r *memTxnRepo) GetSummaryByUserID(_ context.Context, _ uuid.UUID, _ map[string]interface{}) (repository.TransactionSummary, error) {
	if r.summaryErr != nil {
		return repository.TransactionSummary{}, r.summaryErr
	}
	s := r.summary
	return repository.TransactionSummary{SumAmount: s.SumAmount, SumAmountForAverage: s.SumAmountForAverage, Total: s.Total,
		HoldingAmount: s.HoldingAmount, HoldingCount: s.HoldingCount, RealizedPnL: s.RealizedPnL}, nil
}

func (r *memTxnRepo) Update(_ context.Context, tx *model.Transaction) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	cp := *tx
	r.rows[tx.ID] = &cp
	r.updates++
	return nil
}

func (r *memTxnRepo) Delete(_ context.Context, userID, id uuid.UUID) error {
	if tx, ok := r.rows[id]; !ok || tx.UserID != userID {
		return fmt.Errorf("transaction %w", constant.ErrNotFound)
	}
	delete(r.rows, id)
	r.deleted = append(r.deleted, id)
	return nil
}

func (r *memTxnRepo) ListTags(_ context.Context, _ uuid.UUID) ([]repository.TagUsage, error) {
	return r.tags, r.tagsErr
}

func TestListTransactions_PaginationSummaryAndOwnership(t *testing.T) {
	owner, other := uuid.New(), uuid.New()
	desc := "Phở"
	mine := &model.Transaction{ID: uuid.New(), UserID: owner, Amount: 50_000, Type: model.TransactionTypeExpense, Description: &desc,
		TransactionDate: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC), Category: &model.Category{Name: "Ăn uống"}}
	theirs := &model.Transaction{ID: uuid.New(), UserID: other, Amount: 1, Type: model.TransactionTypeExpense}
	repo := newMemTxnRepo(mine, theirs)
	repo.summary = dto.TransactionSummaryDTO{SumAmount: 50_000, SumAmountForAverage: 40_000, Total: 1, HoldingAmount: 7, HoldingCount: 2, RealizedPnL: -3}
	svc := service.NewTransactionService(repo, nil, nil, nil, nil)

	filters := map[string]interface{}{"type": "EXPENSE"}
	items, total, summary, err := svc.ListTransactions(context.Background(), owner, 3, 25, filters)
	require.NoError(t, err)
	require.Equal(t, 25, repo.lastLimit)
	require.Equal(t, 50, repo.lastOffset)
	require.Equal(t, filters, repo.lastFilters)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, mine.ID, items[0].ID)
	require.Equal(t, "Ăn uống", items[0].CategoryName)
	require.Equal(t, "2026-11-03", items[0].TransactionDate)
	require.Equal(t, &repo.summary, summary)

	repo.summaryErr = errRepoDown
	_, _, _, err = svc.ListTransactions(context.Background(), owner, 1, 10, nil)
	require.ErrorIs(t, err, errRepoDown)

	repo.listErr = errRepoDown
	_, _, _, err = svc.ListTransactions(context.Background(), owner, 1, 10, nil)
	require.ErrorIs(t, err, errRepoDown)
}

func TestDeleteTransaction_OnlyOwnerCanDelete(t *testing.T) {
	owner, other := uuid.New(), uuid.New()
	tx := &model.Transaction{ID: uuid.New(), UserID: owner, Amount: 10, Type: model.TransactionTypeExpense}
	repo := newMemTxnRepo(tx)
	svc := service.NewTransactionService(repo, nil, nil, nil, nil)
	ctx := context.Background()

	require.ErrorIs(t, svc.DeleteTransaction(ctx, other, tx.ID), constant.ErrNotFound)
	require.Empty(t, repo.deleted, "another user's transaction must not be deleted")
	require.ErrorIs(t, svc.DeleteTransaction(ctx, owner, uuid.New()), constant.ErrNotFound)

	require.NoError(t, svc.DeleteTransaction(ctx, owner, tx.ID))
	require.Equal(t, []uuid.UUID{tx.ID}, repo.deleted)
}

func TestGetTransaction_OtherUserIsNotFound(t *testing.T) {
	owner := uuid.New()
	tx := &model.Transaction{ID: uuid.New(), UserID: owner, Amount: 10, Type: model.TransactionTypeIncome}
	svc := service.NewTransactionService(newMemTxnRepo(tx), nil, nil, nil, nil)

	_, err := svc.GetTransaction(context.Background(), uuid.New(), tx.ID)
	require.ErrorIs(t, err, constant.ErrNotFound)
	_, err = svc.GetTransaction(context.Background(), owner, uuid.New())
	require.ErrorIs(t, err, constant.ErrNotFound)
	res, err := svc.GetTransaction(context.Background(), owner, tx.ID)
	require.NoError(t, err)
	require.Equal(t, tx.ID, res.ID)
}

func TestListTags_MapsUsageAndFormatsDate(t *testing.T) {
	repo := newMemTxnRepo()
	repo.tags = []repository.TagUsage{
		{Name: "Du lịch Đà Lạt", Count: 12, TotalExpense: 8_500_000, TotalIncome: 0, LastUsed: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)},
		{Name: "công tác", Count: 1, TotalIncome: 2_000_000, LastUsed: time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)},
	}
	svc := service.NewTransactionService(repo, nil, nil, nil, nil)

	tags, err := svc.ListTags(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Equal(t, []dto.TagSummary{
		{Name: "Du lịch Đà Lạt", Count: 12, TotalExpense: 8_500_000, LastUsed: "2026-11-04"},
		{Name: "công tác", Count: 1, TotalIncome: 2_000_000, LastUsed: "2026-01-09"},
	}, tags)

	repo.tags = nil
	tags, err = svc.ListTags(context.Background(), uuid.New())
	require.NoError(t, err)
	require.NotNil(t, tags, "no tags serialises as [] not null")
	require.Empty(t, tags)

	repo.tagsErr = errRepoDown
	_, err = svc.ListTags(context.Background(), uuid.New())
	require.ErrorIs(t, err, errRepoDown)
}

func TestUpdateTransaction_TypeMustMatchCategory(t *testing.T) {
	userID := uuid.New()
	expenseCat := &model.Category{ID: uuid.New(), UserID: userID, Type: model.CategoryTypeExpense, Name: "Ăn uống"}
	incomeCat := &model.Category{ID: uuid.New(), UserID: userID, Type: model.CategoryTypeIncome, Name: "Lương"}

	tests := []struct {
		name    string
		catRepo *model.Category
		req     func() dto.UpdateTransactionRequest
		wantErr error
	}{
		{
			name:    "type change without category must match current category",
			catRepo: expenseCat,
			req: func() dto.UpdateTransactionRequest {
				typ := "INCOME"
				return dto.UpdateTransactionRequest{Type: &typ}
			},
			wantErr: constant.ErrInvalidInput,
		},
		{
			name:    "new category must match the unchanged type",
			catRepo: incomeCat,
			req: func() dto.UpdateTransactionRequest {
				return dto.UpdateTransactionRequest{CategoryID: &incomeCat.ID}
			},
			wantErr: constant.ErrInvalidInput,
		},
		{
			name:    "new category and new type together",
			catRepo: incomeCat,
			req: func() dto.UpdateTransactionRequest {
				typ := "INCOME"
				return dto.UpdateTransactionRequest{CategoryID: &incomeCat.ID, Type: &typ}
			},
		},
		{
			name:    "same type as the current category",
			catRepo: expenseCat,
			req: func() dto.UpdateTransactionRequest {
				typ := "EXPENSE"
				return dto.UpdateTransactionRequest{Type: &typ}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tx := &model.Transaction{ID: uuid.New(), UserID: userID, CategoryID: expenseCat.ID, Amount: 10, Type: model.TransactionTypeExpense}
			repo := newMemTxnRepo(tx)
			svc := service.NewTransactionService(repo, &categoryStub{category: tc.catRepo}, &budgetStub{}, &alertStub{}, nil)

			res, err := svc.UpdateTransaction(context.Background(), userID, tx.ID, tc.req())
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Zero(t, repo.updates, "a rejected update must not be written")
				require.Equal(t, model.TransactionTypeExpense, repo.rows[tx.ID].Type)
				return
			}
			require.NoError(t, err)
			require.Equal(t, string(tc.catRepo.Type), res.Type)
			require.Equal(t, 1, repo.updates)
		})
	}
}

func TestUpdateTransaction_AppliesFieldsAndChecksBudget(t *testing.T) {
	userID, categoryID := uuid.New(), uuid.New()
	tx := &model.Transaction{ID: uuid.New(), UserID: userID, CategoryID: categoryID, Amount: 10, Type: model.TransactionTypeExpense,
		TransactionDate: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	repo := newMemTxnRepo(tx)
	repo.summary = dto.TransactionSummaryDTO{SumAmount: 900_000}
	alerts := &alertStub{}
	budgets := &budgetStub{budget: &model.Budget{ID: uuid.New(), Amount: 1_000_000, PeriodType: "monthly"}}
	svc := service.NewTransactionService(repo, nil, budgets, alerts, nil)

	amount, desc, status, pnl := 900_000.0, "Siêu thị", "SOLD", 5.0
	date := dto.CustomTime{Time: time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)}
	res, err := svc.UpdateTransaction(context.Background(), userID, tx.ID, dto.UpdateTransactionRequest{
		Amount: &amount, Description: &desc, Status: &status, RealizedPnL: &pnl, TransactionDate: &date,
	})
	require.NoError(t, err)
	require.Equal(t, 900_000.0, res.Amount)
	require.Equal(t, "Siêu thị", *res.Description)
	require.Equal(t, "SOLD", *res.Status)
	require.Equal(t, 5.0, *res.RealizedPnL)
	require.Equal(t, "2026-11-07", res.TransactionDate)
	require.Equal(t, userID, repo.rows[tx.ID].UserID, "owner never changes")

	// 900k of a 1M budget = 90% -> approaching_limit alert for the expense.
	require.Len(t, alerts.alerts, 1)
	require.Equal(t, "approaching_limit", alerts.alerts[0].AlertType)
	require.Equal(t, userID, alerts.alerts[0].UserID)
}

func TestUpdateTransaction_RejectsForeignAndPropagatesErrors(t *testing.T) {
	owner := uuid.New()
	tx := &model.Transaction{ID: uuid.New(), UserID: owner, Amount: 10, Type: model.TransactionTypeIncome}
	repo := newMemTxnRepo(tx)
	svc := service.NewTransactionService(repo, nil, nil, nil, nil)
	amount := 99.0

	_, err := svc.UpdateTransaction(context.Background(), uuid.New(), tx.ID, dto.UpdateTransactionRequest{Amount: &amount})
	require.ErrorIs(t, err, constant.ErrNotFound)
	require.Equal(t, 10.0, repo.rows[tx.ID].Amount)

	_, err = svc.UpdateTransaction(context.Background(), owner, uuid.New(), dto.UpdateTransactionRequest{Amount: &amount})
	require.ErrorIs(t, err, constant.ErrNotFound)

	repo.updateErr = errRepoDown
	_, err = svc.UpdateTransaction(context.Background(), owner, tx.ID, dto.UpdateTransactionRequest{Amount: &amount})
	require.ErrorIs(t, err, errRepoDown)
}
