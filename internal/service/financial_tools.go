package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"go.uber.org/zap"
)

// GetFinancialToolDefinitions returns the schema for all financial tools the AI model can call
func GetFinancialToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_financial_overview",
				Description: "Lấy tổng quan tình hình tài chính của người dùng bao gồm tổng số dư ví, tổng thu nhập, tổng chi tiêu, tiền tiết kiệm ròng và tỷ lệ tiết kiệm theo tháng hoặc khoảng ngày.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"month": map[string]interface{}{
							"type":        "string",
							"description": "Tháng cần tra cứu (định dạng YYYY-MM, ví dụ 2026-08). Mặc định là tháng hiện tại.",
						},
						"startDate": map[string]interface{}{
							"type":        "string",
							"description": "Ngày bắt đầu (YYYY-MM-DD)",
						},
						"endDate": map[string]interface{}{
							"type":        "string",
							"description": "Ngày kết thúc (YYYY-MM-DD)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "list_recent_transactions",
				Description: "Tra cứu danh sách các giao dịch thu/chi gần đây của người dùng theo bộ lọc loại giao dịch hoặc khoảng thời gian.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Số lượng giao dịch cần lấy (mặc định 5, tối đa 20)",
						},
						"type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"INCOME", "EXPENSE", "INVESTMENT", "ALL"},
							"description": "Loại giao dịch cần lọc: INCOME (thu nhập), EXPENSE (chi tiêu), INVESTMENT (đầu tư), ALL (tất cả)",
						},
						"startDate": map[string]interface{}{
							"type":        "string",
							"description": "Ngày bắt đầu (YYYY-MM-DD)",
						},
						"endDate": map[string]interface{}{
							"type":        "string",
							"description": "Ngày kết thúc (YYYY-MM-DD)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "create_transaction",
				Description: "Ghi nhận một giao dịch thu chi mới vào hệ thống Nexo cho người dùng (tự động phân loại danh mục và ví).",
				Parameters: map[string]interface{}{
					"type":     "object",
					"required": []string{"amount", "type"},
					"properties": map[string]interface{}{
						"amount": map[string]interface{}{
							"type":        "number",
							"description": "Số tiền giao dịch (VND)",
						},
						"type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"INCOME", "EXPENSE"},
							"description": "Loại giao dịch: EXPENSE (chi tiêu) hoặc INCOME (thu nhập)",
						},
						"categoryName": map[string]interface{}{
							"type":        "string",
							"description": "Tên danh mục (ví dụ: Ăn uống, Mua sắm, Di chuyển, Lương, Thưởng, v.v.)",
						},
						"walletName": map[string]interface{}{
							"type":        "string",
							"description": "Tên ví sử dụng (ví dụ: Tiền mặt, Techcombank, MoMo, v.v.)",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Ghi chú/mô tả chi tiết giao dịch (ví dụ: Ăn trưa phở bò, Mua cà phê, v.v.)",
						},
						"transactionDate": map[string]interface{}{
							"type":        "string",
							"description": "Ngày giao dịch (YYYY-MM-DD), mặc định là hôm nay",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_budget_status",
				Description: "Kiểm tra tiến độ thực hiện và tình hình các ngân sách chi tiêu trong tháng (hạn mức, số tiền đã chi, số tiền còn lại, cảnh báo vượt mức).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"month": map[string]interface{}{
							"type":        "string",
							"description": "Tháng cần kiểm tra (YYYY-MM), mặc định tháng hiện tại",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_debt_summary",
				Description: "Tra cứu báo cáo tổng quan về các khoản nợ phải trả (PAYABLE) và các khoản cho vay cần thu hồi (RECEIVABLE).",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "list_wallets",
				Description: "Xem danh sách tất cả các ví/tài khoản và số dư chi tiết của từng ví của người dùng.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_spending_by_category",
				Description: "Phân tích cơ cấu chi tiêu theo từng danh mục (Breakdown) trong tháng hoặc khoảng ngày.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"month": map[string]interface{}{
							"type":        "string",
							"description": "Tháng cần phân tích (YYYY-MM)",
						},
						"startDate": map[string]interface{}{
							"type":        "string",
							"description": "Ngày bắt đầu (YYYY-MM-DD)",
						},
						"endDate": map[string]interface{}{
							"type":        "string",
							"description": "Ngày kết thúc (YYYY-MM-DD)",
						},
					},
				},
			},
		},
	}
}

// FinancialToolResult represents the execution outcome of a financial tool
type FinancialToolResult struct {
	ToolTitle  string
	ResultJSON string
	ActionCard *dto.ActionCard
}

func (s *chatService) executeFinancialTool(ctx context.Context, userID uuid.UUID, call ToolCall) (*FinancialToolResult, error) {
	name := call.Function.Name
	argsStr := call.Function.Arguments

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
		args = make(map[string]interface{})
	}

	switch name {
	case "get_financial_overview":
		return s.toolGetFinancialOverview(ctx, userID, args)
	case "list_recent_transactions":
		return s.toolListRecentTransactions(ctx, userID, args)
	case "create_transaction":
		return s.toolCreateTransaction(ctx, userID, args)
	case "get_budget_status":
		return s.toolGetBudgetStatus(ctx, userID, args)
	case "get_debt_summary":
		return s.toolGetDebtSummary(ctx, userID, args)
	case "list_wallets":
		return s.toolListWallets(ctx, userID, args)
	case "get_spending_by_category":
		return s.toolGetSpendingByCategory(ctx, userID, args)
	default:
		return &FinancialToolResult{
			ToolTitle:  fmt.Sprintf("Thực thi công cụ %s...", name),
			ResultJSON: fmt.Sprintf(`{"error": "Không tìm thấy công cụ %s"}`, name),
		}, nil
	}
}

func (s *chatService) toolGetFinancialOverview(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	startDate, endDate := resolveDateRange(args)

	summary, err := s.reportService.GetSummary(ctx, userID, startDate, endDate)
	if err != nil {
		s.logger.Error("failed to get financial summary", zap.Error(err))
		return nil, err
	}

	walletSummary, err := s.walletRepo.GetSummaryByUserID(ctx, userID)
	totalBalance := 0.0
	if err == nil && walletSummary != nil {
		totalBalance = walletSummary.TotalBalance
	}

	netSavings := summary.TotalIncome - summary.TotalExpense
	savingsRate := 0.0
	if summary.TotalIncome > 0 {
		savingsRate = (netSavings / summary.TotalIncome) * 100.0
	}

	resultMap := map[string]interface{}{
		"start_date":           startDate,
		"end_date":             endDate,
		"total_income":         summary.TotalIncome,
		"total_expense":        summary.TotalExpense,
		"total_investment":     summary.TotalInvestment,
		"net_savings":          netSavings,
		"savings_rate_percent": math.Round(savingsRate*100) / 100,
		"total_wallet_balance": totalBalance,
	}
	resultBytes, _ := json.Marshal(resultMap)

	actionCard := &dto.ActionCard{
		ActionType:  "FINANCIAL_SUMMARY",
		Title:       "Tổng quan Tài chính",
		Description: fmt.Sprintf("Thu: %s ₫ | Chi: %s ₫ | Tiết kiệm: %s ₫ (%.1f%%)", formatVND(summary.TotalIncome), formatVND(summary.TotalExpense), formatVND(netSavings), savingsRate),
		Data:        resultMap,
	}

	return &FinancialToolResult{
		ToolTitle:  "Đang thống kê tổng quan tài chính...",
		ResultJSON: string(resultBytes),
		ActionCard: actionCard,
	}, nil
}

func (s *chatService) toolListRecentTransactions(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	limit := 5
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
		if limit > 20 {
			limit = 20
		}
	}

	filters := make(map[string]interface{})
	if t, ok := args["type"].(string); ok && t != "" && t != "ALL" {
		filters["type"] = t
	}
	if sd, ok := args["startDate"].(string); ok && sd != "" {
		filters["startDate"] = sd
	}
	if ed, ok := args["endDate"].(string); ok && ed != "" {
		filters["endDate"] = ed
	}

	transactions, total, _, err := s.transactionService.ListTransactions(ctx, userID, 1, limit, filters)
	if err != nil {
		return nil, err
	}

	type SimpleTxn struct {
		ID          uuid.UUID `json:"id"`
		Date        string    `json:"date"`
		Type        string    `json:"type"`
		Amount      float64   `json:"amount"`
		Category    string    `json:"category"`
		Description string    `json:"description"`
	}

	items := make([]SimpleTxn, 0, len(transactions))
	for _, tx := range transactions {
		desc := ""
		if tx.Description != nil {
			desc = *tx.Description
		}
		items = append(items, SimpleTxn{
			ID:          tx.ID,
			Date:        tx.TransactionDate,
			Type:        tx.Type,
			Amount:      tx.Amount,
			Category:    tx.CategoryName,
			Description: desc,
		})
	}

	resultMap := map[string]interface{}{
		"total_count":  total,
		"return_count": len(items),
		"transactions": items,
	}
	resultBytes, _ := json.Marshal(resultMap)

	return &FinancialToolResult{
		ToolTitle:  "Đang tra cứu lịch sử giao dịch...",
		ResultJSON: string(resultBytes),
	}, nil
}

func (s *chatService) toolCreateTransaction(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	amount, _ := args["amount"].(float64)
	if amount <= 0 {
		return &FinancialToolResult{
			ToolTitle:  "Đang ghi nhận giao dịch mới...",
			ResultJSON: `{"error": "Số tiền giao dịch (amount) phải lớn hơn 0"}`,
		}, nil
	}

	txnTypeStr := "EXPENSE"
	if t, ok := args["type"].(string); ok && strings.ToUpper(t) == "INCOME" {
		txnTypeStr = "INCOME"
	}

	categoryName, _ := args["categoryName"].(string)
	walletName, _ := args["walletName"].(string)
	description, _ := args["description"].(string)
	txnDateStr, _ := args["transactionDate"].(string)

	txnDate := time.Now()
	if txnDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", txnDateStr); err == nil {
			txnDate = parsed
		}
	}

	// 1. Resolve Category
	categories, err := s.categoryRepo.List(ctx, userID, txnTypeStr, 100, 0)
	if err != nil {
		return nil, err
	}

	var targetCategory *model.Category
	if categoryName != "" {
		for i := range categories {
			if strings.EqualFold(categories[i].Name, categoryName) || strings.Contains(strings.ToLower(categories[i].Name), strings.ToLower(categoryName)) {
				targetCategory = &categories[i]
				break
			}
		}
	}

	// If not found, pick the first existing or create a new category
	if targetCategory == nil {
		if len(categories) > 0 {
			targetCategory = &categories[0]
		} else {
			defaultName := "Khác"
			if categoryName != "" {
				defaultName = categoryName
			}
			newCat := &model.Category{
				UserID: userID,
				Name:   defaultName,
				Type:   model.CategoryType(txnTypeStr),
			}
			if err := s.categoryRepo.Create(ctx, newCat); err != nil {
				return nil, err
			}
			targetCategory = newCat
		}
	}

	// 2. Resolve Wallet
	wallets, _ := s.walletRepo.ListByUserID(ctx, userID)
	var targetWalletID *uuid.UUID
	var targetWalletName string
	if len(wallets) > 0 {
		var matched *model.Wallet
		if walletName != "" {
			for i := range wallets {
				if strings.EqualFold(wallets[i].Name, walletName) || strings.Contains(strings.ToLower(wallets[i].Name), strings.ToLower(walletName)) {
					matched = &wallets[i]
					break
				}
			}
		}
		if matched == nil {
			matched = &wallets[0]
		}
		targetWalletID = &matched.ID
		targetWalletName = matched.Name
	}

	if description == "" {
		description = targetCategory.Name
	}

	createReq := dto.CreateTransactionRequest{
		CategoryID:      targetCategory.ID,
		WalletID:        targetWalletID,
		Amount:          amount,
		Type:            txnTypeStr,
		Description:     &description,
		TransactionDate: dto.CustomTime{Time: txnDate},
	}

	createdTx, err := s.transactionService.CreateTransaction(ctx, userID, createReq)
	if err != nil {
		return nil, err
	}

	typeNameVi := "Chi tiêu"
	if txnTypeStr == "INCOME" {
		typeNameVi = "Thu nhập"
	}

	actionCard := &dto.ActionCard{
		ActionType:  "TRANSACTION_CREATED",
		Title:       fmt.Sprintf("Đã ghi nhận %s", typeNameVi),
		Description: fmt.Sprintf("%s ₫ - %s (%s)", formatVND(amount), description, targetCategory.Name),
		Data: map[string]interface{}{
			"id":          createdTx.ID,
			"amount":      amount,
			"type":        txnTypeStr,
			"category":    targetCategory.Name,
			"wallet":      targetWalletName,
			"description": description,
			"date":        txnDate.Format("2006-01-02"),
		},
	}

	resultMap := map[string]interface{}{
		"success":       true,
		"transactionId": createdTx.ID,
		"amount":        amount,
		"type":          txnTypeStr,
		"category":      targetCategory.Name,
		"wallet":        targetWalletName,
		"description":   description,
		"date":          txnDate.Format("2006-01-02"),
		"message":       "Giao dịch đã được lưu thành công vào hệ thống Nexo.",
	}
	resultBytes, _ := json.Marshal(resultMap)

	return &FinancialToolResult{
		ToolTitle:  "Đang ghi nhận giao dịch mới...",
		ResultJSON: string(resultBytes),
		ActionCard: actionCard,
	}, nil
}

func (s *chatService) toolGetBudgetStatus(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	startDate, endDate := resolveDateRange(args)

	budgets, _, err := s.budgetService.ListBudgets(ctx, userID, 1, 50)
	if err != nil {
		return nil, err
	}

	type BudgetStatusItem struct {
		Category   string  `json:"category"`
		Limit      float64 `json:"limit"`
		Spent      float64 `json:"spent"`
		Remaining  float64 `json:"remaining"`
		Percent    float64 `json:"percentage"`
		Status     string  `json:"status"` // "OK", "WARNING", "EXCEEDED"
	}

	var items []BudgetStatusItem
	var exceededCategories []string

	for _, b := range budgets {
		catName := b.CategoryName

		// Calculate spent in category for date range
		filters := map[string]interface{}{
			"type":       string(model.TransactionTypeExpense),
			"categoryId": b.CategoryID.String(),
			"startDate":  startDate,
			"endDate":    endDate,
		}
		txns, _, err := s.transactionRepo.ListByUserID(ctx, userID, 1000, 0, filters)
		var spent float64
		if err == nil {
			for _, t := range txns {
				spent += t.Amount
			}
		}

		remaining := b.Amount - spent
		percent := 0.0
		if b.Amount > 0 {
			percent = (spent / b.Amount) * 100.0
		}

		status := "OK"
		if percent >= 100 {
			status = "EXCEEDED"
			exceededCategories = append(exceededCategories, catName)
		} else if percent >= 80 {
			status = "WARNING"
		}

		items = append(items, BudgetStatusItem{
			Category:  catName,
			Limit:     b.Amount,
			Spent:     spent,
			Remaining: remaining,
			Percent:   math.Round(percent*10) / 10,
			Status:    status,
		})
	}

	var actionCard *dto.ActionCard
	if len(exceededCategories) > 0 {
		actionCard = &dto.ActionCard{
			ActionType:  "BUDGET_ALERT",
			Title:       "Cảnh báo vượt Ngân sách",
			Description: fmt.Sprintf("Các danh mục đã vượt hạn mức: %s", strings.Join(exceededCategories, ", ")),
			Data:        items,
		}
	}

	resultMap := map[string]interface{}{
		"start_date":   startDate,
		"end_date":     endDate,
		"budget_count": len(items),
		"budgets":      items,
	}
	resultBytes, _ := json.Marshal(resultMap)

	return &FinancialToolResult{
		ToolTitle:  "Đang kiểm tra tiến độ ngân sách...",
		ResultJSON: string(resultBytes),
		ActionCard: actionCard,
	}, nil
}

func (s *chatService) toolGetDebtSummary(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	summary, err := s.debtService.GetDebtSummary(ctx, userID)
	if err != nil {
		return nil, err
	}

	debts, _ := s.debtService.GetDebts(ctx, userID, "", "")

	resultMap := map[string]interface{}{
		"total_payable":      summary.TotalPayable,
		"total_receivable":   summary.TotalReceivable,
		"overdue_count":      summary.OverdueCount,
		"pending_count":      summary.PendingCount,
		"active_debts_count": len(debts),
		"debts":              debts,
	}
	resultBytes, _ := json.Marshal(resultMap)

	return &FinancialToolResult{
		ToolTitle:  "Đang tổng hợp danh sách nợ & cho vay...",
		ResultJSON: string(resultBytes),
	}, nil
}

func (s *chatService) toolListWallets(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	walletSummary, err := s.walletService.GetWallets(ctx, userID)
	if err != nil {
		return nil, err
	}

	resultBytes, _ := json.Marshal(walletSummary)

	return &FinancialToolResult{
		ToolTitle:  "Đang kiểm tra danh sách ví tài khoản...",
		ResultJSON: string(resultBytes),
	}, nil
}

func (s *chatService) toolGetSpendingByCategory(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	startDate, endDate := resolveDateRange(args)

	breakdown, err := s.reportService.GetCategoryBreakdown(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	resultMap := map[string]interface{}{
		"start_date":    startDate,
		"end_date":      endDate,
		"total_expense": breakdown.TotalExpense,
		"categories":    breakdown.Items,
	}
	resultBytes, _ := json.Marshal(resultMap)

	return &FinancialToolResult{
		ToolTitle:  "Đang phân tích cơ cấu chi tiêu...",
		ResultJSON: string(resultBytes),
	}, nil
}

func resolveDateRange(args map[string]interface{}) (startDate, endDate string) {
	now := time.Now()
	if m, ok := args["month"].(string); ok && m != "" {
		if t, err := time.Parse("2006-01", m); err == nil {
			start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
			end := start.AddDate(0, 1, -1)
			return start.Format("2006-01-02"), end.Format("2006-01-02")
		}
	}

	if sd, ok := args["startDate"].(string); ok && sd != "" {
		startDate = sd
	} else {
		// Default to first day of current month
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).Format("2006-01-02")
	}

	if ed, ok := args["endDate"].(string); ok && ed != "" {
		endDate = ed
	} else {
		endDate = now.Format("2006-01-02")
	}

	return startDate, endDate
}

func formatVND(amount float64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	n := int64(math.Round(amount))
	str := fmt.Sprintf("%d", n)
	var parts []string
	for len(str) > 3 {
		parts = append([]string{str[len(str)-3:]}, parts...)
		str = str[:len(str)-3]
	}
	if len(str) > 0 {
		parts = append([]string{str}, parts...)
	}
	return sign + strings.Join(parts, ".")
}
