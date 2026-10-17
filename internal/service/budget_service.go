package service

import (
	"context"
	"fmt"

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
	budget, err := s.budgetRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if budget.UserID != userID {
		return nil, fmt.Errorf("budget %w", constant.ErrNotFound)
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

func (s *budgetService) UpdateBudget(ctx context.Context, userID, id uuid.UUID, req dto.UpdateBudgetRequest) (*dto.BudgetResponse, error) {
	budget, err := s.budgetRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if budget.UserID != userID {
		return nil, fmt.Errorf("budget %w", constant.ErrNotFound)
	}

	if req.Amount != nil {
		budget.Amount = *req.Amount
	}
	if req.PeriodType != nil {
		budget.PeriodType = *req.PeriodType
	}

	if err := s.budgetRepo.Update(ctx, budget); err != nil {
		return nil, err
	}
	return dto.ToBudgetResponse(budget), nil
}

func (s *budgetService) DeleteBudget(ctx context.Context, userID, id uuid.UUID) error {
	budget, err := s.budgetRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if budget.UserID != userID {
		return fmt.Errorf("budget %w", constant.ErrNotFound)
	}
	return s.budgetRepo.Delete(ctx, id)
}
