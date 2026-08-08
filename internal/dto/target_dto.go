package dto

type UpsertTargetRequest struct {
	TargetType   string  `json:"targetType" binding:"required,oneof=EXPENSE INVESTMENT"`
	TargetAmount float64 `json:"targetAmount" binding:"required,gt=0"`
	Month        int     `json:"month" binding:"required,min=1,max=12"`
	Year         int     `json:"year" binding:"required,min=2000"`
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
