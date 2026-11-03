package dto

type UpsertTargetRequest struct {
	TargetType   string  `json:"targetType" validate:"required,oneof=EXPENSE INVESTMENT"`
	TargetAmount float64 `json:"targetAmount" validate:"required,gt=0,lte=9999999999"`
	Month        int     `json:"month" validate:"required,min=1,max=12"`
	Year         int     `json:"year" validate:"required,min=2000,max=2100"`
}

type ExpenseSummary struct {
	TargetAmount    float64 `json:"targetAmount"`
	SpentAmount     float64 `json:"spentAmount"`
	RemainingAmount float64 `json:"remainingAmount"`
	DailyAllowance  float64 `json:"dailyAllowance"`
	IsOverBudget    bool    `json:"isOverBudget"`
	OverspentAmount float64 `json:"overspentAmount"`
}

type InvestmentSummary struct {
	TargetAmount    float64 `json:"targetAmount"`
	InvestedAmount  float64 `json:"investedAmount"`
	RemainingAmount float64 `json:"remainingAmount"`
	IsTargetReached bool    `json:"isTargetReached"`
	SurplusAmount   float64 `json:"surplusAmount"`
}

type TargetSummaryResponse struct {
	Month         int               `json:"month"`
	Year          int               `json:"year"`
	DaysInMonth   int               `json:"daysInMonth"`
	CurrentDay    int               `json:"currentDay"`
	DaysRemaining int               `json:"daysRemaining"`
	Expense       ExpenseSummary    `json:"expense"`
	Investment    InvestmentSummary `json:"investment"`
}
