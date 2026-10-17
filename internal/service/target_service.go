package service

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

type TargetService interface {
	UpsertTarget(ctx context.Context, userID uuid.UUID, req *dto.UpsertTargetRequest) error
	GetSummary(ctx context.Context, userID uuid.UUID, month, year int) (*dto.TargetSummaryResponse, error)
}

type targetService struct {
	repo repository.TargetRepository
}

func NewTargetService(repo repository.TargetRepository) TargetService {
	return &targetService{repo: repo}
}

func (s *targetService) UpsertTarget(ctx context.Context, userID uuid.UUID, req *dto.UpsertTargetRequest) error {
	target := &model.MonthlyTarget{
		UserID:       userID,
		TargetType:   model.TargetType(req.TargetType),
		TargetAmount: req.TargetAmount,
		Month:        req.Month,
		Year:         req.Year,
	}
	return s.repo.UpsertTarget(ctx, target)
}

func (s *targetService) GetSummary(ctx context.Context, userID uuid.UUID, month, year int) (*dto.TargetSummaryResponse, error) {
	// "Today" is evaluated in the app timezone (Vietnam), not the host's.
	now := util.Now()
	if month <= 0 || year <= 0 || month > 12 {
		month = int(now.Month())
		year = now.Year()
	}

	// Dynamic Days calculation
	firstDay, nextMonth := util.MonthRangeOf(year, time.Month(month))
	lastDay := nextMonth.AddDate(0, 0, -1)
	daysInMonth := lastDay.Day()

	currentDay := 1
	if now.Year() == year && int(now.Month()) == month {
		currentDay = now.Day()
	} else if !now.Before(nextMonth) {
		currentDay = daysInMonth
	}

	daysRemaining := daysInMonth - currentDay + 1
	if daysRemaining < 1 {
		daysRemaining = 1
	}

	startOfMonth := firstDay
	endOfMonth := lastDay

	// Fetch targets
	expenseTargetModel, err := s.repo.GetTarget(ctx, userID, model.TargetTypeExpense, month, year)
	if err != nil {
		return nil, err
	}
	expenseTargetAmount := 0.0
	if expenseTargetModel != nil {
		expenseTargetAmount = expenseTargetModel.TargetAmount
	}

	investmentTargetModel, err := s.repo.GetTarget(ctx, userID, model.TargetTypeInvestment, month, year)
	if err != nil {
		return nil, err
	}
	investmentTargetAmount := 0.0
	if investmentTargetModel != nil {
		investmentTargetAmount = investmentTargetModel.TargetAmount
	}

	// Fetch actual spent & invested totals
	spentAmount, err := s.repo.GetMonthlyTotalByCategoryType(ctx, userID, model.CategoryTypeExpense, startOfMonth, endOfMonth)
	if err != nil {
		return nil, err
	}

	investedAmount, err := s.repo.GetMonthlyTotalByCategoryType(ctx, userID, model.CategoryTypeInvestment, startOfMonth, endOfMonth)
	if err != nil {
		return nil, err
	}

	// Expense Summary Calculations
	remainingExpense := expenseTargetAmount - spentAmount
	isOverBudget := remainingExpense < 0
	overspentAmount := 0.0
	dailyAllowance := 0.0

	if isOverBudget {
		overspentAmount = math.Abs(remainingExpense)
		dailyAllowance = 0.0
	} else if expenseTargetAmount > 0 {
		dailyAllowance = math.Max(0, remainingExpense/float64(daysRemaining))
	}

	// Investment Summary Calculations
	remainingInvestment := investmentTargetAmount - investedAmount
	isTargetReached := remainingInvestment <= 0 && investmentTargetAmount > 0
	surplusAmount := 0.0
	if isTargetReached {
		surplusAmount = math.Abs(remainingInvestment)
		remainingInvestment = 0
	} else if remainingInvestment < 0 {
		remainingInvestment = 0
	}

	return &dto.TargetSummaryResponse{
		Month:         month,
		Year:          year,
		DaysInMonth:   daysInMonth,
		CurrentDay:    currentDay,
		DaysRemaining: daysRemaining,
		Expense: dto.ExpenseSummary{
			TargetAmount:    expenseTargetAmount,
			SpentAmount:     spentAmount,
			RemainingAmount: math.Max(0, remainingExpense),
			DailyAllowance:  dailyAllowance,
			IsOverBudget:    isOverBudget,
			OverspentAmount: overspentAmount,
		},
		Investment: dto.InvestmentSummary{
			TargetAmount:    investmentTargetAmount,
			InvestedAmount:  investedAmount,
			RemainingAmount: remainingInvestment,
			IsTargetReached: isTargetReached,
			SurplusAmount:   surplusAmount,
		},
	}, nil
}
