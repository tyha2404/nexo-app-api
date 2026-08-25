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
	defs := []ToolDefinition{
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
				Name:        "create_category",
				Description: "Tạo một danh mục thu/chi/đầu tư mới cho người dùng khi người dùng yêu cầu hoặc khi danh mục hiện có không đáp ứng được.",
				Parameters: map[string]interface{}{
					"type":     "object",
					"required": []string{"name"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Tên danh mục mới (ví dụ: Học tập, Thú cưng, Tiền điện, Thưởng dự án, v.v.)",
						},
						"type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"EXPENSE", "INCOME", "INVESTMENT"},
							"description": "Loại danh mục: EXPENSE (chi tiêu - mặc định), INCOME (thu nhập), INVESTMENT (đầu tư)",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Mô tả chi tiết về danh mục (tùy chọn)",
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
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "transfer_between_wallets",
				Description: "Thực hiện chuyển tiền nội bộ giữa 2 ví/tài khoản (ví dụ rút tiền từ ngân hàng về tiền mặt, chuyển từ Techcombank sang MoMo).",
				Parameters: map[string]interface{}{
					"type":     "object",
					"required": []string{"amount", "fromWalletName", "toWalletName"},
					"properties": map[string]interface{}{
						"amount": map[string]interface{}{
							"type":        "number",
							"description": "Số tiền cần chuyển (VND)",
						},
						"fromWalletName": map[string]interface{}{
							"type":        "string",
							"description": "Tên ví nguồn chuyển đi (ví dụ: Techcombank, VPBank, Tiền mặt, v.v.)",
						},
						"toWalletName": map[string]interface{}{
							"type":        "string",
							"description": "Tên ví đích nhận tiền (ví dụ: MoMo, Tiền mặt, ZaloPay, v.v.)",
						},
						"fee": map[string]interface{}{
							"type":        "number",
							"description": "Phí chuyển khoản nếu có (VND, mặc định 0)",
						},
						"notes": map[string]interface{}{
							"type":        "string",
							"description": "Ghi chú chuyển khoản",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "create_debt",
				Description: "Ghi nhận một khoản nợ mới (nợ phải trả PAYABLE hoặc cho người khác vay mượn RECEIVABLE).",
				Parameters: map[string]interface{}{
					"type":     "object",
					"required": []string{"title", "totalAmount", "type"},
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Tên khoản nợ/người vay (ví dụ: Cho Nam mượn tiền, Vay anh Tuấn, Mua trả góp iPhone)",
						},
						"totalAmount": map[string]interface{}{
							"type":        "number",
							"description": "Tổng số tiền nợ (VND)",
						},
						"type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"PAYABLE", "RECEIVABLE"},
							"description": "Loại nợ: PAYABLE (mình nợ người khác / phải trả), RECEIVABLE (người khác nợ mình / cần thu hồi)",
						},
						"dueDate": map[string]interface{}{
							"type":        "string",
							"description": "Hạn trả nợ (YYYY-MM-DD)",
						},
						"notes": map[string]interface{}{
							"type":        "string",
							"description": "Ghi chú thêm về khoản nợ",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "record_debt_repayment",
				Description: "Ghi nhận một lần trả nợ hoặc thu hồi nợ (toàn phần hoặc một phần) cho một khoản nợ hiện có.",
				Parameters: map[string]interface{}{
					"type":     "object",
					"required": []string{"debtTitle", "amount"},
					"properties": map[string]interface{}{
						"debtTitle": map[string]interface{}{
							"type":        "string",
							"description": "Tên khoản nợ cần trả hoặc tên người trả nợ (ví dụ: Nam, Anh Tuấn)",
						},
						"amount": map[string]interface{}{
							"type":        "number",
							"description": "Số tiền trả nợ đợt này (VND)",
						},
						"walletName": map[string]interface{}{
							"type":        "string",
							"description": "Tên ví nhận tiền thu hồi hoặc ví dùng để trả nợ",
						},
						"notes": map[string]interface{}{
							"type":        "string",
							"description": "Ghi chú thanh toán",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "set_budget",
				Description: "Tạo mới hoặc điều chỉnh hạn mức ngân sách chi tiêu hàng tháng cho một danh mục.",
				Parameters: map[string]interface{}{
					"type":     "object",
					"required": []string{"categoryName", "amount"},
					"properties": map[string]interface{}{
						"categoryName": map[string]interface{}{
							"type":        "string",
							"description": "Tên danh mục cần đặt ngân sách (ví dụ: Ăn uống, Mua sắm, Di chuyển)",
						},
						"amount": map[string]interface{}{
							"type":        "number",
							"description": "Hạn mức ngân sách (VND)",
						},
						"periodStart": map[string]interface{}{
							"type":        "string",
							"description": "Ngày bắt đầu chu kỳ (YYYY-MM-DD), mặc định ngày 1 đầu tháng",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "compare_financial_periods",
				Description: "So sánh tình hình thu/chi giữa 2 tháng khác nhau để xem biến động tăng giảm các danh mục.",
				Parameters: map[string]interface{}{
					"type":     "object",
					"required": []string{"firstMonth", "secondMonth"},
					"properties": map[string]interface{}{
						"firstMonth": map[string]interface{}{
							"type":        "string",
							"description": "Tháng thứ nhất cần so sánh (YYYY-MM, ví dụ 2026-07)",
						},
						"secondMonth": map[string]interface{}{
							"type":        "string",
							"description": "Tháng thứ hai cần so sánh (YYYY-MM, ví dụ 2026-08)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_financial_targets",
				Description: "Kiểm tra tiến độ thực hiện các mục tiêu tài chính tháng (hạn mức chi tiêu tối đa hoặc mục tiêu tiết kiệm, số tiền đã đạt, tỷ lệ hoàn thành).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"month": map[string]interface{}{
							"type":        "integer",
							"description": "Tháng (1-12), mặc định là tháng hiện tại",
						},
						"year": map[string]interface{}{
							"type":        "integer",
							"description": "Năm (YYYY), mặc định là năm hiện tại",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "set_financial_target",
				Description: "Thiết lập hoặc cập nhật mục tiêu tài chính cho tháng (Mục tiêu tiết kiệm SAVINGS hoặc Hạn mức chi tiêu SPENDING_LIMIT).",
				Parameters: map[string]interface{}{
					"type":     "object",
					"required": []string{"targetType", "targetAmount"},
					"properties": map[string]interface{}{
						"targetType": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"SAVINGS", "SPENDING_LIMIT"},
							"description": "Loại mục tiêu: SAVINGS (tiết kiệm tích lũy) hoặc SPENDING_LIMIT (hạn mức chi tiêu tối đa)",
						},
						"targetAmount": map[string]interface{}{
							"type":        "number",
							"description": "Số tiền mục tiêu (VND)",
						},
						"month": map[string]interface{}{
							"type":        "integer",
							"description": "Tháng áp dụng (1-12), mặc định tháng hiện tại",
						},
						"year": map[string]interface{}{
							"type":        "integer",
							"description": "Năm áp dụng (YYYY), mặc định năm hiện tại",
						},
					},
				},
			},
		},
	}

	return append(defs, getReadOnlyToolDefinitions()...)
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

	if res, handled := s.executeReadOnlyFinancialTool(ctx, userID, name, args); handled {
		return res, nil
	}

	switch name {
	case "get_financial_overview":
		return s.toolGetFinancialOverview(ctx, userID, args)
	case "list_recent_transactions":
		return s.toolListRecentTransactions(ctx, userID, args)
	case "create_transaction":
		return s.toolCreateTransaction(ctx, userID, args)
	case "create_category":
		return s.toolCreateCategory(ctx, userID, args)
	case "get_budget_status":
		return s.toolGetBudgetStatus(ctx, userID, args)
	case "get_debt_summary":
		return s.toolGetDebtSummary(ctx, userID, args)
	case "list_wallets":
		return s.toolListWallets(ctx, userID, args)
	case "get_spending_by_category":
		return s.toolGetSpendingByCategory(ctx, userID, args)
	case "transfer_between_wallets":
		return s.toolTransferBetweenWallets(ctx, userID, args)
	case "create_debt":
		return s.toolCreateDebt(ctx, userID, args)
	case "record_debt_repayment":
		return s.toolRecordDebtRepayment(ctx, userID, args)
	case "set_budget":
		return s.toolSetBudget(ctx, userID, args)
	case "compare_financial_periods":
		return s.toolCompareFinancialPeriods(ctx, userID, args)
	case "get_financial_targets":
		return s.toolGetFinancialTargets(ctx, userID, args)
	case "set_financial_target":
		return s.toolSetFinancialTarget(ctx, userID, args)
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

	// 1. Resolve Category: Find existing or automatically create new category if not matched
	categories, err := s.categoryRepo.List(ctx, userID, txnTypeStr, 100, 0)
	if err != nil {
		return nil, err
	}

	var targetCategory *model.Category
	var isNewCategoryCreated bool
	if categoryName != "" {
		for i := range categories {
			if strings.EqualFold(categories[i].Name, categoryName) || strings.Contains(strings.ToLower(categories[i].Name), strings.ToLower(categoryName)) {
				targetCategory = &categories[i]
				break
			}
		}

		// If user specified a category name but none matched, create it dynamically
		if targetCategory == nil {
			newCat := &model.Category{
				UserID: userID,
				Name:   strings.TrimSpace(categoryName),
				Type:   model.CategoryType(txnTypeStr),
			}
			if err := s.categoryRepo.Create(ctx, newCat); err != nil {
				return nil, err
			}
			targetCategory = newCat
			isNewCategoryCreated = true
		}
	}

	// If no category specified and none created, pick the first existing or create "Khác"
	if targetCategory == nil {
		if len(categories) > 0 {
			targetCategory = &categories[0]
		} else {
			newCat := &model.Category{
				UserID: userID,
				Name:   "Khác",
				Type:   model.CategoryType(txnTypeStr),
			}
			if err := s.categoryRepo.Create(ctx, newCat); err != nil {
				return nil, err
			}
			targetCategory = newCat
			isNewCategoryCreated = true
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

	cardDesc := fmt.Sprintf("%s ₫ - %s (%s)", formatVND(amount), description, targetCategory.Name)
	if isNewCategoryCreated {
		cardDesc = fmt.Sprintf("%s ₫ - %s (Tạo mới danh mục: %s)", formatVND(amount), description, targetCategory.Name)
	}

	actionCard := &dto.ActionCard{
		ActionType:  "TRANSACTION_CREATED",
		Title:       fmt.Sprintf("Đã ghi nhận %s", typeNameVi),
		Description: cardDesc,
		Data: map[string]interface{}{
			"id":                   createdTx.ID,
			"amount":               amount,
			"type":                 txnTypeStr,
			"category":             targetCategory.Name,
			"wallet":               targetWalletName,
			"description":          description,
			"date":                 txnDate.Format("2006-01-02"),
			"isNewCategoryCreated": isNewCategoryCreated,
		},
	}

	resultMap := map[string]interface{}{
		"success":              true,
		"transactionId":        createdTx.ID,
		"amount":               amount,
		"type":                 txnTypeStr,
		"category":             targetCategory.Name,
		"wallet":               targetWalletName,
		"description":          description,
		"date":                 txnDate.Format("2006-01-02"),
		"isNewCategoryCreated": isNewCategoryCreated,
		"message":              "Giao dịch đã được lưu thành công vào hệ thống Nexo.",
	}
	resultBytes, _ := json.Marshal(resultMap)

	return &FinancialToolResult{
		ToolTitle:  "Đang ghi nhận giao dịch mới...",
		ResultJSON: string(resultBytes),
		ActionCard: actionCard,
	}, nil
}

func (s *chatService) toolCreateCategory(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	name, _ := args["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return &FinancialToolResult{
			ToolTitle:  "Tạo danh mục mới",
			ResultJSON: `{"error": "Tên danh mục (name) không được để trống"}`,
		}, nil
	}

	catTypeStr := "EXPENSE"
	if t, ok := args["type"].(string); ok && t != "" {
		upper := strings.ToUpper(strings.TrimSpace(t))
		if upper == "INCOME" || upper == "INVESTMENT" || upper == "EXPENSE" {
			catTypeStr = upper
		}
	}

	description, _ := args["description"].(string)
	var descPtr *string
	if strings.TrimSpace(description) != "" {
		descTrimmed := strings.TrimSpace(description)
		descPtr = &descTrimmed
	}

	// Check if existing category matches by name
	existingCategories, err := s.categoryRepo.List(ctx, userID, catTypeStr, 200, 0)
	if err == nil {
		for _, cat := range existingCategories {
			if strings.EqualFold(cat.Name, name) {
				typeVi := "Chi tiêu"
				if cat.Type == model.CategoryTypeIncome {
					typeVi = "Thu nhập"
				} else if cat.Type == model.CategoryTypeInvestment {
					typeVi = "Đầu tư"
				}

				actionCard := &dto.ActionCard{
					ActionType:  "CATEGORY_CREATED",
					Title:       "Danh mục đã tồn tại",
					Description: fmt.Sprintf("Danh mục \"%s\" (%s) đã có trong hệ thống", cat.Name, typeVi),
					Data: map[string]interface{}{
						"id":          cat.ID,
						"name":        cat.Name,
						"type":        string(cat.Type),
						"isDuplicate": true,
					},
				}

				resMap := map[string]interface{}{
					"success":     true,
					"categoryId":  cat.ID,
					"name":        cat.Name,
					"type":        string(cat.Type),
					"isDuplicate": true,
					"message":     fmt.Sprintf("Danh mục \"%s\" đã tồn tại sẵn trong hệ thống.", cat.Name),
				}
				resBytes, _ := json.Marshal(resMap)

				return &FinancialToolResult{
					ToolTitle:  "Đang kiểm tra danh mục...",
					ResultJSON: string(resBytes),
					ActionCard: actionCard,
				}, nil
			}
		}
	}

	newCat := &model.Category{
		UserID:      userID,
		Name:        name,
		Type:        model.CategoryType(catTypeStr),
		Description: descPtr,
	}

	if err := s.categoryRepo.Create(ctx, newCat); err != nil {
		s.logger.Error("failed to create category from AI tool", zap.Error(err))
		return &FinancialToolResult{
			ToolTitle:  "Tạo danh mục mới thất bại",
			ResultJSON: fmt.Sprintf(`{"error": "Không thể tạo danh mục: %s"}`, err.Error()),
		}, nil
	}

	typeVi := "Chi tiêu"
	if newCat.Type == model.CategoryTypeIncome {
		typeVi = "Thu nhập"
	} else if newCat.Type == model.CategoryTypeInvestment {
		typeVi = "Đầu tư"
	}

	actionCard := &dto.ActionCard{
		ActionType:  "CATEGORY_CREATED",
		Title:       "Đã tạo danh mục mới",
		Description: fmt.Sprintf("Danh mục: %s (%s)", newCat.Name, typeVi),
		Data: map[string]interface{}{
			"id":          newCat.ID,
			"name":        newCat.Name,
			"type":        string(newCat.Type),
			"description": description,
		},
	}

	resMap := map[string]interface{}{
		"success":     true,
		"categoryId":  newCat.ID,
		"name":        newCat.Name,
		"type":        string(newCat.Type),
		"description": description,
		"message":     fmt.Sprintf("Đã tạo thành công danh mục \"%s\" (%s).", newCat.Name, typeVi),
	}
	resBytes, _ := json.Marshal(resMap)

	return &FinancialToolResult{
		ToolTitle:  "Đang tạo danh mục mới...",
		ResultJSON: string(resBytes),
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

func (s *chatService) toolTransferBetweenWallets(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	amount, _ := args["amount"].(float64)
	if amount <= 0 {
		return &FinancialToolResult{
			ToolTitle:  "Lỗi tham số chuyển tiền",
			ResultJSON: `{"error": "Số tiền chuyển phải lớn hơn 0"}`,
		}, nil
	}

	fromName, _ := args["fromWalletName"].(string)
	toName, _ := args["toWalletName"].(string)
	notes, _ := args["notes"].(string)
	fee, _ := args["fee"].(float64)

	wallets, err := s.walletRepo.ListByUserID(ctx, userID)
	if err != nil || len(wallets) == 0 {
		return nil, fmt.Errorf("không tìm thấy danh sách ví của bạn")
	}

	var fromWallet, toWallet *model.Wallet
	for i := range wallets {
		w := &wallets[i]
		if fromWallet == nil && fromName != "" && (strings.EqualFold(w.Name, fromName) || strings.Contains(strings.ToLower(w.Name), strings.ToLower(fromName))) {
			fromWallet = w
		}
		if toWallet == nil && toName != "" && (strings.EqualFold(w.Name, toName) || strings.Contains(strings.ToLower(w.Name), strings.ToLower(toName))) {
			toWallet = w
		}
	}

	if fromWallet == nil || toWallet == nil {
		return &FinancialToolResult{
			ToolTitle:  "Chuyển tiền nội bộ",
			ResultJSON: fmt.Sprintf(`{"error": "Không tìm thấy ví nguồn '%s' hoặc ví đích '%s' trong hệ thống."}`, fromName, toName),
		}, nil
	}

	var notePtr *string
	if notes != "" {
		notePtr = &notes
	}

	transferReq := dto.TransferMoneyRequest{
		FromWalletID: fromWallet.ID,
		ToWalletID:   toWallet.ID,
		Amount:       amount,
		Fee:          fee,
		Note:         notePtr,
	}

	res, err := s.walletService.TransferMoney(ctx, userID, transferReq)
	if err != nil {
		return &FinancialToolResult{
			ToolTitle:  "Chuyển tiền nội bộ thất bại",
			ResultJSON: fmt.Sprintf(`{"error": "%s"}`, err.Error()),
		}, nil
	}

	actionCard := &dto.ActionCard{
		ActionType:  "WALLET_TRANSFER",
		Title:       "Chuyển tiền thành công",
		Description: fmt.Sprintf("Đã chuyển %s ₫ từ ví %s sang ví %s", formatVND(amount), fromWallet.Name, toWallet.Name),
		Data:        res,
	}

	resultBytes, _ := json.Marshal(res)
	return &FinancialToolResult{
		ToolTitle:  "Đang thực hiện chuyển tiền...",
		ResultJSON: string(resultBytes),
		ActionCard: actionCard,
	}, nil
}

func (s *chatService) toolCreateDebt(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	title, _ := args["title"].(string)
	amount, _ := args["totalAmount"].(float64)
	typeStr, _ := args["type"].(string)
	notes, _ := args["notes"].(string)
	dueDateStr, _ := args["dueDate"].(string)

	if title == "" || amount <= 0 {
		return &FinancialToolResult{
			ToolTitle:  "Lỗi tham số khoản nợ",
			ResultJSON: `{"error": "Cần cung cấp tên khoản nợ và số tiền lớn hơn 0"}`,
		}, nil
	}

	debtType := model.DebtTypePayable
	if strings.ToUpper(typeStr) == "RECEIVABLE" {
		debtType = model.DebtTypeReceivable
	}

	var dueDate *time.Time
	if dueDateStr != "" {
		if t, err := time.Parse("2006-01-02", dueDateStr); err == nil {
			dueDate = &t
		}
	}

	req := dto.CreateDebtRequest{
		Type:        debtType,
		Title:       title,
		TotalAmount: amount,
		DueDate:     dueDate,
		Notes:       notes,
	}

	createdDebt, err := s.debtService.CreateDebt(ctx, userID, req)
	if err != nil {
		return nil, err
	}

	typeVi := "Khoản nợ phải trả"
	if debtType == model.DebtTypeReceivable {
		typeVi = "Khoản cho vay (cần thu)"
	}

	actionCard := &dto.ActionCard{
		ActionType:  "DEBT_CREATED",
		Title:       fmt.Sprintf("Đã ghi nhận %s", typeVi),
		Description: fmt.Sprintf("%s ₫ - %s", formatVND(amount), title),
		Data:        createdDebt,
	}

	resultBytes, _ := json.Marshal(createdDebt)
	return &FinancialToolResult{
		ToolTitle:  "Đang ghi nhận khoản nợ...",
		ResultJSON: string(resultBytes),
		ActionCard: actionCard,
	}, nil
}

func (s *chatService) toolRecordDebtRepayment(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	debtTitle, _ := args["debtTitle"].(string)
	amount, _ := args["amount"].(float64)
	notes, _ := args["notes"].(string)

	if debtTitle == "" || amount <= 0 {
		return &FinancialToolResult{
			ToolTitle:  "Lỗi tham số trả nợ",
			ResultJSON: `{"error": "Cần cung cấp tên khoản nợ và số tiền trả nợ"}`,
		}, nil
	}

	debts, err := s.debtService.GetDebts(ctx, userID, "", "")
	if err != nil || len(debts) == 0 {
		return &FinancialToolResult{
			ToolTitle:  "Ghi nhận trả nợ",
			ResultJSON: `{"error": "Bạn không có khoản nợ nào trong hệ thống."}`,
		}, nil
	}

	var matchedDebt *dto.DebtResponse
	for i := range debts {
		d := &debts[i]
		if strings.Contains(strings.ToLower(d.Title), strings.ToLower(debtTitle)) || strings.Contains(strings.ToLower(debtTitle), strings.ToLower(d.Title)) {
			matchedDebt = d
			break
		}
	}

	if matchedDebt == nil {
		return &FinancialToolResult{
			ToolTitle:  "Ghi nhận trả nợ",
			ResultJSON: fmt.Sprintf(`{"error": "Không tìm thấy khoản nợ khớp với '%s'"}`, debtTitle),
		}, nil
	}

	repayReq := dto.AddRepaymentRequest{
		Amount: amount,
		Notes:  notes,
	}

	updatedDebt, err := s.debtService.AddRepayment(ctx, userID, matchedDebt.ID, repayReq)
	if err != nil {
		return nil, err
	}

	actionCard := &dto.ActionCard{
		ActionType:  "DEBT_REPAID",
		Title:       "Đã ghi nhận thanh toán nợ",
		Description: fmt.Sprintf("Đã trả %s ₫ cho '%s' (Còn lại: %s ₫)", formatVND(amount), updatedDebt.Title, formatVND(updatedDebt.Remaining)),
		Data:        updatedDebt,
	}

	resultBytes, _ := json.Marshal(updatedDebt)
	return &FinancialToolResult{
		ToolTitle:  "Đang cập nhật thanh toán nợ...",
		ResultJSON: string(resultBytes),
		ActionCard: actionCard,
	}, nil
}

func (s *chatService) toolSetBudget(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	categoryName, _ := args["categoryName"].(string)
	amount, _ := args["amount"].(float64)
	periodStartStr, _ := args["periodStart"].(string)

	if categoryName == "" || amount <= 0 {
		return &FinancialToolResult{
			ToolTitle:  "Lỗi đặt hạn mức ngân sách",
			ResultJSON: `{"error": "Cần cung cấp tên danh mục và hạn mức ngân sách hợp lệ"}`,
		}, nil
	}

	categories, err := s.categoryRepo.List(ctx, userID, "", 100, 0)
	if err != nil || len(categories) == 0 {
		return nil, fmt.Errorf("không tìm thấy danh mục nào")
	}

	var targetCategory *model.Category
	for i := range categories {
		c := &categories[i]
		if strings.EqualFold(c.Name, categoryName) || strings.Contains(strings.ToLower(c.Name), strings.ToLower(categoryName)) {
			targetCategory = c
			break
		}
	}

	if targetCategory == nil {
		return &FinancialToolResult{
			ToolTitle:  "Đặt ngân sách",
			ResultJSON: fmt.Sprintf(`{"error": "Không tìm thấy danh mục '%s' trong hệ thống của bạn"}`, categoryName),
		}, nil
	}

	periodStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)
	if periodStartStr != "" {
		if t, err := time.Parse("2006-01-02", periodStartStr); err == nil {
			periodStart = t
		}
	}

	req := dto.CreateBudgetRequest{
		CategoryID:  targetCategory.ID,
		Amount:      amount,
		PeriodType:  "MONTHLY",
		PeriodStart: periodStart,
	}

	budgetRes, err := s.budgetService.CreateBudget(ctx, userID, req)
	if err != nil {
		return nil, err
	}

	actionCard := &dto.ActionCard{
		ActionType:  "BUDGET_SET",
		Title:       "Đã thiết lập ngân sách",
		Description: fmt.Sprintf("Hạn mức danh mục %s: %s ₫/tháng", targetCategory.Name, formatVND(amount)),
		Data:        budgetRes,
	}

	resultBytes, _ := json.Marshal(budgetRes)
	return &FinancialToolResult{
		ToolTitle:  "Đang thiết lập ngân sách...",
		ResultJSON: string(resultBytes),
		ActionCard: actionCard,
	}, nil
}

func (s *chatService) toolCompareFinancialPeriods(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	m1, _ := args["firstMonth"].(string)
	m2, _ := args["secondMonth"].(string)

	now := time.Now()
	if m1 == "" {
		m1 = now.AddDate(0, -1, 0).Format("2006-01")
	}
	if m2 == "" {
		m2 = now.Format("2006-01")
	}

	s1, e1 := resolveDateRange(map[string]interface{}{"month": m1})
	s2, e2 := resolveDateRange(map[string]interface{}{"month": m2})

	sum1, err1 := s.reportService.GetSummary(ctx, userID, s1, e1)
	sum2, err2 := s.reportService.GetSummary(ctx, userID, s2, e2)
	if err1 != nil || err2 != nil {
		return nil, fmt.Errorf("không thể lấy dữ liệu so sánh giữa 2 tháng")
	}

	diffExpense := sum2.TotalExpense - sum1.TotalExpense
	diffIncome := sum2.TotalIncome - sum1.TotalIncome

	resultMap := map[string]interface{}{
		"first_month": map[string]interface{}{
			"month":         m1,
			"total_income":  sum1.TotalIncome,
			"total_expense": sum1.TotalExpense,
			"net_savings":   sum1.TotalIncome - sum1.TotalExpense,
		},
		"second_month": map[string]interface{}{
			"month":         m2,
			"total_income":  sum2.TotalIncome,
			"total_expense": sum2.TotalExpense,
			"net_savings":   sum2.TotalIncome - sum2.TotalExpense,
		},
		"difference": map[string]interface{}{
			"expense_change": diffExpense,
			"income_change":  diffIncome,
		},
	}

	resultBytes, _ := json.Marshal(resultMap)
	return &FinancialToolResult{
		ToolTitle:  fmt.Sprintf("Đang so sánh tài chính tháng %s và %s...", m1, m2),
		ResultJSON: string(resultBytes),
	}, nil
}

func (s *chatService) toolGetFinancialTargets(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	now := time.Now()
	month := int(now.Month())
	year := now.Year()

	if mVal, ok := args["month"].(float64); ok && mVal > 0 {
		month = int(mVal)
	}
	if yVal, ok := args["year"].(float64); ok && yVal > 0 {
		year = int(yVal)
	}

	summary, err := s.targetService.GetSummary(ctx, userID, month, year)
	if err != nil {
		return nil, err
	}

	actionCard := &dto.ActionCard{
		ActionType:  "TARGET_STATUS",
		Title:       fmt.Sprintf("Mục tiêu tài chính Tháng %d/%d", month, year),
		Description: fmt.Sprintf("Chi tiêu: %s ₫ (Hạn mức: %s ₫) | Đầu tư: %s ₫", formatVND(summary.Expense.SpentAmount), formatVND(summary.Expense.TargetAmount), formatVND(summary.Investment.InvestedAmount)),
		Data:        summary,
	}

	resultBytes, _ := json.Marshal(summary)
	return &FinancialToolResult{
		ToolTitle:  "Đang kiểm tra mục tiêu tài chính...",
		ResultJSON: string(resultBytes),
		ActionCard: actionCard,
	}, nil
}

func (s *chatService) toolSetFinancialTarget(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	targetType, _ := args["targetType"].(string)
	amount, _ := args["targetAmount"].(float64)

	if targetType == "" || amount <= 0 {
		return &FinancialToolResult{
			ToolTitle:  "Lỗi thiết lập mục tiêu",
			ResultJSON: `{"error": "Cần cung cấp loại mục tiêu (EXPENSE/INVESTMENT) và số tiền lớn hơn 0"}`,
		}, nil
	}

	// Normalize target type
	if strings.ToUpper(targetType) == "SPENDING_LIMIT" || strings.ToUpper(targetType) == "SPENDING" {
		targetType = "EXPENSE"
	} else if strings.ToUpper(targetType) == "SAVINGS" {
		targetType = "INVESTMENT"
	}

	now := time.Now()
	month := int(now.Month())
	year := now.Year()

	if mVal, ok := args["month"].(float64); ok && mVal > 0 {
		month = int(mVal)
	}
	if yVal, ok := args["year"].(float64); ok && yVal > 0 {
		year = int(yVal)
	}

	req := &dto.UpsertTargetRequest{
		TargetType:   targetType,
		TargetAmount: amount,
		Month:        month,
		Year:         year,
	}

	if err := s.targetService.UpsertTarget(ctx, userID, req); err != nil {
		return nil, err
	}

	typeVi := "Hạn mức chi tiêu tối đa"
	if targetType == "INVESTMENT" {
		typeVi = "Mục tiêu đầu tư / tiết kiệm"
	}

	actionCard := &dto.ActionCard{
		ActionType:  "TARGET_SET",
		Title:       "Đã lưu mục tiêu tài chính",
		Description: fmt.Sprintf("%s Tháng %d/%d: %s ₫", typeVi, month, year, formatVND(amount)),
		Data: map[string]interface{}{
			"targetType":   targetType,
			"targetAmount": amount,
			"month":        month,
			"year":         year,
		},
	}

	resultBytes, _ := json.Marshal(map[string]interface{}{
		"success":      true,
		"targetType":   targetType,
		"targetAmount": amount,
		"month":        month,
		"year":         year,
		"message":      "Mục tiêu tài chính đã được thiết lập thành công.",
	})

	return &FinancialToolResult{
		ToolTitle:  "Đang lưu mục tiêu tài chính...",
		ResultJSON: string(resultBytes),
		ActionCard: actionCard,
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
