package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

var errInternalRepo = errors.New("db down")

// recordingCategoryRepo records the arguments of list calls.
type recordingCategoryRepo struct {
	repository.CategoryRepo
	rows                  []model.Category
	created               []*model.Category
	createErr             error
	lastUserID            uuid.UUID
	lastType              string
	lastLimit, lastOffset int
}

func (r *recordingCategoryRepo) Create(_ context.Context, c *model.Category) error {
	if r.createErr != nil {
		return r.createErr
	}
	c.ID = uuid.New()
	r.created = append(r.created, c)
	r.rows = append(r.rows, *c)
	return nil
}

func (r *recordingCategoryRepo) filter(userID uuid.UUID, categoryType string) []model.Category {
	var out []model.Category
	for _, c := range r.rows {
		if c.UserID == userID && (categoryType == "" || string(c.Type) == categoryType) {
			out = append(out, c)
		}
	}
	return out
}

func (r *recordingCategoryRepo) List(_ context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, error) {
	r.lastUserID, r.lastType, r.lastLimit, r.lastOffset = userID, categoryType, limit, offset
	return r.filter(userID, categoryType), nil
}

func (r *recordingCategoryRepo) ListWithTotal(_ context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, int64, error) {
	r.lastUserID, r.lastType, r.lastLimit, r.lastOffset = userID, categoryType, limit, offset
	out := r.filter(userID, categoryType)
	return out, int64(len(out)), nil
}

func TestCategoryService_CreateAndList(t *testing.T) {
	owner, other := uuid.New(), uuid.New()
	repo := &recordingCategoryRepo{rows: []model.Category{
		{ID: uuid.New(), UserID: owner, Name: "Lương", Type: model.CategoryTypeIncome},
		{ID: uuid.New(), UserID: other, Name: "Theirs", Type: model.CategoryTypeExpense},
	}}
	svc := NewCategoryService(repo)
	ctx := context.Background()

	created, err := svc.Create(ctx, &model.Category{UserID: owner, Name: "Ăn uống", Type: model.CategoryTypeExpense})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, created.ID)

	repo.createErr = errInternalRepo
	_, err = svc.Create(ctx, &model.Category{UserID: owner, Name: "x"})
	require.ErrorIs(t, err, errInternalRepo)

	list, err := svc.List(ctx, owner, "EXPENSE", 20, 40)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "Ăn uống", list[0].Name)
	require.Equal(t, owner, repo.lastUserID)
	require.Equal(t, 20, repo.lastLimit)
	require.Equal(t, 40, repo.lastOffset)
}

func TestCategoryService_ListWithTotalOffset(t *testing.T) {
	owner := uuid.New()
	tests := []struct {
		page, limit, wantOffset int
	}{
		{page: 1, limit: 10, wantOffset: 0},
		{page: 3, limit: 10, wantOffset: 20},
		{page: 0, limit: 10, wantOffset: 0},
		{page: -2, limit: 10, wantOffset: 0},
		{page: 4, limit: 0, wantOffset: 0},
	}
	for _, tc := range tests {
		repo := &recordingCategoryRepo{rows: []model.Category{{ID: uuid.New(), UserID: owner, Type: model.CategoryTypeIncome}}}
		_, total, err := NewCategoryService(repo).ListWithTotal(context.Background(), owner, "INCOME", tc.page, tc.limit)
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Equal(t, tc.wantOffset, repo.lastOffset, "page=%d limit=%d", tc.page, tc.limit)
		require.Equal(t, tc.limit, repo.lastLimit)
		require.Equal(t, "INCOME", repo.lastType)
	}
}

// costRecorder adds Create/ListWithCategory to the user-scoped cost repo.
type costRecorder struct {
	*ownedCostRepo
	createErr   error
	listUserID  uuid.UUID
	listFilters map[string]interface{}
}

func (r *costRecorder) Create(_ context.Context, c *model.Cost) error {
	if r.createErr != nil {
		return r.createErr
	}
	c.ID = uuid.New()
	r.rows[c.ID] = c
	return nil
}

func (r *costRecorder) ListWithCategory(_ context.Context, userID uuid.UUID, _, _ int, filters map[string]interface{}) ([]model.Cost, error) {
	r.listUserID, r.listFilters = userID, filters
	var out []model.Cost
	for _, c := range r.rows {
		if c.UserID == userID {
			out = append(out, *c)
		}
	}
	return out, nil
}

func TestCostService_CreateAndListWithCategory(t *testing.T) {
	owner, other := uuid.New(), uuid.New()
	cat := &model.Category{ID: uuid.New(), UserID: owner}
	cats := &ownedCategoryRepo{rows: map[uuid.UUID]*model.Category{cat.ID: cat}}
	repo := &costRecorder{ownedCostRepo: &ownedCostRepo{rows: map[uuid.UUID]*model.Cost{}}}
	svc := NewCostService(repo, cats)
	ctx := context.Background()

	cost, err := svc.Create(ctx, &model.Cost{UserID: owner, CategoryID: cat.ID, Title: "Cafe", Amount: 35_000})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, cost.ID)

	repo.rows[uuid.New()] = &model.Cost{UserID: other, Title: "theirs"}
	filters := map[string]interface{}{"categoryId": cat.ID.String()}
	list, err := svc.ListWithCategory(ctx, owner, 10, 0, filters)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "Cafe", list[0].Title)
	require.Equal(t, owner, repo.listUserID)
	require.Equal(t, filters, repo.listFilters)

	repo.createErr = errInternalRepo
	_, err = svc.Create(ctx, &model.Cost{UserID: owner, CategoryID: cat.ID})
	require.ErrorIs(t, err, errInternalRepo)

	// A DB failure on the category lookup is not turned into a 404.
	failing := &failingCategoryRepo{err: errInternalRepo}
	_, err = NewCostService(repo, failing).Create(ctx, &model.Cost{UserID: owner, CategoryID: cat.ID})
	require.ErrorIs(t, err, errInternalRepo)
	require.NotErrorIs(t, err, constant.ErrNotFound)
}

type failingCategoryRepo struct {
	repository.CategoryRepo
	err error
}

func (r *failingCategoryRepo) GetByIDForUser(context.Context, uuid.UUID, uuid.UUID) (*model.Category, error) {
	return nil, r.err
}

func TestCostService_UpdateAppliesAllFields(t *testing.T) {
	owner := uuid.New()
	cat := &model.Category{ID: uuid.New(), UserID: owner}
	cost := &model.Cost{ID: uuid.New(), UserID: owner, Title: "Cafe"}
	repo := &ownedCostRepo{rows: map[uuid.UUID]*model.Cost{cost.ID: cost}}
	svc := NewCostService(repo, &ownedCategoryRepo{rows: map[uuid.UUID]*model.Category{cat.ID: cat}})

	title, amount, currency := "Trà sữa", 45_000.0, "VND"
	at := dto.CustomTime{Time: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)}
	_, err := svc.Update(context.Background(), owner, cost.ID, dto.UpdateCostRequest{Title: &title, Amount: &amount, Currency: &currency, IncurredAt: &at, CategoryID: &cat.ID})
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{
		"title": title, "amount": amount, "currency": currency, "incurred_at": at.Time, "category_id": cat.ID,
	}, repo.lastUpdates)
}

func TestUserService_GetListDelete(t *testing.T) {
	repo := newMockUserRepo()
	a := &model.User{ID: uuid.New(), Username: "alice", Email: "a@example.com"}
	b := &model.User{ID: uuid.New(), Username: "bob", Email: "b@example.com"}
	_ = repo.Create(context.Background(), a)
	_ = repo.Create(context.Background(), b)
	svc := NewUserService(repo)
	ctx := context.Background()

	got, err := svc.Get(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, "alice", got.Username)
	_, err = svc.Get(ctx, uuid.New())
	require.ErrorIs(t, err, constant.ErrNotFound)

	list, err := svc.List(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 2)

	require.ErrorIs(t, svc.Delete(ctx, uuid.New()), constant.ErrNotFound)
	require.NoError(t, svc.Delete(ctx, a.ID))
	_, err = svc.Get(ctx, a.ID)
	require.ErrorIs(t, err, constant.ErrNotFound)
}

func TestUserService_CreateRejectsDuplicatesAndTrims(t *testing.T) {
	repo := newMockUserRepo()
	_ = repo.Create(context.Background(), &model.User{ID: uuid.New(), Username: "alice", Email: "a@example.com"})
	svc := NewUserService(repo)

	_, err := svc.Create(context.Background(), dto.CreateUserRequest{Username: "new", Email: "  a@example.com ", Password: "password123"})
	require.ErrorIs(t, err, constant.ErrEmailAlreadyExists)
	_, err = svc.Create(context.Background(), dto.CreateUserRequest{Username: " alice ", Email: "n@example.com", Password: "password123"})
	require.ErrorIs(t, err, constant.ErrUsernameTaken)

	u, err := svc.Create(context.Background(), dto.CreateUserRequest{Username: "  carol ", Email: " c@example.com  ", Password: "password123"})
	require.NoError(t, err)
	require.Equal(t, "carol", u.Username)
	require.Equal(t, "c@example.com", u.Email)
}

func TestParseDateString(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "2026-08-20", want: "2026-08-20"},
		{in: "2026-02-28T23:30:00+07:00", want: "2026-02-28"},
		{in: "2026-12-31T00:00:00Z", want: "2026-12-31"},
		{in: "", wantErr: true},
		{in: "20/08/2026", wantErr: true},
		{in: "2026-02-30", wantErr: true},
		{in: "2026-8-1", wantErr: true},
	}
	for _, tc := range tests {
		got, err := parseDateString(tc.in)
		if tc.wantErr {
			require.ErrorIs(t, err, ErrInvalidDate, tc.in)
			require.True(t, got.IsZero())
			continue
		}
		require.NoError(t, err, tc.in)
		require.Equal(t, tc.want, got.Format("2006-01-02"), tc.in)
	}
}
