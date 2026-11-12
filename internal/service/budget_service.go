package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type BudgetService interface {
	CreateBudget(ctx context.Context, userID uuid.UUID, req dto.CreateBudgetRequest) (*dto.BudgetResponse, error)
	GetBudget(ctx context.Context, userID, id uuid.UUID) (*dto.BudgetResponse, error)
	ListBudgets(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.BudgetResponse, int64, error)
	// ListActiveBudgets returns the budget in force on the day of at for each
	// category (at most one per category: the most recent period wins).
	ListActiveBudgets(ctx context.Context, userID uuid.UUID, at time.Time) ([]dto.BudgetResponse, error)
	UpdateBudget(ctx context.Context, userID, id uuid.UUID, req dto.UpdateBudgetRequest) (*dto.BudgetResponse, error)
	DeleteBudget(ctx context.Context, userID, id uuid.UUID) error
}

type budgetService struct {
	budgetRepo   repository.BudgetRepository
	categoryRepo repository.CategoryRepo
}

func NewBudgetService(budgetRepo repository.BudgetRepository, categoryRepo repository.CategoryRepo) BudgetService {
	return &budgetService{
		budgetRepo:   budgetRepo,
		categoryRepo: categoryRepo,
	}
}

func (s *budgetService) CreateBudget(ctx context.Context, userID uuid.UUID, req dto.CreateBudgetRequest) (*dto.BudgetResponse, error) {
	// Callers other than the HTTP handler (AI tools) skip DTO validation.
	if !isBudgetPeriod(req.PeriodType) {
		return nil, fmt.Errorf("%w: period type must be %q or %q", constant.ErrInvalidInput, model.BudgetPeriodMonthly, model.BudgetPeriodYearly)
	}
	category, err := s.categoryRepo.GetByIDForUser(ctx, userID, req.CategoryID)
	if err != nil {
		return nil, categoryLookupError(err)
	}

	budget := &model.Budget{
		UserID:      userID,
		CategoryID:  req.CategoryID,
		Amount:      req.Amount,
		PeriodType:  req.PeriodType,
		PeriodStart: req.PeriodStart,
	}

	if err := s.budgetRepo.Create(ctx, budget); err != nil {
		return nil, err
	}

	budget.Category = *category

	return dto.ToBudgetResponse(budget), nil
}

func (s *budgetService) GetBudget(ctx context.Context, userID, id uuid.UUID) (*dto.BudgetResponse, error) {
	budget, err := s.budgetRepo.GetByIDForUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return dto.ToBudgetResponse(budget), nil
}

func (s *budgetService) ListBudgets(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.BudgetResponse, int64, error) {
	offset := (page - 1) * limit
	budgets, total, err := s.budgetRepo.ListByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	var res []dto.BudgetResponse
	for _, b := range budgets {
		res = append(res, *dto.ToBudgetResponse(&b))
	}
	return res, total, nil
}

func (s *budgetService) ListActiveBudgets(ctx context.Context, userID uuid.UUID, at time.Time) ([]dto.BudgetResponse, error) {
	budgets, err := s.budgetRepo.ListActiveByUserID(ctx, userID, at)
	if err != nil {
		return nil, err
	}
	seen := make(map[uuid.UUID]bool, len(budgets))
	res := make([]dto.BudgetResponse, 0, len(budgets))
	for i := range budgets {
		if seen[budgets[i].CategoryID] {
			continue // an older overlapping period (e.g. a yearly budget under a monthly one)
		}
		seen[budgets[i].CategoryID] = true
		res = append(res, *dto.ToBudgetResponse(&budgets[i]))
	}
	return res, nil
}

func (s *budgetService) UpdateBudget(ctx context.Context, userID, id uuid.UUID, req dto.UpdateBudgetRequest) (*dto.BudgetResponse, error) {
	budget, err := s.budgetRepo.GetByIDForUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	if req.Amount != nil {
		budget.Amount = *req.Amount
	}
	if req.PeriodType != nil {
		if !isBudgetPeriod(*req.PeriodType) {
			return nil, fmt.Errorf("%w: period type must be %q or %q", constant.ErrInvalidInput, model.BudgetPeriodMonthly, model.BudgetPeriodYearly)
		}
		budget.PeriodType = *req.PeriodType
	}

	if err := s.budgetRepo.Update(ctx, budget); err != nil {
		return nil, err
	}
	return dto.ToBudgetResponse(budget), nil
}

func (s *budgetService) DeleteBudget(ctx context.Context, userID, id uuid.UUID) error {
	return s.budgetRepo.Delete(ctx, userID, id)
}

func isBudgetPeriod(p string) bool {
	return p == model.BudgetPeriodMonthly || p == model.BudgetPeriodYearly
}
