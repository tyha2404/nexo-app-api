package service

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
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

// validateTarget enforces the same rules as dto.UpsertTargetRequest's tags,
// for callers that do not go through the HTTP validator (the AI tools).
func validateTarget(targetType string, amount float64, month, year int) error {
	if targetType != string(model.TargetTypeExpense) && targetType != string(model.TargetTypeInvestment) {
		return constant.NewKindError(constant.ErrInvalidInput, "loại mục tiêu phải là EXPENSE hoặc INVESTMENT")
	}
	if amount <= 0 || amount > maxTargetAmount {
		return constant.NewKindError(constant.ErrInvalidInput, "số tiền mục tiêu không hợp lệ")
	}
	if month < 1 || month > 12 || year < 2000 || year > 2100 {
		return constant.NewKindError(constant.ErrInvalidInput, "tháng/năm không hợp lệ")
	}
	return nil
}

// maxTargetAmount fits monthly_targets.target_amount numeric(12,2).
const maxTargetAmount = 9999999999

func (s *targetService) UpsertTarget(ctx context.Context, userID uuid.UUID, req *dto.UpsertTargetRequest) error {
	if err := validateTarget(req.TargetType, req.TargetAmount, req.Month, req.Year); err != nil {
		return err
	}
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
	// A missing month/year is the current one; an invalid month means the
	// current month of the current year.
	now := util.Now()
	if month <= 0 {
		month = int(now.Month())
	}
	if year <= 0 {
		year = now.Year()
	}
	if month > 12 {
		month, year = int(now.Month()), now.Year()
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
	expenseTargetAmount, err := s.targetAmount(ctx, userID, model.TargetTypeExpense, month, year)
	if err != nil {
		return nil, err
	}
	investmentTargetAmount, err := s.targetAmount(ctx, userID, model.TargetTypeInvestment, month, year)
	if err != nil {
		return nil, err
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

// targetAmount is the user's target of that type for the month, 0 when none is set.
func (s *targetService) targetAmount(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (float64, error) {
	target, err := s.repo.GetTarget(ctx, userID, targetType, month, year)
	if errors.Is(err, constant.ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return target.TargetAmount, nil
}
