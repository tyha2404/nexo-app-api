package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type RolloverService interface {
	ProcessRolloverForMonth(ctx context.Context, targetMonth, targetYear int) error
	ProcessRolloverForUser(ctx context.Context, userID uuid.UUID, targetMonth, targetYear int) error
}

type rolloverService struct {
	repo   repository.RolloverRepository
	logger *zap.Logger
}

func NewRolloverService(repo repository.RolloverRepository, logger *zap.Logger) RolloverService {
	return &rolloverService{
		repo:   repo,
		logger: logger,
	}
}

func (s *rolloverService) ProcessRolloverForMonth(ctx context.Context, targetMonth, targetYear int) error {
	userIDs, err := s.repo.GetAllActiveUserIDs(ctx)
	if err != nil {
		s.logger.Error("failed to get active users for rollover", zap.Error(err))
		return err
	}

	s.logger.Info("starting monthly rollover process",
		zap.Int("month", targetMonth),
		zap.Int("year", targetYear),
		zap.Int("total_users", len(userIDs)),
	)

	successCount := 0
	errorCount := 0

	for _, userID := range userIDs {
		if err := s.ProcessRolloverForUser(ctx, userID, targetMonth, targetYear); err != nil {
			s.logger.Error("failed to process rollover for user",
				zap.String("user_id", userID.String()),
				zap.Int("month", targetMonth),
				zap.Int("year", targetYear),
				zap.Error(err),
			)
			errorCount++
			continue
		}
		successCount++
	}

	s.logger.Info("completed monthly rollover process",
		zap.Int("month", targetMonth),
		zap.Int("year", targetYear),
		zap.Int("success_count", successCount),
		zap.Int("error_count", errorCount),
	)

	return nil
}

func (s *rolloverService) ProcessRolloverForUser(ctx context.Context, userID uuid.UUID, targetMonth, targetYear int) error {
	targetPeriodStart := time.Date(targetYear, time.Month(targetMonth), 1, 0, 0, 0, 0, time.UTC)

	// 1. Process Monthly Targets (EXPENSE & INVESTMENT)
	var targetsToCreate []model.MonthlyTarget
	targetTypes := []model.TargetType{model.TargetTypeExpense, model.TargetTypeInvestment}

	for _, targetType := range targetTypes {
		existingTarget, err := s.repo.GetTargetForMonth(ctx, userID, targetType, targetMonth, targetYear)
		if err != nil {
			return err
		}

		// If user doesn't have a target for the upcoming month, find the latest historical target
		if existingTarget == nil {
			latestTarget, err := s.repo.GetLatestTargetBefore(ctx, userID, targetType, targetMonth, targetYear)
			if err != nil {
				return err
			}

			if latestTarget != nil {
				targetsToCreate = append(targetsToCreate, model.MonthlyTarget{
					UserID:       userID,
					TargetType:   targetType,
					TargetAmount: latestTarget.TargetAmount,
					Month:        targetMonth,
					Year:         targetYear,
				})
			}
		}
	}

	if len(targetsToCreate) > 0 {
		if err := s.repo.CreateMonthlyTargets(ctx, targetsToCreate); err != nil {
			return err
		}
	}

	// 2. Process Category Budgets
	existingBudgets, err := s.repo.GetMonthlyBudgetsForPeriod(ctx, userID, targetPeriodStart)
	if err != nil {
		return err
	}

	existingCategoryIDs := make(map[uuid.UUID]bool)
	for _, b := range existingBudgets {
		existingCategoryIDs[b.CategoryID] = true
	}

	latestBudgets, err := s.repo.GetLatestMonthlyBudgetsBefore(ctx, userID, targetPeriodStart)
	if err != nil {
		return err
	}

	var budgetsToCreate []model.Budget
	for _, b := range latestBudgets {
		if !existingCategoryIDs[b.CategoryID] {
			budgetsToCreate = append(budgetsToCreate, model.Budget{
				UserID:      userID,
				CategoryID:  b.CategoryID,
				Amount:      b.Amount,
				PeriodType:  "monthly",
				PeriodStart: targetPeriodStart,
			})
		}
	}

	if len(budgetsToCreate) > 0 {
		if err := s.repo.CreateBudgets(ctx, budgetsToCreate); err != nil {
			return err
		}
	}

	return nil
}
