package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

// ownedCategoryRepo emulates the user-scoped SQL (WHERE id = ? AND user_id = ?).
type ownedCategoryRepo struct {
	repository.CategoryRepo
	rows map[uuid.UUID]*model.Category
}

func (r *ownedCategoryRepo) find(userID, id uuid.UUID) (*model.Category, error) {
	c, ok := r.rows[id]
	if !ok || c.UserID != userID {
		return nil, constant.ErrNotFound
	}
	return c, nil
}
func (r *ownedCategoryRepo) GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Category, error) {
	return r.find(userID, id)
}
func (r *ownedCategoryRepo) UpdateFieldsForUser(ctx context.Context, userID, id uuid.UUID, updates map[string]interface{}) error {
	c, err := r.find(userID, id)
	if err != nil {
		return err
	}
	if v, ok := updates["name"].(string); ok {
		c.Name = v
	}
	return nil
}
func (r *ownedCategoryRepo) DeleteForUser(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := r.find(userID, id); err != nil {
		return err
	}
	delete(r.rows, id)
	return nil
}

func TestCategoryService_ScopedToOwner(t *testing.T) {
	owner, other := uuid.New(), uuid.New()
	cat := &model.Category{ID: uuid.New(), UserID: owner, Name: "Ăn uống"}
	repo := &ownedCategoryRepo{rows: map[uuid.UUID]*model.Category{cat.ID: cat}}
	svc := NewCategoryService(repo)
	ctx := context.Background()

	if _, err := svc.Get(ctx, other, cat.ID); !errors.Is(err, constant.ErrNotFound) {
		t.Fatalf("other user's Get: expected ErrNotFound, got %v", err)
	}
	if err := svc.UpdateFields(ctx, other, cat.ID, map[string]interface{}{"name": "hacked"}); !errors.Is(err, constant.ErrNotFound) {
		t.Fatalf("other user's Update: expected ErrNotFound, got %v", err)
	}
	if err := svc.Delete(ctx, other, cat.ID); !errors.Is(err, constant.ErrNotFound) {
		t.Fatalf("other user's Delete: expected ErrNotFound, got %v", err)
	}
	if cat.Name != "Ăn uống" {
		t.Fatal("category must not be modified by another user")
	}

	if got, err := svc.Get(ctx, owner, cat.ID); err != nil || got.ID != cat.ID {
		t.Fatalf("owner Get failed: %v", err)
	}
	if err := svc.Delete(ctx, owner, cat.ID); err != nil {
		t.Fatalf("owner Delete failed: %v", err)
	}
}

type ownedCostRepo struct {
	repository.CostRepo
	rows        map[uuid.UUID]*model.Cost
	lastUpdates map[string]interface{}
}

func (r *ownedCostRepo) GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Cost, error) {
	c, ok := r.rows[id]
	if !ok || c.UserID != userID {
		return nil, constant.ErrNotFound
	}
	return c, nil
}
func (r *ownedCostRepo) UpdateFieldsForUser(ctx context.Context, userID, id uuid.UUID, updates map[string]interface{}) error {
	if _, err := r.GetByIDForUser(ctx, userID, id); err != nil {
		return err
	}
	r.lastUpdates = updates
	return nil
}
func (r *ownedCostRepo) DeleteForUser(ctx context.Context, userID, id uuid.UUID) error {
	_, err := r.GetByIDForUser(ctx, userID, id)
	return err
}

func TestCostService_UpdateNeverChangesOwner(t *testing.T) {
	owner, other := uuid.New(), uuid.New()
	cost := &model.Cost{ID: uuid.New(), UserID: owner, Title: "Cafe", Amount: 35000}
	repo := &ownedCostRepo{rows: map[uuid.UUID]*model.Cost{cost.ID: cost}}
	svc := NewCostService(repo, &ownedCategoryRepo{rows: map[uuid.UUID]*model.Category{}})
	ctx := context.Background()

	title := "Cafe sữa"
	if _, err := svc.Update(ctx, other, cost.ID, dto.UpdateCostRequest{Title: &title}); !errors.Is(err, constant.ErrNotFound) {
		t.Fatalf("other user's update: expected ErrNotFound, got %v", err)
	}
	if _, err := svc.Update(ctx, owner, cost.ID, dto.UpdateCostRequest{Title: &title}); err != nil {
		t.Fatalf("owner update failed: %v", err)
	}
	if _, ok := repo.lastUpdates["user_id"]; ok {
		t.Fatal("update must never set user_id")
	}
	if repo.lastUpdates["title"] != title {
		t.Fatalf("expected title update, got %v", repo.lastUpdates)
	}
	if _, err := svc.Update(ctx, owner, cost.ID, dto.UpdateCostRequest{}); !errors.Is(err, constant.ErrInvalidInput) {
		t.Fatalf("empty update: expected ErrInvalidInput, got %v", err)
	}
	if _, err := svc.Get(ctx, other, cost.ID); !errors.Is(err, constant.ErrNotFound) {
		t.Fatalf("other user's Get: expected ErrNotFound, got %v", err)
	}
	if err := svc.Delete(ctx, other, cost.ID); !errors.Is(err, constant.ErrNotFound) {
		t.Fatalf("other user's Delete: expected ErrNotFound, got %v", err)
	}
}

func TestCostService_CategoryMustBelongToUser(t *testing.T) {
	owner, other := uuid.New(), uuid.New()
	mine := &model.Category{ID: uuid.New(), UserID: owner}
	theirs := &model.Category{ID: uuid.New(), UserID: other}
	cats := &ownedCategoryRepo{rows: map[uuid.UUID]*model.Category{mine.ID: mine, theirs.ID: theirs}}
	cost := &model.Cost{ID: uuid.New(), UserID: owner, CategoryID: mine.ID}
	repo := &ownedCostRepo{rows: map[uuid.UUID]*model.Cost{cost.ID: cost}}
	svc := NewCostService(repo, cats)
	ctx := context.Background()

	if _, err := svc.Create(ctx, &model.Cost{UserID: owner, CategoryID: theirs.ID}); !errors.Is(err, constant.ErrNotFound) {
		t.Fatalf("create with foreign category: expected ErrNotFound, got %v", err)
	}
	if _, err := svc.Update(ctx, owner, cost.ID, dto.UpdateCostRequest{CategoryID: &theirs.ID}); !errors.Is(err, constant.ErrNotFound) {
		t.Fatalf("update to foreign category: expected ErrNotFound, got %v", err)
	}
	if _, ok := repo.lastUpdates["category_id"]; ok {
		t.Fatal("foreign category must not be written")
	}
	if _, err := svc.Update(ctx, owner, cost.ID, dto.UpdateCostRequest{CategoryID: &mine.ID}); err != nil {
		t.Fatalf("own category update failed: %v", err)
	}
}
