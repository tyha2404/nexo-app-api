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

// getReadOnlyToolDefinitions returns schemas for generic read-only tools that
// cover every domain of the project without any write capability.
func getReadOnlyToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "search_transactions",
				Description: "Tìm kiếm và lọc nâng cao các giao dịch thu/chi/đầu tư: theo tên danh mục, tên ví tiền, khoảng thời gian (ngày bắt đầu, ngày kết thúc, tháng), khoảng số tiền (tối thiểu, tối đa), từ khóa mô tả và sắp xếp. Hãy gọi tool này khi người dùng yêu cầu: 'tìm các khoản chi trên 500k', 'tìm giao dịch có chữ Grab', 'lọc chi tiêu ăn uống từ ngày A đến ngày B'.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"INCOME", "EXPENSE", "INVESTMENT", "ALL"},
							"description": "Loại giao dịch cần lọc ('INCOME', 'EXPENSE', 'INVESTMENT', 'ALL'). Mặc định ALL.",
						},
						"categoryName": map[string]interface{}{
							"type":        "string",
							"description": "Tên danh mục cần lọc (ví dụ: 'Ăn uống', 'Di chuyển', 'Mua sắm').",
						},
						"walletName": map[string]interface{}{
							"type":        "string",
							"description": "Tên ví/tài khoản cần lọc (ví dụ: 'Techcombank', 'Tiền mặt', 'MoMo').",
						},
						"month": map[string]interface{}{
							"type":        "string",
							"description": "Tháng cần lọc theo định dạng YYYY-MM (ví dụ: '2026-08').",
						},
						"startDate": map[string]interface{}{
							"type":        "string",
							"description": "Ngày bắt đầu (YYYY-MM-DD).",
						},
						"endDate": map[string]interface{}{
							"type":        "string",
							"description": "Ngày kết thúc (YYYY-MM-DD).",
						},
						"minAmount": map[string]interface{}{
							"type":        "number",
							"description": "Số tiền tối thiểu (VND).",
						},
						"maxAmount": map[string]interface{}{
							"type":        "number",
							"description": "Số tiền tối đa (VND).",
						},
						"keyword": map[string]interface{}{
							"type":        "string",
							"description": "Từ khóa tìm trong ghi chú/mô tả giao dịch (ví dụ: 'phở', 'grab', 'tiền điện').",
						},
						"sortBy": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"date_desc", "date_asc", "amount_desc", "amount_asc"},
							"description": "Cách sắp xếp: 'date_desc' (mới nhất trước), 'date_asc' (cũ nhất trước), 'amount_desc' (tiền nhiều nhất), 'amount_asc' (tiền ít nhất). Mặc định date_desc.",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Số kết quả trả về (mặc định 10, tối đa 50).",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "list_categories",
				Description: "Liệt kê toàn bộ danh mục thu/chi hiện có của người dùng, kèm số tiền đã chi tiêu trong tháng hiện tại và ngân sách của từng danh mục. Hãy gọi tool này khi người dùng hỏi: 'tôi có những danh mục chi tiêu nào?', 'liệt kê các nhóm danh mục'.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"INCOME", "EXPENSE", "ALL"},
							"description": "Lọc theo loại danh mục: 'INCOME', 'EXPENSE', 'ALL'. Mặc định ALL.",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_monthly_trend",
				Description: "Phân tích xu hướng tài chính (tổng thu, tổng chi, tiết kiệm và mức chi trung bình mỗi tháng) qua N tháng liên tiếp gần đây. Hãy gọi tool này khi người dùng hỏi: 'xu hướng chi tiêu mấy tháng gần đây thế nào?', 'trung bình mỗi tháng tôi tiêu bao nhiêu?'.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"months": map[string]interface{}{
							"type":        "integer",
							"description": "Số tháng liên tiếp cần phân tích lùi từ tháng hiện tại (mặc định 6, tối đa 12).",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_wallet_detail",
				Description: "Xem chi tiết một ví/tài khoản cụ thể: số dư hiện tại và lịch sử các giao dịch gần nhất trên ví đó. Hãy gọi tool này khi người dùng hỏi: 'ví Techcombank còn bao nhiêu và gần đây có giao dịch gì?', 'chi tiết ví MoMo'.",
				Parameters: map[string]interface{}{
					"type":     "object",
					"required": []string{"walletName"},
					"properties": map[string]interface{}{
						"walletName": map[string]interface{}{
							"type":        "string",
							"description": "Tên ví cần tra cứu chi tiết (ví dụ: 'Techcombank', 'MoMo', 'Tiền mặt').",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Số giao dịch gần nhất trên ví cần lấy (mặc định 10, tối đa 20).",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_debt_detail",
				Description: "Xem chi tiết một khoản nợ hoặc cho vay cụ thể: tổng số tiền, số tiền đã trả, số tiền còn lại, ngày đến hạn và toàn bộ lịch sử thanh toán. Hãy gọi tool này khi người dùng hỏi: 'chi tiết khoản nợ của Nam', 'khoản vay anh Tuấn đã trả được bao nhiêu rồi?'.",
				Parameters: map[string]interface{}{
					"type":     "object",
					"required": []string{"debtTitle"},
					"properties": map[string]interface{}{
						"debtTitle": map[string]interface{}{
							"type":        "string",
							"description": "Tên khoản nợ hoặc tên người liên quan (ví dụ: 'Cho Nam mượn tiền', 'Vay anh Tuấn').",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_investment_summary",
				Description: "Xem tổng quan danh mục đầu tư tài chính của người dùng: tổng giá trị tài sản đang nắm giữ, số vị thế holding, lãi/lỗ đã hiện thực (realized PnL) và các giao dịch đầu tư mới nhất. Hãy gọi tool này khi người dùng hỏi: 'tình hình đầu tư của tôi', 'danh mục đầu tư lãi lỗ ra sao?'.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "search_financial_knowledge",
				Description: "Tra cứu kiến thức và nguyên tắc tài chính trong kho tri thức nội bộ (ví dụ: quy tắc 50/30/20, xây dựng quỹ dự phòng khẩn cấp, phương pháp quản lý nợ snowball/avalanche, chiến lược tiết kiệm). Hãy gọi tool này khi người dùng xin lời khuyên tài chính chung.",
				Parameters: map[string]interface{}{
					"type":     "object",
					"required": []string{"query"},
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Chủ đề hoặc câu hỏi tài chính cần tra cứu (ví dụ: 'quy tắc 50 30 20', 'quỹ khẩn cấp bao nhiêu là đủ', 'cách trả nợ nhanh').",
						},
					},
				},
			},
		},
	}
}

// executeReadOnlyFinancialTool dispatches read-only tool calls. Returns
// (nil, false) when the tool name is not handled so the caller can fall back.
func (s *chatService) executeReadOnlyFinancialTool(ctx context.Context, userID uuid.UUID, name string, args map[string]interface{}) (*FinancialToolResult, bool) {
	switch name {
	case "search_transactions":
		return runTool(s.toolSearchTransactions(ctx, userID, args)), true
	case "list_categories":
		return runTool(s.toolListCategories(ctx, userID, args)), true
	case "get_monthly_trend":
		return runTool(s.toolGetMonthlyTrend(ctx, userID, args)), true
	case "get_wallet_detail":
		return runTool(s.toolGetWalletDetail(ctx, userID, args)), true
	case "get_debt_detail":
		return runTool(s.toolGetDebtDetail(ctx, userID, args)), true
	case "get_investment_summary":
		return runTool(s.toolGetInvestmentSummary(ctx, userID, args)), true
	case "search_financial_knowledge":
		return runTool(s.toolSearchFinancialKnowledge(ctx, userID, args)), true
	default:
		return nil, false
	}
}

// runTool converts a tool handler result/error into a FinancialToolResult.
func runTool(res *FinancialToolResult, err error) *FinancialToolResult {
	if err != nil {
		return &FinancialToolResult{
			ToolTitle:  "Lỗi thực thi công cụ",
			ResultJSON: fmt.Sprintf(`{"error": "%s"}`, err.Error()),
		}
	}
	return res
}

func (s *chatService) toolSearchTransactions(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	startDate, endDate := resolveDateRange(args)

	filters := map[string]interface{}{
		"startDate": startDate,
		"endDate":   endDate,
	}
	if t, ok := args["type"].(string); ok && t != "" {
		filters["type"] = t
	}
	if kw, ok := args["keyword"].(string); ok && kw != "" {
		filters["keyword"] = kw
	}
	if sort, ok := args["sortBy"].(string); ok && sort != "" {
		filters["sortBy"] = sort
	}
	if minA, ok := args["minAmount"].(float64); ok {
		filters["minAmount"] = minA
	}
	if maxA, ok := args["maxAmount"].(float64); ok {
		filters["maxAmount"] = maxA
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	if limit > 50 {
		limit = 50
	}

	// Resolve category name -> category ID
	if catName, ok := args["categoryName"].(string); ok && catName != "" {
		categories, err := s.categoryRepo.List(ctx, userID, "", 200, 0)
		if err != nil {
			return nil, err
		}
		var matched *model.Category
		for i := range categories {
			if strings.EqualFold(categories[i].Name, catName) || strings.Contains(strings.ToLower(categories[i].Name), strings.ToLower(catName)) {
				matched = &categories[i]
				break
			}
		}
		if matched == nil {
			resMap := map[string]interface{}{
				"error":   fmt.Sprintf("Không tìm thấy danh mục '%s'", catName),
				"hints":   categoryNames(categories),
				"message": "Hãy thử lại với tên danh mục chính xác hơn.",
			}
			resBytes, _ := json.Marshal(resMap)
			return &FinancialToolResult{ToolTitle: "Đang tìm kiếm giao dịch...", ResultJSON: string(resBytes)}, nil
		}
		filters["categoryId"] = matched.ID.String()
	}

	// Resolve wallet name -> wallet ID
	walletNamesByID := map[uuid.UUID]string{}
	if walletName, ok := args["walletName"].(string); ok && walletName != "" {
		wallets, err := s.walletRepo.ListByUserID(ctx, userID)
		if err != nil {
			return nil, err
		}
		var matched *model.Wallet
		for i := range wallets {
			walletNamesByID[wallets[i].ID] = wallets[i].Name
			if strings.EqualFold(wallets[i].Name, walletName) || strings.Contains(strings.ToLower(wallets[i].Name), strings.ToLower(walletName)) {
				matched = &wallets[i]
			}
		}
		if matched == nil {
			names := make([]string, 0, len(wallets))
			for _, w := range wallets {
				names = append(names, w.Name)
			}
			resMap := map[string]interface{}{
				"error":   fmt.Sprintf("Không tìm thấy ví '%s'", walletName),
				"hints":   names,
				"message": "Hãy thử lại với tên ví chính xác hơn.",
			}
			resBytes, _ := json.Marshal(resMap)
			return &FinancialToolResult{ToolTitle: "Đang tìm kiếm giao dịch...", ResultJSON: string(resBytes)}, nil
		}
		filters["walletId"] = matched.ID.String()
	} else if wallets, err := s.walletRepo.ListByUserID(ctx, userID); err == nil {
		for _, w := range wallets {
			walletNamesByID[w.ID] = w.Name
		}
	}

	txns, total, err := s.transactionRepo.SearchByUserID(ctx, userID, limit, 0, filters)
	if err != nil {
		return nil, err
	}

	type SearchTxnItem struct {
		Date        string  `json:"date"`
		Type        string  `json:"type"`
		Amount      float64 `json:"amount"`
		Category    string  `json:"category"`
		Wallet      string  `json:"wallet"`
		Description string  `json:"description"`
	}

	items := make([]SearchTxnItem, 0, len(txns))
	sumAmount := 0.0
	for _, tx := range txns {
		desc := ""
		if tx.Description != nil {
			desc = *tx.Description
		}
		catName := ""
		if tx.Category != nil {
			catName = tx.Category.Name
		}
		walletName := ""
		if tx.WalletID != nil {
			walletName = walletNamesByID[*tx.WalletID]
		}
		items = append(items, SearchTxnItem{
			Date:        tx.TransactionDate.Format("2006-01-02"),
			Type:        string(tx.Type),
			Amount:      tx.Amount,
			Category:    catName,
			Wallet:      walletName,
			Description: desc,
		})
		sumAmount += tx.Amount
	}

	resultMap := map[string]interface{}{
		"start_date":   startDate,
		"end_date":     endDate,
		"total_count":  total,
		"return_count": len(items),
		"sum_amount":   sumAmount,
		"transactions": items,
	}
	resultBytes, _ := json.Marshal(resultMap)

	return &FinancialToolResult{
		ToolTitle:  "Đang tìm kiếm giao dịch...",
		ResultJSON: string(resultBytes),
	}, nil
}

func (s *chatService) toolListCategories(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	typeFilter, _ := args["type"].(string)

	categories, err := s.categoryRepo.List(ctx, userID, "", 200, 0)
	if err != nil {
		return nil, err
	}

	// Spending this month per category (EXPENSE only)
	spentByCategory := map[uuid.UUID]float64{}
	now := time.Now()
	breakdown, err := s.reportService.GetCategoryBreakdown(
		ctx, userID,
		time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).Format("2006-01-02"),
		now.Format("2006-01-02"),
	)
	if err == nil {
		for _, item := range breakdown.Items {
			if item.CategoryID != uuid.Nil {
				spentByCategory[item.CategoryID] = item.TotalAmount
			}
		}
	}

	// Active budgets per category
	budgetLimitByCategory := map[uuid.UUID]float64{}
	budgets, _, err := s.budgetService.ListBudgets(ctx, userID, 1, 100)
	if err == nil {
		for _, b := range budgets {
			budgetLimitByCategory[b.CategoryID] = b.Amount
		}
	}

	type CategoryItem struct {
		Name            string   `json:"name"`
		Type            string   `json:"type"`
		SpentThisMonth  float64  `json:"spentThisMonth,omitempty"`
		BudgetLimit     float64  `json:"budgetLimit,omitempty"`
	}

	items := make([]CategoryItem, 0, len(categories))
	for _, c := range categories {
		if typeFilter != "" && typeFilter != "ALL" && string(c.Type) != typeFilter {
			continue
		}
		item := CategoryItem{Name: c.Name, Type: string(c.Type)}
		if c.Type == model.CategoryTypeExpense {
			item.SpentThisMonth = spentByCategory[c.ID]
			item.BudgetLimit = budgetLimitByCategory[c.ID]
		}
		items = append(items, item)
	}

	resultMap := map[string]interface{}{
		"month":      now.Format("2006-01"),
		"categories": items,
		"count":      len(items),
	}
	resultBytes, _ := json.Marshal(resultMap)

	return &FinancialToolResult{
		ToolTitle:  "Đang liệt kê danh mục...",
		ResultJSON: string(resultBytes),
	}, nil
}

func (s *chatService) toolGetMonthlyTrend(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	monthsBack := 6
	if m, ok := args["months"].(float64); ok && m >= 2 {
		monthsBack = int(m)
	}
	if monthsBack > 12 {
		monthsBack = 12
	}

	now := time.Now()
	type MonthPoint struct {
		Month       string  `json:"month"`
		Income      float64 `json:"income"`
		Expense     float64 `json:"expense"`
		Investment  float64 `json:"investment"`
		NetSavings  float64 `json:"netSavings"`
		SavingsRate float64 `json:"savingsRatePercent"`
	}

	points := make([]MonthPoint, 0, monthsBack)
	totalIncome, totalExpense := 0.0, 0.0
	for i := monthsBack - 1; i >= 0; i-- {
		t := now.AddDate(0, -i, 0)
		start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 1, -1)

		summary, err := s.reportService.GetSummary(ctx, userID, start.Format("2006-01-02"), end.Format("2006-01-02"))
		if err != nil {
			return nil, err
		}

		net := summary.TotalIncome - summary.TotalExpense
		rate := 0.0
		if summary.TotalIncome > 0 {
			rate = (net / summary.TotalIncome) * 100.0
		}
		points = append(points, MonthPoint{
			Month:       start.Format("2006-01"),
			Income:      summary.TotalIncome,
			Expense:     summary.TotalExpense,
			Investment:  summary.TotalInvestment,
			NetSavings:  net,
			SavingsRate: math.Round(rate*10) / 10,
		})
		totalIncome += summary.TotalIncome
		totalExpense += summary.TotalExpense
	}

	resultMap := map[string]interface{}{
		"months_count":           monthsBack,
		"monthly_points":         points,
		"avg_income_per_month":   math.Round(totalIncome/float64(monthsBack)*100) / 100,
		"avg_expense_per_month":  math.Round(totalExpense/float64(monthsBack)*100) / 100,
		"avg_savings_per_month":  math.Round((totalIncome-totalExpense)/float64(monthsBack)*100) / 100,
	}
	resultBytes, _ := json.Marshal(resultMap)

	return &FinancialToolResult{
		ToolTitle:  fmt.Sprintf("Đang phân tích xu hướng %d tháng...", monthsBack),
		ResultJSON: string(resultBytes),
	}, nil
}

func (s *chatService) toolGetWalletDetail(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	walletName, _ := args["walletName"].(string)
	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	if limit > 20 {
		limit = 20
	}

	walletSummary, err := s.walletService.GetWallets(ctx, userID)
	if err != nil {
		return nil, err
	}

	var matched *dto.WalletResponse
	for i := range walletSummary.Wallets {
		w := &walletSummary.Wallets[i]
		if strings.EqualFold(w.Name, walletName) || strings.Contains(strings.ToLower(w.Name), strings.ToLower(walletName)) || strings.Contains(strings.ToLower(walletName), strings.ToLower(w.Name)) {
			matched = w
			break
		}
	}

	if matched == nil {
		names := make([]string, 0, len(walletSummary.Wallets))
		for _, w := range walletSummary.Wallets {
			names = append(names, w.Name)
		}
		resMap := map[string]interface{}{
			"error":   fmt.Sprintf("Không tìm thấy ví '%s'", walletName),
			"hints":   names,
			"message": "Hãy thử lại với tên ví chính xác hơn.",
		}
		resBytes, _ := json.Marshal(resMap)
		return &FinancialToolResult{ToolTitle: "Đang xem chi tiết ví...", ResultJSON: string(resBytes)}, nil
	}

	txns, total, err := s.transactionRepo.SearchByUserID(ctx, userID, limit, 0, map[string]interface{}{
		"walletId":  matched.ID.String(),
		"startDate": "1970-01-01",
		"endDate":   time.Now().Format("2006-01-02"),
	})
	if err != nil {
		return nil, err
	}

	type WalletTxn struct {
		Date        string  `json:"date"`
		Type        string  `json:"type"`
		Amount      float64 `json:"amount"`
		Category    string  `json:"category"`
		Description string  `json:"description"`
	}
	items := make([]WalletTxn, 0, len(txns))
	for _, tx := range txns {
		desc := ""
		if tx.Description != nil {
			desc = *tx.Description
		}
		catName := ""
		if tx.Category != nil {
			catName = tx.Category.Name
		}
		items = append(items, WalletTxn{
			Date:        tx.TransactionDate.Format("2006-01-02"),
			Type:        string(tx.Type),
			Amount:      tx.Amount,
			Category:    catName,
			Description: desc,
		})
	}

	resultMap := map[string]interface{}{
		"wallet":              matched,
		"recent_transactions": items,
		"transaction_count":   total,
	}
	resultBytes, _ := json.Marshal(resultMap)

	return &FinancialToolResult{
		ToolTitle:  fmt.Sprintf("Đang xem chi tiết ví %s...", matched.Name),
		ResultJSON: string(resultBytes),
	}, nil
}

func (s *chatService) toolGetDebtDetail(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	debtTitle, _ := args["debtTitle"].(string)
	if debtTitle == "" {
		return &FinancialToolResult{
			ToolTitle:  "Tra cứu chi tiết khoản nợ",
			ResultJSON: `{"error": "Cần cung cấp tên khoản nợ hoặc tên người vay"}`,
		}, nil
	}

	debts, err := s.debtService.GetDebts(ctx, userID, "", "")
	if err != nil {
		return nil, err
	}

	var matched *dto.DebtResponse
	for i := range debts {
		d := &debts[i]
		if strings.Contains(strings.ToLower(d.Title), strings.ToLower(debtTitle)) || strings.Contains(strings.ToLower(debtTitle), strings.ToLower(d.Title)) {
			matched = d
			break
		}
	}

	if matched == nil {
		titles := make([]string, 0, len(debts))
		for _, d := range debts {
			titles = append(titles, d.Title)
		}
		resMap := map[string]interface{}{
			"error":   fmt.Sprintf("Không tìm thấy khoản nợ khớp với '%s'", debtTitle),
			"hints":   titles,
			"message": "Bạn có thể dùng get_debt_summary để xem toàn bộ danh sách.",
		}
		resBytes, _ := json.Marshal(resMap)
		return &FinancialToolResult{ToolTitle: "Đang tra cứu khoản nợ...", ResultJSON: string(resBytes)}, nil
	}

	resultBytes, _ := json.Marshal(matched)

	return &FinancialToolResult{
		ToolTitle:  fmt.Sprintf("Đang tra cứu '%s'...", matched.Title),
		ResultJSON: string(resultBytes),
	}, nil
}

func (s *chatService) toolGetInvestmentSummary(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	filters := map[string]interface{}{"type": string(model.TransactionTypeInvestment)}
	investments, total, summary, err := s.transactionService.ListTransactions(ctx, userID, 1, 15, filters)
	if err != nil {
		return nil, err
	}

	recent := make([]map[string]interface{}, 0, len(investments))
	for _, inv := range investments {
		desc := ""
		if inv.Description != nil {
			desc = *inv.Description
		}
		status := ""
		if inv.Status != nil {
			status = *inv.Status
		}
		recent = append(recent, map[string]interface{}{
			"id":          inv.ID,
			"date":        inv.TransactionDate,
			"amount":      inv.Amount,
			"status":      status,
			"realizedPnl": inv.RealizedPnL,
			"category":    inv.CategoryName,
			"description": desc,
		})
	}

	resultMap := map[string]interface{}{
		"holding_amount":   summary.HoldingAmount,
		"holding_count":    summary.HoldingCount,
		"realized_pnl":     summary.RealizedPnL,
		"total_invested":   summary.SumAmount,
		"investment_count": total,
		"recent":           recent,
	}
	resultBytes, _ := json.Marshal(resultMap)

	actionCard := &dto.ActionCard{
		ActionType:  "FINANCIAL_SUMMARY",
		Title:       "Danh mục Đầu tư",
		Description: fmt.Sprintf("Đang giữ: %s ₫ (%d vị trí) | Lãi/Lỗ đã chốt: %s ₫", formatVND(summary.HoldingAmount), summary.HoldingCount, formatVND(summary.RealizedPnL)),
		Data:        resultMap,
	}

	return &FinancialToolResult{
		ToolTitle:  "Đang tổng hợp danh mục đầu tư...",
		ResultJSON: string(resultBytes),
		ActionCard: actionCard,
	}, nil
}

func (s *chatService) toolSearchFinancialKnowledge(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return &FinancialToolResult{
			ToolTitle:  "Tra cứu kiến thức tài chính",
			ResultJSON: `{"error": "Cần cung cấp câu hỏi hoặc chủ đề cần tra cứu"}`,
		}, nil
	}

	results, err := s.ragService.SearchKnowledge(ctx, query, 3)
	if err != nil {
		s.logger.Warn("knowledge search failed", zap.String("query", query), zap.Error(err))
		results = nil
	}

	fallbackCount := 0
	for _, r := range results {
		if r.Fallback {
			fallbackCount++
		}
	}

	payload := map[string]interface{}{
		"query":          query,
		"results":        results,
		"fallback_count": fallbackCount,
	}
	if len(results) == 0 {
		payload["message"] = "Không tìm thấy kiến thức phù hợp, hãy trả lời người dùng dựa trên hiểu biết chung."
	} else if fallbackCount > 0 {
		payload["message"] = "Một số kết quả không khớp chính xác với câu hỏi (fallback), hãy chỉ dùng chúng nếu thực sự liên quan."
	}
	resultBytes, _ := json.Marshal(payload)

	return &FinancialToolResult{
		ToolTitle:  "Đang tra cứu kiến thức tài chính...",
		ResultJSON: string(resultBytes),
		ActionCard: &dto.ActionCard{
			ActionType:  "KNOWLEDGE_SOURCE",
			Title:       "Nguồn tri thức nội bộ",
			Description: fmt.Sprintf("Tìm thấy %d tài liệu liên quan tới \"%s\"", len(results), query),
			Data:        payload,
		},
	}, nil
}

func categoryNames(categories []model.Category) []string {
	names := make([]string, 0, len(categories))
	for _, c := range categories {
		names = append(names, c.Name)
	}
	return names
}
