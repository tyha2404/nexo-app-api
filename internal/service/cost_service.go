package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

// CostService backs the legacy /costs endpoints. Clients should use
// /transactions; every method here is scoped to the owning user.
type CostService interface {
	Create(ctx context.Context, cost *model.Cost) (*model.Cost, error)
	Get(ctx context.Context, userID, id uuid.UUID) (*model.Cost, error)
	Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateCostRequest) (*model.Cost, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
	ListWithCategory(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Cost, error)
}

type costService struct {
	repo         repository.CostRepo
	categoryRepo repository.CategoryRepo
}

func NewCostService(repo repository.CostRepo, categoryRepo repository.CategoryRepo) CostService {
	return &costService{
		repo:         repo,
		categoryRepo: categoryRepo,
	}
}

func (s *costService) Create(ctx context.Context, cost *model.Cost) (*model.Cost, error) {
	// The category must belong to the cost's owner (404 otherwise).
	if _, err := s.categoryRepo.GetByIDForUser(ctx, cost.UserID, cost.CategoryID); err != nil {
		return nil, categoryLookupError(err)
	}
	if err := s.repo.Create(ctx, cost); err != nil {
		return nil, err
	}
	return cost, nil
}

func (s *costService) Get(ctx context.Context, userID, id uuid.UUID) (*model.Cost, error) {
	return s.repo.GetByIDForUser(ctx, userID, id)
}

// Update applies only the fields present in req. The owner (user_id) is
// never taken from the request.
func (s *costService) Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateCostRequest) (*model.Cost, error) {
	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Amount != nil {
		updates["amount"] = *req.Amount
	}
	if req.Currency != nil {
		updates["currency"] = *req.Currency
	}
	if req.IncurredAt != nil {
		updates["incurred_at"] = req.IncurredAt.Time
	}
	if req.CategoryID != nil {
		if _, err := s.categoryRepo.GetByIDForUser(ctx, userID, *req.CategoryID); err != nil {
			return nil, categoryLookupError(err)
		}
		updates["category_id"] = *req.CategoryID
	}
	if len(updates) == 0 {
		return nil, constant.ErrInvalidInput
	}

	if err := s.repo.UpdateFieldsForUser(ctx, userID, id, updates); err != nil {
		return nil, err
	}
	return s.repo.GetByIDForUser(ctx, userID, id)
}

func (s *costService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.DeleteForUser(ctx, userID, id)
}

func (s *costService) ListWithCategory(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Cost, error) {
	return s.repo.ListWithCategory(ctx, userID, limit, offset, filters)
}
