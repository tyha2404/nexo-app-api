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
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

// financialToolSpecs are the AI tools of the financial domain (reads and
// write proposals), in the order they are offered to the model.
func financialToolSpecs() []chatTool {
	return []chatTool{
		{
			run: (*chatService).toolGetFinancialOverview,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "get_financial_overview",
					Description: "Lấy báo cáo tổng quan tình hình tài chính của người dùng (tổng số dư ví hiện tại, tổng thu nhập, tổng chi tiêu, tiền tiết kiệm ròng và tỷ lệ tiết kiệm). Hãy gọi tool này khi người dùng hỏi: 'tháng này tài chính thế nào?', 'tôi có bao nhiêu tiền?', 'tổng quan thu chi', 'tình hình tiết kiệm'.",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"month": map[string]interface{}{
								"type":        "string",
								"description": "Tháng cần tra cứu theo định dạng YYYY-MM (ví dụ: '2026-08'). Mặc định là tháng hiện tại.",
							},
							"startDate": map[string]interface{}{
								"type":        "string",
								"description": "Ngày bắt đầu theo định dạng YYYY-MM-DD nếu muốn lọc theo khoảng ngày cụ thể.",
							},
							"endDate": map[string]interface{}{
								"type":        "string",
								"description": "Ngày kết thúc theo định dạng YYYY-MM-DD.",
							},
						},
					},
				},
			},
		},
		{
			run: (*chatService).toolListRecentTransactions,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "list_recent_transactions",
					Description: "Lấy danh sách các giao dịch thu/chi mới nhất của người dùng. Hãy gọi tool này khi người dùng hỏi: 'các giao dịch gần đây', 'vừa chi tiêu những gì gần đây', 'lịch sử thu chi mới nhất'.",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"limit": map[string]interface{}{
								"type":        "integer",
								"description": "Số lượng giao dịch cần lấy (mặc định 5, tối đa 20).",
							},
							"type": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"INCOME", "EXPENSE", "INVESTMENT", "ALL"},
								"description": "Loại giao dịch cần lọc: 'INCOME' (thu nhập), 'EXPENSE' (chi tiêu), 'INVESTMENT' (đầu tư), 'ALL' (tất cả). Mặc định là ALL.",
							},
							"startDate": map[string]interface{}{
								"type":        "string",
								"description": "Ngày bắt đầu (YYYY-MM-DD).",
							},
							"endDate": map[string]interface{}{
								"type":        "string",
								"description": "Ngày kết thúc (YYYY-MM-DD).",
							},
						},
					},
				},
			},
		},
		{
			preview: (*chatService).previewCreateTransaction,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "create_transaction",
					Description: "Tạo và ghi nhận một giao dịch thu nhập hoặc chi tiêu mới vào hệ thống Nexo. BẮT BUỘC gọi tool này khi người dùng nói: 'vừa ăn phở 50k', 'mua cafe 35k tiền mặt', 'vừa nhận lương 20 triệu', 'thêm khoản chi 100k'. Hệ thống sẽ tự động gán hoặc tạo danh mục phù hợp.",
					Parameters: map[string]interface{}{
						"type":     "object",
						"required": []string{"amount", "type"},
						"properties": map[string]interface{}{
							"amount": map[string]interface{}{
								"type":        "number",
								"description": "Số tiền giao dịch thực tế bằng số nguyên (VND). Ví dụ: 50000, 35000, 20000000.",
							},
							"type": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"INCOME", "EXPENSE"},
								"description": "Bắt buộc chọn: 'EXPENSE' nếu là chi tiêu/mua sắm/thanh toán, 'INCOME' nếu là thu nhập/nhận tiền/lương/thưởng.",
							},
							"categoryName": map[string]interface{}{
								"type":        "string",
								"description": "Tên danh mục chi tiêu/thu nhập (ví dụ: 'Ăn uống', 'Cafe', 'Mua sắm', 'Di chuyển', 'Lương', 'Thưởng').",
							},
							"walletName": map[string]interface{}{
								"type":        "string",
								"description": "Tên ví/tài khoản sử dụng (ví dụ: 'Tiền mặt', 'Techcombank', 'MoMo', 'Vietcombank'). Nếu người dùng không nhắc đến ví, hãy để trống: hệ thống sẽ trả về danh sách ví để bạn hỏi lại người dùng (trừ khi chỉ có một ví).",
							},
							"description": map[string]interface{}{
								"type":        "string",
								"description": "Mô tả/ghi chú chi tiết cho giao dịch (ví dụ: 'Ăn phở bò sáng', 'Cafe Highland', 'Lương tháng 8').",
							},
							"transactionDate": map[string]interface{}{
								"type":        "string",
								"description": "Ngày thực hiện giao dịch (định dạng YYYY-MM-DD). Mặc định là ngày hôm nay nếu người dùng không nói rõ ngày khác.",
							},
						},
					},
				},
			},
		},
		{
			preview: (*chatService).previewCreateCategory,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "create_category",
					Description: "Tạo một danh mục thu/chi/đầu tư mới trong hệ thống. Hãy gọi tool này khi người dùng chủ động yêu cầu: 'tạo danh mục Học tập', 'thêm category Tiền điện loại chi tiêu', 'tạo danh mục Freelance loại thu nhập'.",
					Parameters: map[string]interface{}{
						"type":     "object",
						"required": []string{"name"},
						"properties": map[string]interface{}{
							"name": map[string]interface{}{
								"type":        "string",
								"description": "Tên danh mục mới cần tạo (ví dụ: 'Học tập', 'Thú cưng', 'Tiền điện nước', 'Freelance').",
							},
							"type": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"EXPENSE", "INCOME", "INVESTMENT"},
								"description": "Loại danh mục: 'EXPENSE' (chi tiêu - mặc định), 'INCOME' (thu nhập), 'INVESTMENT' (đầu tư).",
							},
							"description": map[string]interface{}{
								"type":        "string",
								"description": "Mô tả ngắn gọn về danh mục mới (tùy chọn).",
							},
						},
					},
				},
			},
		},
		{
			run: (*chatService).toolGetBudgetStatus,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "get_budget_status",
					Description: "Kiểm tra tiến độ thực hiện ngân sách trong tháng: hạn mức đã đặt, số tiền đã chi, số tiền còn lại và cảnh báo nếu vượt ngân sách. Hãy gọi tool này khi người dùng hỏi: 'ngân sách tháng này thế nào?', 'tôi còn bao nhiêu tiền trong ngân sách ăn uống?', 'đã chi vượt ngân sách chưa?'.",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"month": map[string]interface{}{
								"type":        "string",
								"description": "Tháng cần kiểm tra (định dạng YYYY-MM, ví dụ: '2026-08'). Mặc định là tháng hiện tại.",
							},
						},
					},
				},
			},
		},
		{
			run: (*chatService).toolGetDebtSummary,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "get_debt_summary",
					Description: "Lấy báo cáo tổng quan về các khoản nợ phải trả (mình nợ ai) và các khoản cho vay (ai nợ mình). Hãy gọi tool này khi người dùng hỏi: 'tôi đang nợ ai?', 'ai đang mượn tiền tôi?', 'tổng nợ hiện tại bao nhiêu?'.",
					Parameters: map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
			},
		},
		{
			run: (*chatService).toolListWallets,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "list_wallets",
					Description: "Xem danh sách toàn bộ các ví/tài khoản và số dư thực tế của từng ví (Tiền mặt, Ngân hàng, Thẻ tín dụng - kèm hạn mức, dư nợ và ngày sao kê/hạn thanh toán, Ví điện tử). Hãy gọi tool này khi người dùng hỏi: 'tôi có những ví nào?', 'số dư tài khoản của tôi?', 'kiểm tra các ví', 'hạn mức thẻ tín dụng còn bao nhiêu?'.",
					Parameters: map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
			},
		},
		{
			run: (*chatService).toolGetSpendingByCategory,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "get_spending_by_category",
					Description: "Phân tích cơ cấu và tỷ trọng chi tiêu theo từng danh mục (Breakdown). Hãy gọi tool này khi người dùng hỏi: 'tháng này tiêu vào việc gì nhiều nhất?', 'cơ cấu chi tiêu', 'tôi đã tiêu bao nhiêu cho ăn uống?'.",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"month": map[string]interface{}{
								"type":        "string",
								"description": "Tháng cần phân tích (định dạng YYYY-MM, ví dụ: '2026-08').",
							},
							"startDate": map[string]interface{}{
								"type":        "string",
								"description": "Ngày bắt đầu (YYYY-MM-DD).",
							},
							"endDate": map[string]interface{}{
								"type":        "string",
								"description": "Ngày kết thúc (YYYY-MM-DD).",
							},
						},
					},
				},
			},
		},
		{
			preview: (*chatService).previewTransfer,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "transfer_between_wallets",
					Description: "Thực hiện chuyển tiền nội bộ giữa 2 ví/tài khoản cá nhân. Hãy gọi tool này khi người dùng nói: 'chuyển 500k từ Techcombank sang MoMo', 'rút 1 triệu từ tài khoản về tiền mặt'.",
					Parameters: map[string]interface{}{
						"type":     "object",
						"required": []string{"amount", "fromWalletName", "toWalletName"},
						"properties": map[string]interface{}{
							"amount": map[string]interface{}{
								"type":        "number",
								"description": "Số tiền cần chuyển (VND). Ví dụ: 500000, 1000000.",
							},
							"fromWalletName": map[string]interface{}{
								"type":        "string",
								"description": "Tên ví nguồn chuyển đi (ví dụ: 'Techcombank', 'VPBank', 'Tiền mặt').",
							},
							"toWalletName": map[string]interface{}{
								"type":        "string",
								"description": "Tên ví đích nhận tiền (ví dụ: 'MoMo', 'Tiền mặt', 'ZaloPay').",
							},
							"fee": map[string]interface{}{
								"type":        "number",
								"description": "Phí chuyển khoản nếu có (VND, mặc định 0).",
							},
							"notes": map[string]interface{}{
								"type":        "string",
								"description": "Ghi chú cho việc chuyển khoản.",
							},
						},
					},
				},
			},
		},
		{
			preview: (*chatService).previewCreateDebt,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "create_debt",
					Description: "Tạo một khoản nợ mới (nợ phải trả PAYABLE hoặc cho vay cần thu hồi RECEIVABLE). Hãy gọi tool này khi người dùng nói: 'cho Nam vay 2 triệu', 'mượn anh Tuấn 5 triệu', 'ghi nợ mua điện thoại 10tr'.",
					Parameters: map[string]interface{}{
						"type":     "object",
						"required": []string{"title", "totalAmount", "type"},
						"properties": map[string]interface{}{
							"title": map[string]interface{}{
								"type":        "string",
								"description": "Tên khoản nợ hoặc người liên quan (ví dụ: 'Cho Nam mượn tiền', 'Vay anh Tuấn', 'Mua trả góp laptop').",
							},
							"totalAmount": map[string]interface{}{
								"type":        "number",
								"description": "Tổng số tiền nợ/cho vay (VND).",
							},
							"type": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"PAYABLE", "RECEIVABLE"},
								"description": "Bắt buộc: 'PAYABLE' (mình nợ người khác / phải trả), 'RECEIVABLE' (người khác nợ mình / mình cho vay).",
							},
							"dueDate": map[string]interface{}{
								"type":        "string",
								"description": "Hạn trả nợ nếu có (định dạng YYYY-MM-DD).",
							},
							"notes": map[string]interface{}{
								"type":        "string",
								"description": "Ghi chú thêm về khoản nợ.",
							},
						},
					},
				},
			},
		},
		{
			preview: (*chatService).previewDebtRepayment,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "record_debt_repayment",
					Description: "Ghi nhận một lần thanh toán trả nợ hoặc thu hồi nợ (toàn phần hoặc một phần). Hãy gọi tool này khi người dùng nói: 'Nam vừa trả tôi 1 triệu', 'tôi vừa trả nợ anh Tuấn 2tr'.",
					Parameters: map[string]interface{}{
						"type":     "object",
						"required": []string{"debtTitle", "amount"},
						"properties": map[string]interface{}{
							"debtTitle": map[string]interface{}{
								"type":        "string",
								"description": "Tên khoản nợ hoặc tên người trả/được trả (ví dụ: 'Nam', 'Anh Tuấn').",
							},
							"amount": map[string]interface{}{
								"type":        "number",
								"description": "Số tiền thanh toán trả nợ đợt này (VND).",
							},
							"walletName": map[string]interface{}{
								"type":        "string",
								"description": "Tên ví nhận tiền thu hồi hoặc ví dùng để trích tiền trả nợ.",
							},
							"notes": map[string]interface{}{
								"type":        "string",
								"description": "Ghi chú thanh toán.",
							},
						},
					},
				},
			},
		},
		{
			preview: (*chatService).previewSetBudget,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "set_budget",
					Description: "Thiết lập hoặc cập nhật hạn mức ngân sách chi tiêu hàng tháng cho một danh mục. Hãy gọi tool này khi người dùng nói: 'đặt ngân sách ăn uống 3 triệu/tháng', 'hạn mức mua sắm tháng này là 2tr'.",
					Parameters: map[string]interface{}{
						"type":     "object",
						"required": []string{"categoryName", "amount"},
						"properties": map[string]interface{}{
							"categoryName": map[string]interface{}{
								"type":        "string",
								"description": "Tên danh mục cần đặt ngân sách (ví dụ: 'Ăn uống', 'Mua sắm', 'Di chuyển').",
							},
							"amount": map[string]interface{}{
								"type":        "number",
								"description": "Hạn mức ngân sách bằng số (VND). Ví dụ: 3000000.",
							},
							"periodStart": map[string]interface{}{
								"type":        "string",
								"description": "Ngày bắt đầu chu kỳ ngân sách (YYYY-MM-DD), mặc định là ngày đầu tháng.",
							},
						},
					},
				},
			},
		},
		{
			run: (*chatService).toolCompareFinancialPeriods,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "compare_financial_periods",
					Description: "So sánh tình hình thu/chi giữa 2 tháng khác nhau để xem biến động tăng giảm các danh mục. Hãy gọi tool này khi người dùng hỏi: 'so sánh chi tiêu tháng 7 và tháng 8', 'tháng này tiêu nhiều hơn tháng trước không?'.",
					Parameters: map[string]interface{}{
						"type":     "object",
						"required": []string{"firstMonth", "secondMonth"},
						"properties": map[string]interface{}{
							"firstMonth": map[string]interface{}{
								"type":        "string",
								"description": "Tháng thứ nhất cần so sánh (định dạng YYYY-MM, ví dụ: '2026-07').",
							},
							"secondMonth": map[string]interface{}{
								"type":        "string",
								"description": "Tháng thứ hai cần so sánh (định dạng YYYY-MM, ví dụ: '2026-08').",
							},
						},
					},
				},
			},
		},
		{
			run: (*chatService).toolGetFinancialTargets,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "get_financial_targets",
					Description: "Kiểm tra tiến độ thực hiện mục tiêu tài chính tháng (mục tiêu tiết kiệm tích lũy hoặc hạn mức trần chi tiêu). Hãy gọi tool này khi người dùng hỏi: 'tiến độ mục tiêu tài chính tháng này', 'tôi đã tiết kiệm được bao nhiêu so với mục tiêu?'.",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"month": map[string]interface{}{
								"type":        "integer",
								"description": "Tháng (1-12), mặc định là tháng hiện tại.",
							},
							"year": map[string]interface{}{
								"type":        "integer",
								"description": "Năm (YYYY), mặc định là năm hiện tại.",
							},
						},
					},
				},
			},
		},
		{
			preview: (*chatService).previewSetTarget,
			def: ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        "set_financial_target",
					Description: "Thiết lập mục tiêu tài chính tháng (Mục tiêu tiết kiệm SAVINGS hoặc Hạn mức chi tiêu tối đa SPENDING_LIMIT). Hãy gọi tool này khi người dùng nói: 'đặt mục tiêu tiết kiệm tháng này 10 triệu', 'tháng này chỉ được tiêu tối đa 15 triệu'.",
					Parameters: map[string]interface{}{
						"type":     "object",
						"required": []string{"targetType", "targetAmount"},
						"properties": map[string]interface{}{
							"targetType": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"SAVINGS", "SPENDING_LIMIT"},
								"description": "Loại mục tiêu: 'SAVINGS' (tiết kiệm tích lũy) hoặc 'SPENDING_LIMIT' (hạn mức chi tiêu tối đa).",
							},
							"targetAmount": map[string]interface{}{
								"type":        "number",
								"description": "Số tiền mục tiêu (VND). Ví dụ: 10000000.",
							},
							"month": map[string]interface{}{
								"type":        "integer",
								"description": "Tháng áp dụng (1-12), mặc định tháng hiện tại.",
							},
							"year": map[string]interface{}{
								"type":        "integer",
								"description": "Năm áp dụng (YYYY), mặc định năm hiện tại.",
							},
						},
					},
				},
			},
		},
	}
}

func (s *chatService) toolGetFinancialOverview(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	startDate, endDate := resolveDateRange(args)

	summary, err := s.reportService.GetSummary(ctx, userID, startDate, endDate)
	if err != nil {
		s.logger.Error("failed to get financial summary", zap.Error(err))
		return nil, err
	}

	// A wallet listing error only leaves the balance at 0.
	totalBalance := 0.0
	if wallets, err := s.walletRepo.ListByUserID(ctx, userID); err == nil {
		totalBalance = dto.SummarizeWallets(wallets).TotalBalance
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

func (s *chatService) toolGetBudgetStatus(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	startDate, endDate := resolveDateRange(args)

	// Only the budgets of the requested month: the rollover keeps one row per
	// category per month, so listing every budget would repeat categories.
	budgets, err := s.budgetService.ListActiveBudgets(ctx, userID, budgetReferenceDate(args))
	if err != nil {
		return nil, err
	}

	type BudgetStatusItem struct {
		Category  string  `json:"category"`
		Limit     float64 `json:"limit"`
		Spent     float64 `json:"spent"`
		Remaining float64 `json:"remaining"`
		Percent   float64 `json:"percentage"`
		Status    string  `json:"status"` // "OK", "WARNING", "EXCEEDED"
	}

	var items []BudgetStatusItem
	var exceededCategories []string

	// One SQL aggregation for all categories (same expense definition as
	// reports and targets) instead of loading each category's rows.
	spentByCategory := map[uuid.UUID]float64{}
	breakdown, err := s.reportService.GetCategoryBreakdown(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	for _, item := range breakdown.Items {
		spentByCategory[item.CategoryID] = item.TotalAmount
	}

	for _, b := range budgets {
		catName := b.CategoryName
		spent := spentByCategory[b.CategoryID]

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

func (s *chatService) toolCompareFinancialPeriods(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error) {
	m1, _ := args["firstMonth"].(string)
	m2, _ := args["secondMonth"].(string)

	now := util.Now()
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
	now := util.Now()
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

// budgetReferenceDate is the day whose budgets the budget tools report: today
// (app timezone) for the current month or when no valid "month" (YYYY-MM) is
// given, otherwise the last day of the requested month.
func budgetReferenceDate(args map[string]interface{}) time.Time {
	now := util.Now()
	if m, ok := args["month"].(string); ok && m != "" {
		if t, err := time.Parse("2006-01", m); err == nil && (t.Year() != now.Year() || t.Month() != now.Month()) {
			_, next := util.MonthRangeOf(t.Year(), t.Month())
			return next.AddDate(0, 0, -1)
		}
	}
	return now
}

func resolveDateRange(args map[string]interface{}) (startDate, endDate string) {
	now := util.Now()
	if m, ok := args["month"].(string); ok && m != "" {
		if t, err := time.Parse("2006-01", m); err == nil {
			start, next := util.MonthRangeOf(t.Year(), t.Month())
			end := next.AddDate(0, 0, -1)
			return start.Format(time.DateOnly), end.Format(time.DateOnly)
		}
	}

	if sd, ok := args["startDate"].(string); ok && sd != "" {
		startDate = sd
	} else {
		// Default to first day of current month (app timezone)
		monthStart, _ := util.MonthRange(now)
		startDate = monthStart.Format(time.DateOnly)
	}

	if ed, ok := args["endDate"].(string); ok && ed != "" {
		endDate = ed
	} else {
		endDate = now.Format(time.DateOnly)
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
