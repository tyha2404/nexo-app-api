package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

func TestExecuteFinancialTool_RefusesWriteTools(t *testing.T) {
	e := newToolEnv(t)
	e.addCategory("Ăn uống", model.CategoryTypeExpense)
	res, err := e.svc.executeFinancialTool(context.Background(), e.userID, toolCall(t, "c", "create_transaction", map[string]interface{}{"amount": 1.0, "walletName": "tiền mặt"}))
	require.Error(t, err, "write tools only run through a confirmed proposal")
	assert.Nil(t, res)
	assert.Empty(t, e.txnSvc.created)
}

func TestRunToolCall_HidesInternalErrors(t *testing.T) {
	e := newToolEnv(t)
	turn := &chatTurn{}

	e.txnRepo.err = errors.New("pq: relation \"transactions\" does not exist")
	res := e.svc.runToolCall(context.Background(), e.userID, turn, toolCall(t, "c", "search_transactions", nil))
	assert.Equal(t, hiddenSystemError, toolError(t, res))
	assert.Equal(t, "Lỗi thực thi công cụ", res.ToolTitle)

	e.txnRepo.err = constant.ErrNotFound
	res = e.svc.runToolCall(context.Background(), e.userID, turn, toolCall(t, "c", "search_transactions", nil))
	assert.Equal(t, constant.ErrNotFound.Error(), toolError(t, res))
}

func TestToolSearchTransactions_FiltersAndResolution(t *testing.T) {
	e := newToolEnv(t)
	food := e.addCategory("Ăn uống", model.CategoryTypeExpense)
	e.addCategory("Du lịch", model.CategoryTypeExpense, uuid.New()) // another user's
	cash := e.wallets.wallets[0]
	desc := "Phở"
	e.txnRepo.rows = []model.Transaction{
		{Amount: 50_000, Type: model.TransactionTypeExpense, Description: &desc, WalletID: &cash.ID, Category: &food,
			TransactionDate: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)},
		{Amount: 30_000, Type: model.TransactionTypeExpense, TransactionDate: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)},
	}

	out := decodeTool(t, e.call(t, "search_transactions", map[string]interface{}{
		"month": "2026-11", "type": "EXPENSE", "keyword": "phở", "sortBy": "amount_desc", "minAmount": 10_000.0, "maxAmount": 100_000.0,
		"limit": 500.0, "categoryName": "ăn", "walletName": "tiền mặt",
	}))
	assert.Equal(t, 50, e.txnRepo.lastLimit, "limit capped at 50")
	f := e.txnRepo.lastFilters
	assert.Equal(t, "2026-11-01", f["startDate"])
	assert.Equal(t, "2026-11-30", f["endDate"])
	assert.Equal(t, "EXPENSE", f["type"])
	assert.Equal(t, "phở", f["keyword"])
	assert.Equal(t, "amount_desc", f["sortBy"])
	assert.Equal(t, 10_000.0, f["minAmount"])
	assert.Equal(t, 100_000.0, f["maxAmount"])
	assert.Equal(t, food.ID.String(), f["categoryId"])
	assert.Equal(t, cash.ID.String(), f["walletId"])
	assert.Equal(t, 80_000.0, out["sum_amount"])
	assert.Equal(t, 2.0, out["return_count"])
	items := out["transactions"].([]interface{})
	first := items[0].(map[string]interface{})
	assert.Equal(t, "Ví Tiền mặt", first["wallet"])
	assert.Equal(t, "Ăn uống", first["category"])
	assert.Equal(t, "Phở", first["description"])
	assert.Equal(t, "2026-11-02", first["date"])
	second := items[1].(map[string]interface{})
	assert.Equal(t, "", second["wallet"])
	assert.Equal(t, "", second["category"])

	// Defaults: limit 10, no category/wallet filters, wallet names still resolved.
	e.call(t, "search_transactions", map[string]interface{}{})
	assert.Equal(t, 10, e.txnRepo.lastLimit)
	assert.NotContains(t, e.txnRepo.lastFilters, "categoryId")
	assert.NotContains(t, e.txnRepo.lastFilters, "walletId")

	// Another user's category name is not resolvable.
	out = decodeTool(t, e.call(t, "search_transactions", map[string]interface{}{"categoryName": "Du lịch"}))
	assert.Contains(t, out["error"], "Du lịch")
	assert.Equal(t, []interface{}{"Ăn uống"}, out["hints"])

	out = decodeTool(t, e.call(t, "search_transactions", map[string]interface{}{"walletName": "MoMo"}))
	assert.Contains(t, out["error"], "MoMo")
	assert.Len(t, out["hints"], len(e.wallets.wallets))

	e.txnRepo.err = errInternalRepo
	res := e.call(t, "search_transactions", map[string]interface{}{})
	assert.Equal(t, hiddenSystemError, toolError(t, res))

	e.wallets.listErr = errInternalRepo
	res = e.call(t, "search_transactions", map[string]interface{}{"walletName": "tiền mặt"})
	assert.Equal(t, hiddenSystemError, toolError(t, res))
	requireOnlyUser(t, e.userID, e.txnRepo.users)
}

func TestToolListCategories(t *testing.T) {
	e := newToolEnv(t)
	food := e.addCategory("Ăn uống", model.CategoryTypeExpense)
	salary := e.addCategory("Lương", model.CategoryTypeIncome)
	e.addCategory("Cổ phiếu", model.CategoryTypeInvestment)
	e.addCategory("Theirs", model.CategoryTypeExpense, uuid.New())
	e.report.breakdown = &dto.CategoryBreakdownReport{Items: []dto.CategoryBreakdownItem{
		{CategoryID: food.ID, TotalAmount: 700_000}, {CategoryID: salary.ID, TotalAmount: 99}, {CategoryID: uuid.Nil, TotalAmount: 5},
	}}
	e.budget.budgets = []dto.BudgetResponse{{CategoryID: food.ID, Amount: 1_000_000}}

	out := decodeTool(t, e.call(t, "list_categories", map[string]interface{}{"type": "ALL"}))
	assert.Equal(t, 3.0, out["count"], "only the user's own categories")
	assert.Equal(t, util.Now().Format("2006-01"), out["month"])
	byName := map[string]map[string]interface{}{}
	for _, it := range out["categories"].([]interface{}) {
		m := it.(map[string]interface{})
		byName[m["name"].(string)] = m
	}
	assert.Equal(t, 700_000.0, byName["Ăn uống"]["spentThisMonth"])
	assert.Equal(t, 1_000_000.0, byName["Ăn uống"]["budgetLimit"])
	assert.Nil(t, byName["Lương"]["spentThisMonth"], "spending is only reported for expense categories")

	out = decodeTool(t, e.call(t, "list_categories", map[string]interface{}{"type": "INCOME"}))
	assert.Equal(t, 1.0, out["count"])

	// Report/budget failures degrade gracefully.
	e.report.breakErr = errInternalRepo
	e.budget.listErr = errInternalRepo
	out = decodeTool(t, e.call(t, "list_categories", nil))
	assert.Equal(t, 3.0, out["count"])
	start, _ := util.MonthRange(util.Now())
	assert.Contains(t, e.report.ranges, [2]string{start.Format("2006-01-02"), util.Now().Format("2006-01-02")}, "month to date in the app timezone")
}

func TestToolGetMonthlyTrend(t *testing.T) {
	cases := []struct {
		name   string
		months interface{}
		want   int
	}{
		{"default", nil, 6},
		{"below minimum uses default", 1.0, 6},
		{"explicit", 3.0, 3},
		{"capped at 12", 48.0, 12},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newToolEnv(t)
			args := map[string]interface{}{}
			if tc.months != nil {
				args["months"] = tc.months
			}
			out := decodeTool(t, e.call(t, "get_monthly_trend", args))
			assert.Equal(t, float64(tc.want), out["months_count"])
			points := out["monthly_points"].([]interface{})
			require.Len(t, points, tc.want)

			// Contiguous calendar months, oldest first, ending with the current month.
			cur, _ := util.MonthRange(util.Now())
			for i, p := range points {
				wantMonth := cur.AddDate(0, -(tc.want - 1 - i), 0).Format("2006-01")
				assert.Equal(t, wantMonth, p.(map[string]interface{})["month"])
			}
			assert.Equal(t, []int{tc.want}, e.report.trendMonths, "one trend query for the whole window")
			assert.Empty(t, e.report.ranges, "no per-month summary queries")
		})
	}

	e := newToolEnv(t)
	cur, _ := util.MonthRange(util.Now())
	e.report.summaries[cur.Format("2006-01-02")] = &dto.SummaryReport{TotalIncome: 10_000_000, TotalExpense: 7_500_000}
	e.report.summaries[cur.AddDate(0, -1, 0).Format("2006-01-02")] = &dto.SummaryReport{TotalIncome: 0, TotalExpense: 1_000_000}
	out := decodeTool(t, e.call(t, "get_monthly_trend", map[string]interface{}{"months": 2.0}))
	points := out["monthly_points"].([]interface{})
	assert.Equal(t, 0.0, points[0].(map[string]interface{})["savingsRatePercent"], "no income: 0 not NaN")
	assert.Equal(t, 25.0, points[1].(map[string]interface{})["savingsRatePercent"])
	assert.Equal(t, 5_000_000.0, out["avg_income_per_month"])
	assert.Equal(t, 4_250_000.0, out["avg_expense_per_month"])
	assert.Equal(t, 750_000.0, out["avg_savings_per_month"])

	e.report.summaryErr = errInternalRepo
	assert.Equal(t, hiddenSystemError, toolError(t, e.call(t, "get_monthly_trend", nil)))
}

// referenceMonthlyTrend is the pre-refactor get_monthly_trend algorithm (one
// GetSummary per month), kept to prove the tool's output did not change.
func referenceMonthlyTrend(t *testing.T, e *toolEnv, monthsBack int) string {
	t.Helper()
	currentMonthStart, _ := util.MonthRange(util.Now())
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
		start := currentMonthStart.AddDate(0, -i, 0)
		end := start.AddDate(0, 1, -1)
		summary, err := e.report.GetSummary(context.Background(), e.userID, start.Format(time.DateOnly), end.Format(time.DateOnly))
		require.NoError(t, err)
		net := summary.TotalIncome - summary.TotalExpense
		rate := 0.0
		if summary.TotalIncome > 0 {
			rate = (net / summary.TotalIncome) * 100.0
		}
		points = append(points, MonthPoint{Month: start.Format("2006-01"), Income: summary.TotalIncome, Expense: summary.TotalExpense,
			Investment: summary.TotalInvestment, NetSavings: net, SavingsRate: math.Round(rate*10) / 10})
		totalIncome += summary.TotalIncome
		totalExpense += summary.TotalExpense
	}
	raw, _ := json.Marshal(map[string]interface{}{
		"months_count":          monthsBack,
		"monthly_points":        points,
		"avg_income_per_month":  math.Round(totalIncome/float64(monthsBack)*100) / 100,
		"avg_expense_per_month": math.Round(totalExpense/float64(monthsBack)*100) / 100,
		"avg_savings_per_month": math.Round((totalIncome-totalExpense)/float64(monthsBack)*100) / 100,
	})
	return string(raw)
}

func TestToolGetMonthlyTrend_MatchesPerMonthAlgorithm(t *testing.T) {
	e := newToolEnv(t)
	cur, _ := util.MonthRange(util.Now())
	e.report.summaries[cur.Format("2006-01-02")] = &dto.SummaryReport{TotalIncome: 10_000_000, TotalExpense: 7_333_333, TotalInvestment: 1_000_000}
	e.report.summaries[cur.AddDate(0, -1, 0).Format("2006-01-02")] = &dto.SummaryReport{TotalExpense: 1_000_000, TotalInvestment: 250_000}
	e.report.summaries[cur.AddDate(0, -4, 0).Format("2006-01-02")] = &dto.SummaryReport{TotalIncome: 3_000_000, TotalExpense: 3_500_001}
	for _, months := range []float64{2, 3, 6, 12} {
		res := e.call(t, "get_monthly_trend", map[string]interface{}{"months": months})
		assert.Equal(t, fmt.Sprintf("Đang phân tích xu hướng %d tháng...", int(months)), res.ToolTitle)
		assert.Equal(t, referenceMonthlyTrend(t, e, int(months)), res.ResultJSON, "months=%v", months)
	}
}

func TestToolGetWalletDetail(t *testing.T) {
	e := newToolEnv(t)
	cash := dto.WalletResponse{ID: uuid.New(), Name: "Ví Tiền mặt", Balance: 500_000}
	bank := dto.WalletResponse{ID: uuid.New(), Name: "Techcombank", Balance: 9_000_000}
	e.wallet.summary = &dto.WalletSummaryResponse{Wallets: []dto.WalletResponse{cash, bank}}
	desc := "ATM"
	e.txnRepo.rows = []model.Transaction{{Amount: 200_000, Type: model.TransactionTypeExpense, Description: &desc,
		Category: &model.Category{Name: "Rút tiền"}, TransactionDate: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}}

	res := e.call(t, "get_wallet_detail", map[string]interface{}{"walletName": "techcombank", "limit": 100.0})
	out := decodeTool(t, res)
	assert.Equal(t, 20, e.txnRepo.lastLimit, "limit capped at 20")
	assert.Equal(t, bank.ID.String(), e.txnRepo.lastFilters["walletId"])
	assert.Equal(t, util.Now().Format("2006-01-02"), e.txnRepo.lastFilters["endDate"])
	assert.Equal(t, "Techcombank", out["wallet"].(map[string]interface{})["name"])
	assert.Equal(t, 1.0, out["transaction_count"])
	txn := out["recent_transactions"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "Rút tiền", txn["category"])
	assert.Equal(t, "ATM", txn["description"])
	assert.Contains(t, res.ToolTitle, "Techcombank")

	e.call(t, "get_wallet_detail", map[string]interface{}{"walletName": "số dư ví tiền mặt"})
	assert.Equal(t, cash.ID.String(), e.txnRepo.lastFilters["walletId"], "query containing the wallet name")
	assert.Equal(t, 10, e.txnRepo.lastLimit)

	out = decodeTool(t, e.call(t, "get_wallet_detail", map[string]interface{}{"walletName": "MoMo"}))
	assert.Contains(t, out["error"], "MoMo")
	assert.Equal(t, []interface{}{"Ví Tiền mặt", "Techcombank"}, out["hints"])

	e.txnRepo.err = errInternalRepo
	assert.Equal(t, hiddenSystemError, toolError(t, e.call(t, "get_wallet_detail", map[string]interface{}{"walletName": "Techcombank"})))
	e.wallet.summaryErr = errInternalRepo
	assert.Equal(t, hiddenSystemError, toolError(t, e.call(t, "get_wallet_detail", map[string]interface{}{"walletName": "Techcombank"})))
}

func TestToolGetDebtDetail(t *testing.T) {
	e := newToolEnv(t)
	assert.NotEmpty(t, toolError(t, e.call(t, "get_debt_detail", map[string]interface{}{})))

	nam := dto.DebtResponse{ID: uuid.New(), Title: "Vay anh Nam", TotalAmount: 1_000_000, Remaining: 600_000}
	e.debt.debts = []dto.DebtResponse{nam, {ID: uuid.New(), Title: "Cho Lan vay"}}

	res := e.call(t, "get_debt_detail", map[string]interface{}{"debtTitle": "anh nam"})
	out := decodeTool(t, res)
	assert.Equal(t, nam.ID.String(), out["id"])
	assert.Equal(t, 600_000.0, out["remaining"])

	out = decodeTool(t, e.call(t, "get_debt_detail", map[string]interface{}{"debtTitle": "Hùng"}))
	assert.Contains(t, out["error"], "Hùng")
	assert.Equal(t, []interface{}{"Vay anh Nam", "Cho Lan vay"}, out["hints"])

	e.debt.getErr = errInternalRepo
	assert.Equal(t, hiddenSystemError, toolError(t, e.call(t, "get_debt_detail", map[string]interface{}{"debtTitle": "Nam"})))
}

func TestToolGetInvestmentSummary(t *testing.T) {
	e := newToolEnv(t)
	status, pnl, desc := "HOLDING", 0.0, "VNM"
	e.txnSvc.list = []dto.TransactionResponse{{ID: uuid.New(), TransactionDate: "2026-10-01", Amount: 10_000_000, Status: &status, RealizedPnL: &pnl, CategoryName: "Cổ phiếu", Description: &desc}}
	e.txnSvc.summary = &dto.TransactionSummaryDTO{SumAmount: 25_000_000, HoldingAmount: 10_000_000, HoldingCount: 1, RealizedPnL: 1_500_000}

	res := e.call(t, "get_investment_summary", nil)
	assert.Equal(t, map[string]interface{}{"type": "INVESTMENT"}, e.txnSvc.lastFilters)
	assert.Equal(t, 15, e.txnSvc.lastLimit)
	out := decodeTool(t, res)
	assert.Equal(t, 10_000_000.0, out["holding_amount"])
	assert.Equal(t, 1_500_000.0, out["realized_pnl"])
	assert.Equal(t, 25_000_000.0, out["total_invested"])
	recent := out["recent"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "HOLDING", recent["status"])
	assert.Equal(t, "VNM", recent["description"])
	assert.Contains(t, res.ActionCard.Description, "10.000.000")

	e.txnSvc.list = []dto.TransactionResponse{{ID: uuid.New()}}
	recent = decodeTool(t, e.call(t, "get_investment_summary", nil))["recent"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "", recent["status"])
	assert.Equal(t, "", recent["description"])

	e.txnSvc.listErr = errInternalRepo
	assert.Equal(t, hiddenSystemError, toolError(t, e.call(t, "get_investment_summary", nil)))
}

func TestToolSearchFinancialKnowledge(t *testing.T) {
	e := newToolEnv(t)
	assert.NotEmpty(t, toolError(t, e.call(t, "search_financial_knowledge", map[string]interface{}{})))
	assert.Empty(t, e.rag.queries)

	e.rag.results = []KnowledgeSearchResult{{Topic: "budget", Title: "50/30/20", Score: 0.8}, {Topic: "x", Title: "y", Fallback: true}}
	res := e.call(t, "search_financial_knowledge", map[string]interface{}{"query": "quỹ khẩn cấp"})
	out := decodeTool(t, res)
	assert.Equal(t, []string{"quỹ khẩn cấp"}, e.rag.queries)
	assert.Equal(t, []int{3}, e.rag.limits)
	assert.Equal(t, 1.0, out["fallback_count"])
	assert.Contains(t, out["message"], "fallback")
	assert.Equal(t, "KNOWLEDGE_SOURCE", res.ActionCard.ActionType)

	e.rag.results = []KnowledgeSearchResult{{Topic: "budget", Score: 0.8}}
	out = decodeTool(t, e.call(t, "search_financial_knowledge", map[string]interface{}{"query": "q"}))
	assert.Nil(t, out["message"])

	e.rag.err = errInternalRepo
	out = decodeTool(t, e.call(t, "search_financial_knowledge", map[string]interface{}{"query": "q"}))
	assert.Contains(t, out["message"], "Không tìm thấy", "a RAG failure degrades to no results")
	assert.Nil(t, out["error"])
}

// ------------------------- write proposals ---------------------------------

func TestParseToolDate(t *testing.T) {
	fallback := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "2026-01-31", parseToolDate("2026-01-31", fallback).Format("2006-01-02"))
	for _, bad := range []string{"", "31/01/2026", "2026-02-30", "tomorrow"} {
		assert.True(t, parseToolDate(bad, fallback).Equal(fallback), bad)
	}
}

func TestNormalizeCategoryTypeAndVi(t *testing.T) {
	cases := []struct {
		in   interface{}
		want string
		vi   string
	}{
		{"income", "INCOME", "Thu nhập"},
		{" Investment ", "INVESTMENT", "Đầu tư"},
		{"EXPENSE", "EXPENSE", "Chi tiêu"},
		{"saving", "EXPENSE", "Chi tiêu"},
		{nil, "EXPENSE", "Chi tiêu"},
		{42.0, "EXPENSE", "Chi tiêu"},
	}
	for _, tc := range cases {
		got := normalizeCategoryType(tc.in)
		assert.Equal(t, tc.want, got, "%v", tc.in)
		assert.Equal(t, tc.vi, categoryTypeVi(got))
	}
}

func TestPreviewCreateCategory(t *testing.T) {
	e := newToolEnv(t)
	existing := e.addCategory("Cà phê", model.CategoryTypeExpense)
	ctx := context.Background()

	_, errRes, err := e.svc.previewCreateCategory(ctx, e.userID, map[string]interface{}{"name": "  "})
	require.NoError(t, err)
	assert.NotEmpty(t, toolError(t, errRes))

	// An existing category needs no confirmation: reported right away, nothing created.
	p, res, err := e.svc.previewCreateCategory(ctx, e.userID, map[string]interface{}{"name": "cà phê"})
	require.NoError(t, err)
	assert.Nil(t, p)
	out := decodeTool(t, res)
	assert.Equal(t, true, out["isDuplicate"])
	assert.Equal(t, existing.ID.String(), out["categoryId"])
	assert.Empty(t, e.cats.created)

	p, errRes, err = e.svc.previewCreateCategory(ctx, e.userID, map[string]interface{}{"name": " Thú cưng ", "type": "investment", "description": " chó "})
	require.NoError(t, err)
	require.Nil(t, errRes)
	assert.Equal(t, &createCategoryArgs{Name: "Thú cưng", Type: "INVESTMENT", Description: "chó"}, p.args)
	assert.Contains(t, p.description, "Đầu tư")
	assert.Empty(t, e.cats.created, "preview never writes")
}

func TestPreviewDebtRepayment(t *testing.T) {
	e := newToolEnv(t)
	ctx := context.Background()

	_, errRes, err := e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Nam", "amount": 0.0})
	require.NoError(t, err)
	assert.NotEmpty(t, toolError(t, errRes))

	_, errRes, err = e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Nam", "amount": 1.0})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, errRes), "không có khoản nợ")

	payable := dto.DebtResponse{ID: uuid.New(), Title: "Vay anh Nam", Type: model.DebtTypePayable, Remaining: 600_000}
	receivable := dto.DebtResponse{ID: uuid.New(), Title: "Cho Lan vay", Type: model.DebtTypeReceivable, Remaining: 300_000}
	e.debt.debts = []dto.DebtResponse{payable, receivable}

	_, errRes, err = e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Hùng", "amount": 1.0})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, errRes), "Hùng")

	p, errRes, err := e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "nam", "amount": 100_000.0, "notes": "đợt 1"})
	require.NoError(t, err)
	require.Nil(t, errRes)
	assert.Equal(t, "Ghi trả nợ", p.kind)
	assert.Equal(t, payable.ID, p.args.(*debtRepaymentArgs).DebtID, "resolved to the stored debt")
	assert.Equal(t, payable.ID.String(), p.data["debtId"])
	assert.Contains(t, p.description, "600.000")

	p, _, err = e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Lan", "amount": "50000"})
	require.NoError(t, err)
	assert.Equal(t, "Ghi thu nợ", p.kind)
	assert.Equal(t, 50_000.0, p.args.(*debtRepaymentArgs).Amount)
	assert.Empty(t, e.debt.repaid, "preview never writes")

	e.debt.getErr = errInternalRepo
	_, _, err = e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Lan", "amount": 1.0})
	require.ErrorIs(t, err, errInternalRepo)
}

func TestPreviewSetBudget(t *testing.T) {
	e := newToolEnv(t)
	ctx := context.Background()
	e.addCategory("Ăn uống", model.CategoryTypeExpense)
	e.addCategory("Du lịch", model.CategoryTypeExpense, uuid.New())

	_, errRes, err := e.svc.previewSetBudget(ctx, e.userID, map[string]interface{}{"categoryName": "", "amount": 1.0})
	require.NoError(t, err)
	assert.NotEmpty(t, toolError(t, errRes))

	_, errRes, err = e.svc.previewSetBudget(ctx, e.userID, map[string]interface{}{"categoryName": "Du lịch", "amount": 1.0})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, errRes), "Du lịch", "another user's category is not found")

	// The default period starts on the 1st of the service clock's month in the app timezone:
	// 2026-10-31T20:00Z is already 1 Nov in Vietnam.
	*e.clock = time.Date(2026, 10, 31, 20, 0, 0, 0, time.UTC)
	p, errRes, err := e.svc.previewSetBudget(ctx, e.userID, map[string]interface{}{"categoryName": "ăn uống", "amount": 2_000_000.0})
	require.NoError(t, err)
	require.Nil(t, errRes)
	budgetArgs := p.args.(*setBudgetArgs)
	assert.Equal(t, "2026-11-01", budgetArgs.PeriodStart.Format("2006-01-02"))
	assert.Equal(t, "Ăn uống", budgetArgs.CategoryName)

	p, _, err = e.svc.previewSetBudget(ctx, e.userID, map[string]interface{}{"categoryName": "Ăn uống", "amount": 1.0, "periodStart": "2027-01-01"})
	require.NoError(t, err)
	assert.Equal(t, "2027-01-01", p.args.(*setBudgetArgs).PeriodStart.Format("2006-01-02"))
	assert.Empty(t, e.budget.created, "preview never writes")
}

func TestPreviewSetTargetValidation(t *testing.T) {
	e := newToolEnv(t)
	preview := func(args map[string]interface{}) (*writeProposal, *FinancialToolResult) {
		p, errRes, err := e.svc.previewSetTarget(context.Background(), e.userID, args)
		require.NoError(t, err)
		return p, errRes
	}
	_, errRes := preview(map[string]interface{}{"targetType": "EXPENSE", "targetAmount": 1.0, "month": 13.0})
	assert.NotEmpty(t, toolError(t, errRes))
	_, errRes = preview(map[string]interface{}{"targetType": "LOTTERY", "targetAmount": 1.0})
	assert.NotEmpty(t, toolError(t, errRes))
	_, errRes = preview(map[string]interface{}{"targetType": "EXPENSE"})
	assert.NotEmpty(t, toolError(t, errRes))
	p, errRes := preview(map[string]interface{}{"targetType": "spending", "targetAmount": 3.0})
	require.Nil(t, errRes)
	targetArgs := p.args.(*setTargetArgs)
	assert.Equal(t, "EXPENSE", targetArgs.TargetType)
	assert.Equal(t, int(e.clock.Month()), targetArgs.Month)
}

func TestPreviewTransferAndCreateDebtErrors(t *testing.T) {
	e := newToolEnv(t)
	ctx := context.Background()

	for _, args := range []map[string]interface{}{
		{"amount": 0.0, "fromWalletName": "tiền mặt", "toWalletName": "ngân hàng"},
		{"amount": 1.0, "fee": -1.0, "fromWalletName": "tiền mặt", "toWalletName": "ngân hàng"},
		{"amount": 1.0, "fromWalletName": "tiền mặt"},
		{"amount": 1.0, "fromWalletName": "MoMo", "toWalletName": "ngân hàng"},
		{"amount": 1.0, "fromWalletName": "tiền mặt", "toWalletName": "MoMo"},
		{"amount": 1.0, "fromWalletName": "tiền mặt", "toWalletName": "Ví Tiền Mặt"},
	} {
		_, errRes, err := e.svc.previewTransfer(ctx, e.userID, args)
		require.NoError(t, err)
		assert.NotEmpty(t, toolError(t, errRes), "%v", args)
	}

	p, errRes, err := e.svc.previewTransfer(ctx, e.userID, map[string]interface{}{"amount": 1_000_000.0, "fee": 3_300.0, "fromWalletName": "ngân hàng", "toWalletName": "tiền mặt"})
	require.NoError(t, err)
	require.Nil(t, errRes)
	assert.Contains(t, p.description, "phí 3.300")

	e.wallets.wallets = nil
	_, errRes, err = e.svc.previewTransfer(ctx, e.userID, map[string]interface{}{"amount": 1.0, "fromWalletName": "a", "toWalletName": "b"})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, errRes), "chưa có ví")

	e.wallets.listErr = errInternalRepo
	_, _, err = e.svc.previewTransfer(ctx, e.userID, map[string]interface{}{"amount": 1.0, "fromWalletName": "a", "toWalletName": "b"})
	require.ErrorIs(t, err, errInternalRepo)
	assert.Empty(t, e.wallet.transfers)

	p, errRes, _ = e.svc.previewCreateDebt(ctx, e.userID, map[string]interface{}{"title": "Vay", "totalAmount": 1.0, "dueDate": "soon"})
	require.Nil(t, errRes)
	assert.Nil(t, p.args.(*createDebtArgs).DueDate)
	assert.Equal(t, model.DebtTypePayable, p.args.(*createDebtArgs).Type)
	assert.Equal(t, "Ghi khoản đi vay", p.kind)
}

// Every write tool goes through propose -> confirm: nothing is written while
// proposing, and the confirmed action writes exactly once for the same user.
func TestWriteTools_ProposeThenConfirmEndToEnd(t *testing.T) {
	cases := []struct {
		tool    string
		args    map[string]interface{}
		written func(e *toolEnv) int
		check   func(t *testing.T, e *toolEnv)
	}{
		{
			tool:    "create_transaction",
			args:    map[string]interface{}{"amount": 50_000.0, "categoryName": "Ăn uống", "walletName": "tiền mặt", "transactionDate": "2026-11-07"},
			written: func(e *toolEnv) int { return len(e.txnSvc.created) },
			check: func(t *testing.T, e *toolEnv) {
				assert.Equal(t, "2026-11-07", e.txnSvc.created[0].TransactionDate.Format("2006-01-02"))
				assert.Equal(t, e.wallets.wallets[0].ID, *e.txnSvc.created[0].WalletID)
			},
		},
		{
			tool:    "create_category",
			args:    map[string]interface{}{"name": "Thú cưng", "type": "EXPENSE"},
			written: func(e *toolEnv) int { return len(e.cats.created) },
			check: func(t *testing.T, e *toolEnv) {
				assert.Equal(t, "Thú cưng", e.cats.created[0].Name)
				assert.Equal(t, e.userID, e.cats.created[0].UserID)
			},
		},
		{
			tool:    "transfer_between_wallets",
			args:    map[string]interface{}{"amount": 200_000.0, "fromWalletName": "ngân hàng", "toWalletName": "tiền mặt"},
			written: func(e *toolEnv) int { return len(e.wallet.transfers) },
			check: func(t *testing.T, e *toolEnv) {
				assert.Equal(t, e.wallets.wallets[1].ID, e.wallet.transfers[0].FromWalletID)
				assert.Equal(t, e.wallets.wallets[0].ID, e.wallet.transfers[0].ToWalletID)
			},
		},
		{
			tool:    "create_debt",
			args:    map[string]interface{}{"title": "Vay Nam", "totalAmount": 1_000_000.0, "type": "PAYABLE", "dueDate": "2026-12-31"},
			written: func(e *toolEnv) int { return len(e.debt.created) },
			check: func(t *testing.T, e *toolEnv) {
				assert.Equal(t, "2026-12-31", e.debt.created[0].DueDate.Format("2006-01-02"))
			},
		},
		{
			tool:    "record_debt_repayment",
			args:    map[string]interface{}{"debtTitle": "anh Nam", "amount": 100_000.0},
			written: func(e *toolEnv) int { return len(e.debt.repaid) },
			check: func(t *testing.T, e *toolEnv) {
				assert.Equal(t, e.debt.debts[0].ID, e.debt.repaid[0].debtID)
			},
		},
		{
			tool:    "set_budget",
			args:    map[string]interface{}{"categoryName": "Ăn uống", "amount": 3_000_000.0, "periodStart": "2026-12-01"},
			written: func(e *toolEnv) int { return len(e.budget.created) },
			check: func(t *testing.T, e *toolEnv) {
				assert.Equal(t, "2026-12-01", e.budget.created[0].PeriodStart.Format("2006-01-02"))
			},
		},
		{
			tool:    "set_financial_target",
			args:    map[string]interface{}{"targetType": "savings", "targetAmount": 5_000_000.0, "month": 12.0, "year": 2026.0},
			written: func(e *toolEnv) int { return len(e.target.upserts) },
			check: func(t *testing.T, e *toolEnv) {
				assert.Equal(t, dto.UpsertTargetRequest{TargetType: "INVESTMENT", TargetAmount: 5_000_000, Month: 12, Year: 2026}, e.target.upserts[0])
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			e := newToolEnv(t)
			e.addCategory("Ăn uống", model.CategoryTypeExpense)
			e.debt.debts = []dto.DebtResponse{{ID: uuid.New(), Title: "Vay anh Nam", TotalAmount: 1_000_000, Remaining: 1_000_000, Type: model.DebtTypePayable}}
			raw := mustJSON(t, tc.args)
			msgID := uuid.New()

			res, err := e.svc.proposeWriteTool(context.Background(), e.userID, uuid.New(), msgID,
				ToolCall{ID: "call", Type: "function", Function: ToolCallFunction{Name: tc.tool, Arguments: raw}})
			require.NoError(t, err)
			require.NotNil(t, res.ActionCard, "result: %s", res.ResultJSON)
			assert.Equal(t, ActionTypePendingConfirmation, res.ActionCard.ActionType)
			assert.Equal(t, "pending_confirmation", decodeTool(t, res)["status"])
			assert.Zero(t, tc.written(e), "nothing is written before confirmation")
			data := res.ActionCard.Data.(map[string]interface{})
			actionID := uuid.MustParse(data["actionId"].(string))
			assert.Equal(t, tc.tool, data["toolName"])

			_, err = e.svc.ConfirmAction(context.Background(), uuid.New(), actionID)
			assert.ErrorIs(t, err, ErrPendingActionNotFound, "another user cannot confirm")
			assert.Zero(t, tc.written(e))

			result, err := e.svc.ConfirmAction(context.Background(), e.userID, actionID)
			require.NoError(t, err)
			assert.Equal(t, "CONFIRMED", result.Status)
			assert.Equal(t, 1, tc.written(e))
			tc.check(t, e)

			for _, users := range [][]uuid.UUID{e.wallet.users, e.debt.users, e.budget.users, e.target.users, e.txnSvc.users} {
				requireOnlyUser(t, e.userID, users)
			}
		})
	}
}

func TestProposeWriteTool_RejectsReadOnlyToolAndBadJSON(t *testing.T) {
	e := newToolEnv(t)
	_, err := e.svc.proposeWriteTool(context.Background(), e.userID, uuid.New(), uuid.New(),
		ToolCall{Function: ToolCallFunction{Name: "list_wallets", Arguments: "{}"}})
	require.Error(t, err)

	// Unparseable arguments behave like empty arguments: a validation error, no proposal.
	res, err := e.svc.proposeWriteTool(context.Background(), e.userID, uuid.New(), uuid.New(),
		ToolCall{Function: ToolCallFunction{Name: "create_debt", Arguments: "{oops"}})
	require.NoError(t, err)
	assert.Nil(t, res.ActionCard)
	assert.NotEmpty(t, toolError(t, res))
}

func TestConfirmAction_ToolErrorKeepsPending(t *testing.T) {
	e := newToolEnv(t)
	e.addCategory("Ăn uống", model.CategoryTypeExpense)
	res, err := e.svc.proposeWriteTool(context.Background(), e.userID, uuid.New(), uuid.Nil,
		ToolCall{Function: ToolCallFunction{Name: "set_budget", Arguments: mustJSON(t, map[string]interface{}{"categoryName": "Ăn uống", "amount": 1.0})}})
	require.NoError(t, err)
	actionID := uuid.MustParse(res.ActionCard.Data.(map[string]interface{})["actionId"].(string))

	// The category disappears before confirmation (CreateBudget reports it
	// not found): the tool returns an error payload.
	e.budget.createErr = fmt.Errorf("category %w", constant.ErrNotFound)
	_, err = e.svc.ConfirmAction(context.Background(), e.userID, actionID)
	require.ErrorIs(t, err, ErrPendingActionFailed)
	assert.Contains(t, err.Error(), "Không tìm thấy danh mục 'Ăn uống'")
	got, getErr := e.svc.pendingActions.Get(e.userID, actionID)
	require.NoError(t, getErr)
	assert.Equal(t, PendingActionPending, got.Status, "a failed confirm can be retried")

	e.budget.createErr = constant.ErrAlreadyExists
	_, err = e.svc.ConfirmAction(context.Background(), e.userID, actionID)
	assert.ErrorIs(t, err, ErrPendingActionFailed, "business errors are reported as a failed action")
	assert.Contains(t, err.Error(), constant.ErrAlreadyExists.Error())
}

// Wallets "Techcombank" and "Techcombank Visa" (defaultWallets): an exact name
// wins, a unique partial name resolves, an ambiguous one is reported instead of
// silently using the last match.
func TestToolSearchTransactions_WalletNameResolution(t *testing.T) {
	e := newToolEnv(t)
	byName := map[string]uuid.UUID{}
	for _, w := range e.wallets.wallets {
		byName[w.Name] = w.ID
	}

	e.call(t, "search_transactions", map[string]interface{}{"walletName": "techcombank"})
	assert.Equal(t, byName["Techcombank"].String(), e.txnRepo.lastFilters["walletId"], "exact name")

	e.call(t, "search_transactions", map[string]interface{}{"walletName": "visa"})
	assert.Equal(t, byName["Techcombank Visa"].String(), e.txnRepo.lastFilters["walletId"], "unique partial")

	e.txnRepo.lastFilters = nil
	out := decodeTool(t, e.call(t, "search_transactions", map[string]interface{}{"walletName": "techcom"}))
	assert.Contains(t, out["error"], "nhiều ví")
	assert.ElementsMatch(t, []interface{}{"Techcombank", "Techcombank Visa"}, out["hints"])
	assert.Nil(t, e.txnRepo.lastFilters, "no search with a guessed wallet")
}

func TestToolGetDebtDetail_IgnoresUntitledDebts(t *testing.T) {
	e := newToolEnv(t)
	nam := dto.DebtResponse{ID: uuid.New(), Title: "Vay anh Nam"}
	e.debt.debts = []dto.DebtResponse{{ID: uuid.New(), Title: ""}, {ID: uuid.New(), Title: "Nam"}, nam}

	out := decodeTool(t, e.call(t, "get_debt_detail", map[string]interface{}{"debtTitle": "Hùng"}))
	assert.NotEmpty(t, out["error"], "an untitled debt must not match every query")

	// Exact title wins over an earlier containment match.
	out = decodeTool(t, e.call(t, "get_debt_detail", map[string]interface{}{"debtTitle": "vay anh nam"}))
	assert.Equal(t, nam.ID.String(), out["id"])
}

func TestToolSearchTransactions_PrefersExactCategoryName(t *testing.T) {
	e := newToolEnv(t)
	e.addCategory("Ăn uống ngoài", model.CategoryTypeExpense)
	food := e.addCategory("Ăn uống", model.CategoryTypeExpense)

	e.call(t, "search_transactions", map[string]interface{}{"categoryName": "ăn uống"})
	assert.Equal(t, food.ID.String(), e.txnRepo.lastFilters["categoryId"], "exact name wins over an earlier partial match")
}

func TestToolGetWalletDetail_ReportsAmbiguousName(t *testing.T) {
	e := newToolEnv(t)
	e.wallet.summary = &dto.WalletSummaryResponse{Wallets: []dto.WalletResponse{
		{ID: uuid.New(), Name: "Techcombank"},
		{ID: uuid.New(), Name: "Techcombank Visa"},
	}}
	e.txnRepo.lastFilters = nil

	out := decodeTool(t, e.call(t, "get_wallet_detail", map[string]interface{}{"walletName": "techcom"}))
	assert.Contains(t, out["error"], "nhiều ví")
	assert.ElementsMatch(t, []interface{}{"Techcombank", "Techcombank Visa"}, out["hints"])
	assert.Nil(t, e.txnRepo.lastFilters, "no lookup with a guessed wallet")

	e.call(t, "get_wallet_detail", map[string]interface{}{"walletName": "techcombank"})
	assert.Equal(t, e.wallet.summary.Wallets[0].ID.String(), e.txnRepo.lastFilters["walletId"], "exact name preferred")
}
