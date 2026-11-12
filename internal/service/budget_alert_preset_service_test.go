package service_test

import (
	"context"
	"errors"
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

// ---------------------------------------------------------------------------
// In-memory repositories. Like the real repos, GetByIDForUser and Delete are
// user-scoped: another user's row is reported as not found. GetByIDForUser
// returns a copy so a service mutating it does not change "the DB".
// ---------------------------------------------------------------------------

var errRepoDown = errors.New("db down")

type memAlertRepo struct {
	repository.AlertRepository
	rows       map[uuid.UUID]model.Alert
	order      []uuid.UUID
	deleted    []uuid.UUID
	lastLimit  int
	lastOffset int
	listErr    error
}

func newMemAlertRepo(alerts ...model.Alert) *memAlertRepo {
	r := &memAlertRepo{rows: map[uuid.UUID]model.Alert{}}
	for _, a := range alerts {
		r.rows[a.ID] = a
		r.order = append(r.order, a.ID)
	}
	return r
}

func (r *memAlertRepo) ListByUserID(_ context.Context, userID uuid.UUID, limit, offset int) ([]model.Alert, int64, error) {
	r.lastLimit, r.lastOffset = limit, offset
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	var mine []model.Alert
	for _, id := range r.order {
		if a := r.rows[id]; a.UserID == userID {
			mine = append(mine, a)
		}
	}
	total := int64(len(mine))
	if offset >= len(mine) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(mine) {
		end = len(mine)
	}
	return mine[offset:end], total, nil
}

func (r *memAlertRepo) Delete(_ context.Context, userID, id uuid.UUID) error {
	if row, ok := r.rows[id]; !ok || row.UserID != userID {
		return fmt.Errorf("alert %w", constant.ErrNotFound)
	}
	delete(r.rows, id)
	r.deleted = append(r.deleted, id)
	return nil
}

func TestAlertService_ListAlertsPaginatesOwnAlertsOnly(t *testing.T) {
	owner, other := uuid.New(), uuid.New()
	var alerts []model.Alert
	for i := 0; i < 5; i++ {
		alerts = append(alerts, model.Alert{ID: uuid.New(), UserID: owner, AlertType: "over_limit", Message: "m", TriggeredAt: time.Now()})
	}
	alerts = append(alerts, model.Alert{ID: uuid.New(), UserID: other, AlertType: "over_limit"})
	repo := newMemAlertRepo(alerts...)
	svc := service.NewAlertService(repo)

	res, total, err := svc.ListAlerts(context.Background(), owner, 2, 2)
	require.NoError(t, err)
	require.Equal(t, 2, repo.lastLimit)
	require.Equal(t, 2, repo.lastOffset, "page 2 with limit 2 starts at offset 2")
	require.Equal(t, int64(5), total, "other user's alert is not counted")
	require.Len(t, res, 2)
	require.Equal(t, alerts[2].ID, res[0].ID)
	for _, a := range res {
		require.Equal(t, owner, a.UserID)
	}

	res, total, err = svc.ListAlerts(context.Background(), uuid.New(), 1, 10)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, res)

	repo.listErr = errRepoDown
	_, _, err = svc.ListAlerts(context.Background(), owner, 1, 10)
	require.ErrorIs(t, err, errRepoDown)
}

func TestAlertService_DeleteAlertOwnership(t *testing.T) {
	owner, other := uuid.New(), uuid.New()
	alert := model.Alert{ID: uuid.New(), UserID: owner}
	repo := newMemAlertRepo(alert)
	svc := service.NewAlertService(repo)
	ctx := context.Background()

	err := svc.DeleteAlert(ctx, other, alert.ID)
	require.ErrorIs(t, err, constant.ErrNotFound)
	require.Empty(t, repo.deleted, "another user's alert must never be deleted")

	require.ErrorIs(t, svc.DeleteAlert(ctx, owner, uuid.New()), constant.ErrNotFound)

	require.NoError(t, svc.DeleteAlert(ctx, owner, alert.ID))
	require.Equal(t, []uuid.UUID{alert.ID}, repo.deleted)
}

type memBudgetRepo struct {
	repository.BudgetRepository
	rows       map[uuid.UUID]model.Budget
	updated    []model.Budget
	deleted    []uuid.UUID
	lastLimit  int
	lastOffset int
	updateErr  error
}

func newMemBudgetRepo(budgets ...model.Budget) *memBudgetRepo {
	r := &memBudgetRepo{rows: map[uuid.UUID]model.Budget{}}
	for _, b := range budgets {
		r.rows[b.ID] = b
	}
	return r
}

func (r *memBudgetRepo) GetByIDForUser(_ context.Context, userID, id uuid.UUID) (*model.Budget, error) {
	b, ok := r.rows[id]
	if !ok || b.UserID != userID {
		return nil, fmt.Errorf("budget %w", constant.ErrNotFound)
	}
	return &b, nil
}

func (r *memBudgetRepo) ListByUserID(_ context.Context, userID uuid.UUID, limit, offset int) ([]model.Budget, int64, error) {
	r.lastLimit, r.lastOffset = limit, offset
	var out []model.Budget
	for _, b := range r.rows {
		if b.UserID == userID {
			out = append(out, b)
		}
	}
	return out, int64(len(out)), nil
}

func (r *memBudgetRepo) Update(_ context.Context, b *model.Budget) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.rows[b.ID] = *b
	r.updated = append(r.updated, *b)
	return nil
}

func (r *memBudgetRepo) Delete(_ context.Context, userID, id uuid.UUID) error {
	if row, ok := r.rows[id]; !ok || row.UserID != userID {
		return fmt.Errorf("budget %w", constant.ErrNotFound)
	}
	delete(r.rows, id)
	r.deleted = append(r.deleted, id)
	return nil
}

func TestBudgetService_GetListUpdateDeleteScopedToOwner(t *testing.T) {
	owner, other := uuid.New(), uuid.New()
	mine := model.Budget{ID: uuid.New(), UserID: owner, CategoryID: uuid.New(), Amount: 3_000_000, PeriodType: "monthly",
		PeriodStart: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Category: model.Category{Name: "Ăn uống"}}
	theirs := model.Budget{ID: uuid.New(), UserID: other, Amount: 1, PeriodType: "monthly"}
	repo := newMemBudgetRepo(mine, theirs)
	svc := service.NewBudgetService(repo, nil)
	ctx := context.Background()

	// Get
	got, err := svc.GetBudget(ctx, owner, mine.ID)
	require.NoError(t, err)
	require.Equal(t, "Ăn uống", got.CategoryName)
	require.Equal(t, "2026-11-01", got.PeriodStart)
	_, err = svc.GetBudget(ctx, owner, theirs.ID)
	require.ErrorIs(t, err, constant.ErrNotFound)
	_, err = svc.GetBudget(ctx, owner, uuid.New())
	require.ErrorIs(t, err, constant.ErrNotFound)

	// List
	list, total, err := svc.ListBudgets(ctx, owner, 3, 20)
	require.NoError(t, err)
	require.Equal(t, 40, repo.lastOffset)
	require.Equal(t, 20, repo.lastLimit)
	require.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	require.Equal(t, mine.ID, list[0].ID)

	// Update: another user's budget is untouched.
	amount := 9.0
	_, err = svc.UpdateBudget(ctx, owner, theirs.ID, dto.UpdateBudgetRequest{Amount: &amount})
	require.ErrorIs(t, err, constant.ErrNotFound)
	require.Empty(t, repo.updated)
	require.Equal(t, 1.0, repo.rows[theirs.ID].Amount)
	_, err = svc.UpdateBudget(ctx, owner, uuid.New(), dto.UpdateBudgetRequest{Amount: &amount})
	require.ErrorIs(t, err, constant.ErrNotFound)

	// Partial update keeps the fields that were not sent.
	amount = 4_500_000
	updated, err := svc.UpdateBudget(ctx, owner, mine.ID, dto.UpdateBudgetRequest{Amount: &amount})
	require.NoError(t, err)
	require.Equal(t, 4_500_000.0, updated.Amount)
	require.Equal(t, "monthly", updated.PeriodType)
	require.Equal(t, owner, repo.rows[mine.ID].UserID)

	yearly := "yearly"
	updated, err = svc.UpdateBudget(ctx, owner, mine.ID, dto.UpdateBudgetRequest{PeriodType: &yearly})
	require.NoError(t, err)
	require.Equal(t, "yearly", updated.PeriodType)
	require.Equal(t, 4_500_000.0, updated.Amount)

	repo.updateErr = errRepoDown
	_, err = svc.UpdateBudget(ctx, owner, mine.ID, dto.UpdateBudgetRequest{Amount: &amount})
	require.ErrorIs(t, err, errRepoDown)

	// Delete
	require.ErrorIs(t, svc.DeleteBudget(ctx, owner, theirs.ID), constant.ErrNotFound)
	require.ErrorIs(t, svc.DeleteBudget(ctx, owner, uuid.New()), constant.ErrNotFound)
	require.Empty(t, repo.deleted)
	require.NoError(t, svc.DeleteBudget(ctx, owner, mine.ID))
	require.Equal(t, []uuid.UUID{mine.ID}, repo.deleted)
}

func TestBudgetService_CreateBudgetCopiesCategoryAndPropagatesErrors(t *testing.T) {
	userID, categoryID := uuid.New(), uuid.New()
	cat := &categoryStub{category: &model.Category{ID: categoryID, UserID: userID, Name: "Cà phê"}}
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	budgets := &budgetCreateStub{}
	res, err := service.NewBudgetService(budgets, cat).CreateBudget(context.Background(), userID,
		dto.CreateBudgetRequest{CategoryID: categoryID, Amount: 500_000, PeriodType: "monthly", PeriodStart: start})
	require.NoError(t, err)
	require.Equal(t, "Cà phê", res.CategoryName)
	require.Equal(t, userID, res.UserID, "owner comes from the authenticated user")
	require.Equal(t, 500_000.0, res.Amount)
	require.Equal(t, "2026-11-01", res.PeriodStart)

	_, err = service.NewBudgetService(budgets, cat).CreateBudget(context.Background(), userID,
		dto.CreateBudgetRequest{CategoryID: uuid.New(), Amount: 1, PeriodType: "monthly"})
	require.ErrorIs(t, err, constant.ErrNotFound, "unknown category is a 404")
}

type memPresetRepo struct {
	repository.PresetRepository
	rows      map[uuid.UUID]model.Preset
	created   []model.Preset
	updated   []model.Preset
	deleted   []uuid.UUID
	createErr error
	listErr   error
}

func newMemPresetRepo(presets ...model.Preset) *memPresetRepo {
	r := &memPresetRepo{rows: map[uuid.UUID]model.Preset{}}
	for _, p := range presets {
		r.rows[p.ID] = p
	}
	return r
}

func (r *memPresetRepo) Create(_ context.Context, p *model.Preset) error {
	if r.createErr != nil {
		return r.createErr
	}
	p.ID = uuid.New()
	r.rows[p.ID] = *p
	r.created = append(r.created, *p)
	return nil
}

func (r *memPresetRepo) GetByIDForUser(_ context.Context, userID, id uuid.UUID) (*model.Preset, error) {
	p, ok := r.rows[id]
	if !ok || p.UserID != userID {
		return nil, fmt.Errorf("preset %w", constant.ErrNotFound)
	}
	return &p, nil
}

func (r *memPresetRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]model.Preset, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	var out []model.Preset
	for _, p := range r.rows {
		if p.UserID == userID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *memPresetRepo) Update(_ context.Context, p *model.Preset) error {
	r.rows[p.ID] = *p
	r.updated = append(r.updated, *p)
	return nil
}

func (r *memPresetRepo) Delete(_ context.Context, userID, id uuid.UUID) error {
	if row, ok := r.rows[id]; !ok || row.UserID != userID {
		return fmt.Errorf("preset %w", constant.ErrNotFound)
	}
	delete(r.rows, id)
	r.deleted = append(r.deleted, id)
	return nil
}

func TestPresetService_CreatePreset(t *testing.T) {
	userID, categoryID := uuid.New(), uuid.New()
	cat := &categoryStub{category: &model.Category{ID: categoryID, UserID: userID, Name: "Cà phê"}}
	repo := newMemPresetRepo()
	svc := service.NewPresetService(repo, cat)

	res, err := svc.CreatePreset(context.Background(), userID, dto.CreatePresetRequest{
		CategoryID: categoryID, Name: "Cafe sáng", Amount: 35_000, Type: "EXPENSE", Description: "Highlands", Icon: "☕", SortOrder: 2,
	})
	require.NoError(t, err)
	require.Equal(t, "Cà phê", res.CategoryName)
	require.Equal(t, userID, res.UserID)
	require.Equal(t, "EXPENSE", res.Type)
	require.Equal(t, 35_000.0, res.Amount)
	require.Equal(t, 2, res.SortOrder)
	require.Len(t, repo.created, 1)
	require.Equal(t, userID, repo.created[0].UserID)

	repo.createErr = errRepoDown
	_, err = svc.CreatePreset(context.Background(), userID, dto.CreatePresetRequest{CategoryID: categoryID, Name: "x", Amount: 1, Type: "EXPENSE"})
	require.ErrorIs(t, err, errRepoDown)
}

func TestPresetService_GetListUpdateDeleteScopedToOwner(t *testing.T) {
	owner, other := uuid.New(), uuid.New()
	ownCat := &model.Category{ID: uuid.New(), UserID: owner, Name: "Ăn sáng"}
	mine := model.Preset{ID: uuid.New(), UserID: owner, CategoryID: uuid.New(), Name: "Phở", Amount: 50_000, Type: model.TransactionTypeExpense, Icon: "🍜"}
	theirs := model.Preset{ID: uuid.New(), UserID: other, Name: "Theirs", Amount: 1}
	repo := newMemPresetRepo(mine, theirs)
	svc := service.NewPresetService(repo, &categoryStub{category: ownCat})
	ctx := context.Background()

	got, err := svc.GetPreset(ctx, owner, mine.ID)
	require.NoError(t, err)
	require.Equal(t, "Phở", got.Name)
	_, err = svc.GetPreset(ctx, owner, theirs.ID)
	require.ErrorIs(t, err, constant.ErrNotFound)
	_, err = svc.GetPreset(ctx, owner, uuid.New())
	require.ErrorIs(t, err, constant.ErrNotFound)

	list, err := svc.ListPresets(ctx, owner)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, mine.ID, list[0].ID)

	empty, err := svc.ListPresets(ctx, uuid.New())
	require.NoError(t, err)
	require.NotNil(t, empty, "an empty list serialises as [] not null")
	require.Empty(t, empty)

	repo.listErr = errRepoDown
	_, err = svc.ListPresets(ctx, owner)
	require.ErrorIs(t, err, errRepoDown)
	repo.listErr = nil

	// Update another user's preset: rejected, nothing written.
	name := "hacked"
	_, err = svc.UpdatePreset(ctx, owner, theirs.ID, dto.UpdatePresetRequest{Name: &name})
	require.ErrorIs(t, err, constant.ErrNotFound)
	_, err = svc.UpdatePreset(ctx, owner, uuid.New(), dto.UpdatePresetRequest{Name: &name})
	require.ErrorIs(t, err, constant.ErrNotFound)
	require.Empty(t, repo.updated)
	require.Equal(t, "Theirs", repo.rows[theirs.ID].Name)

	// Moving the preset to someone else's category is a 404 and nothing is written.
	foreignCat := uuid.New()
	_, err = svc.UpdatePreset(ctx, owner, mine.ID, dto.UpdatePresetRequest{CategoryID: &foreignCat, Name: &name})
	require.ErrorIs(t, err, constant.ErrNotFound)
	require.Empty(t, repo.updated)
	require.Equal(t, "Phở", repo.rows[mine.ID].Name)

	// Full update with an owned category.
	newName, amount, typ, desc, icon, sort := "Bún chả", 60_000.0, "EXPENSE", "trưa", "🥢", 5
	updated, err := svc.UpdatePreset(ctx, owner, mine.ID, dto.UpdatePresetRequest{
		CategoryID: &ownCat.ID, Name: &newName, Amount: &amount, Type: &typ, Description: &desc, Icon: &icon, SortOrder: &sort,
	})
	require.NoError(t, err)
	require.Equal(t, ownCat.ID, updated.CategoryID)
	require.Equal(t, "Ăn sáng", updated.CategoryName)
	require.Equal(t, "Bún chả", updated.Name)
	require.Equal(t, 60_000.0, updated.Amount)
	require.Equal(t, "trưa", updated.Description)
	require.Equal(t, "🥢", updated.Icon)
	require.Equal(t, 5, updated.SortOrder)
	require.Equal(t, owner, repo.rows[mine.ID].UserID)

	// Partial update keeps everything else.
	amount = 70_000
	updated, err = svc.UpdatePreset(ctx, owner, mine.ID, dto.UpdatePresetRequest{Amount: &amount})
	require.NoError(t, err)
	require.Equal(t, "Bún chả", updated.Name)
	require.Equal(t, 70_000.0, updated.Amount)

	require.ErrorIs(t, svc.DeletePreset(ctx, owner, theirs.ID), constant.ErrNotFound)
	require.ErrorIs(t, svc.DeletePreset(ctx, owner, uuid.New()), constant.ErrNotFound)
	require.Empty(t, repo.deleted)
	require.NoError(t, svc.DeletePreset(ctx, owner, mine.ID))
	require.Equal(t, []uuid.UUID{mine.ID}, repo.deleted)
}

// AI tools call the service directly, without DTO validation: the service
// itself must only store the period types every budget query matches.
func TestBudgetService_RejectsUnknownPeriodType(t *testing.T) {
	userID, categoryID := uuid.New(), uuid.New()
	cat := &categoryStub{category: &model.Category{ID: categoryID, UserID: userID, Name: "Cà phê"}}
	ctx := context.Background()

	for _, period := range []string{"MONTHLY", "weekly", ""} {
		budgets := &budgetCreateStub{}
		_, err := service.NewBudgetService(budgets, cat).CreateBudget(ctx, userID,
			dto.CreateBudgetRequest{CategoryID: categoryID, Amount: 1, PeriodType: period})
		require.ErrorIs(t, err, constant.ErrInvalidInput, "period %q", period)
	}

	mine := model.Budget{ID: uuid.New(), UserID: userID, Amount: 1, PeriodType: model.BudgetPeriodMonthly}
	repo := newMemBudgetRepo(mine)
	bad := "Yearly"
	_, err := service.NewBudgetService(repo, nil).UpdateBudget(ctx, userID, mine.ID, dto.UpdateBudgetRequest{PeriodType: &bad})
	require.ErrorIs(t, err, constant.ErrInvalidInput)
	yearly := model.BudgetPeriodYearly
	res, err := service.NewBudgetService(repo, nil).UpdateBudget(ctx, userID, mine.ID, dto.UpdateBudgetRequest{PeriodType: &yearly})
	require.NoError(t, err)
	require.Equal(t, "yearly", res.PeriodType)
}
