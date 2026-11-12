package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

const hiddenSystemError = "Có lỗi hệ thống, vui lòng thử lại sau"

func TestResolveDateRange(t *testing.T) {
	now := util.Now()
	monthStart, _ := util.MonthRange(now)
	today := now.Format("2006-01-02")

	tests := []struct {
		name      string
		args      map[string]interface{}
		wantStart string
		wantEnd   string
	}{
		{"month", map[string]interface{}{"month": "2026-08"}, "2026-08-01", "2026-08-31"},
		{"leap february", map[string]interface{}{"month": "2024-02"}, "2024-02-01", "2024-02-29"},
		{"february", map[string]interface{}{"month": "2026-02"}, "2026-02-01", "2026-02-28"},
		{"december", map[string]interface{}{"month": "2026-12"}, "2026-12-01", "2026-12-31"},
		{"month wins over dates", map[string]interface{}{"month": "2026-04", "startDate": "2020-01-01"}, "2026-04-01", "2026-04-30"},
		{"explicit dates", map[string]interface{}{"startDate": "2026-01-05", "endDate": "2026-03-07"}, "2026-01-05", "2026-03-07"},
		{"invalid month falls back to dates", map[string]interface{}{"month": "08/2026", "startDate": "2026-01-01", "endDate": "2026-01-31"}, "2026-01-01", "2026-01-31"},
		{"default is current month to today", map[string]interface{}{}, monthStart.Format("2006-01-02"), today},
		{"only start", map[string]interface{}{"startDate": "2026-05-01"}, "2026-05-01", today},
		{"non-string month ignored", map[string]interface{}{"month": 8.0}, monthStart.Format("2006-01-02"), today},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end := resolveDateRange(tc.args)
			assert.Equal(t, tc.wantStart, start)
			assert.Equal(t, tc.wantEnd, end)
		})
	}
}

func TestFormatVND(t *testing.T) {
	cases := map[float64]string{0: "0", 999: "999", 1000: "1.000", 1234567: "1.234.567", -50000: "-50.000", 1999.6: "2.000", 12_500_000: "12.500.000"}
	for in, want := range cases {
		assert.Equal(t, want, formatVND(in), "%v", in)
	}
}

func TestExecuteFinancialTool_UnknownToolAndBadArgs(t *testing.T) {
	e := newToolEnv(t)
	res := e.call(t, "drop_database", nil)
	assert.Contains(t, toolError(t, res), "drop_database")

	// Unparseable arguments are treated as no arguments (defaults apply).
	res, err := e.svc.executeFinancialTool(context.Background(), e.userID, ToolCall{Function: ToolCallFunction{Name: "list_wallets", Arguments: "{not json"}})
	require.NoError(t, err)
	assert.Empty(t, toolError(t, res))
}

func TestExecuteFinancialTool_DispatchesEveryTool(t *testing.T) {
	e := newToolEnv(t)
	food := e.addCategory("Ăn uống", model.CategoryTypeExpense)
	e.debt.debts = []dto.DebtResponse{{ID: uuid.New(), Title: "Vay Nam", TotalAmount: 1_000_000, Remaining: 1_000_000, Type: model.DebtTypePayable}}
	e.wallet.summary = &dto.WalletSummaryResponse{Wallets: []dto.WalletResponse{{ID: uuid.New(), Name: "Ví Tiền mặt"}}}
	e.rag.results = []KnowledgeSearchResult{{Topic: "budget", Title: "50/30/20", Score: 0.9}}
	_ = food

	calls := map[string]map[string]interface{}{
		"get_financial_overview":     {},
		"list_recent_transactions":   {},
		"create_transaction":         {"amount": 10_000.0, "walletName": "tiền mặt"},
		"create_category":            {"name": "Thú cưng"},
		"get_budget_status":          {},
		"get_debt_summary":           {},
		"list_wallets":               {},
		"get_spending_by_category":   {},
		"transfer_between_wallets":   {"amount": 1.0, "fromWalletName": "tiền mặt", "toWalletName": "ngân hàng"},
		"create_debt":                {"title": "Vay", "totalAmount": 5.0},
		"record_debt_repayment":      {"debtTitle": "Vay Nam", "amount": 5.0},
		"set_budget":                 {"categoryName": "Ăn uống", "amount": 5.0},
		"compare_financial_periods":  {},
		"get_financial_targets":      {},
		"set_financial_target":       {"targetType": "EXPENSE", "targetAmount": 5.0},
		"search_transactions":        {},
		"list_categories":            {},
		"get_monthly_trend":          {},
		"get_wallet_detail":          {"walletName": "tiền mặt"},
		"get_debt_detail":            {"debtTitle": "Nam"},
		"get_investment_summary":     {},
		"search_financial_knowledge": {"query": "quy tắc 50/30/20"},
	}
	defined := map[string]bool{}
	for _, d := range GetFinancialToolDefinitions() {
		defined[d.Function.Name] = true
	}
	for name, args := range calls {
		t.Run(name, func(t *testing.T) {
			require.True(t, defined[name], "tool %s must be advertised to the model", name)
			res := e.call(t, name, args)
			assert.NotEmpty(t, res.ToolTitle)
			assert.Empty(t, toolError(t, res), "unexpected tool error: %s", res.ResultJSON)
		})
	}
	assert.Len(t, calls, len(defined), "every advertised tool is dispatched")

	for _, users := range [][]uuid.UUID{e.report.users, e.wallet.users, e.debt.users, e.budget.users, e.target.users, e.txnSvc.users, e.txnRepo.users, e.wallets.users} {
		requireOnlyUser(t, e.userID, users)
	}
	for _, c := range e.cats.rows {
		require.Equal(t, e.userID, c.UserID, "categories are created for the caller only")
	}
}

func TestToolGetFinancialOverview(t *testing.T) {
	e := newToolEnv(t)
	e.report.summaries["2026-08-01"] = &dto.SummaryReport{TotalIncome: 20_000_000, TotalExpense: 15_000_000, TotalInvestment: 2_000_000}
	e.wallets.wallets[0].Balance, e.wallets.wallets[0].IsIncludedInTotal = 42_000_000, true

	res := e.call(t, "get_financial_overview", map[string]interface{}{"month": "2026-08"})
	out := decodeTool(t, res)
	assert.Equal(t, "2026-08-01", out["start_date"])
	assert.Equal(t, "2026-08-31", out["end_date"])
	assert.Equal(t, 5_000_000.0, out["net_savings"])
	assert.Equal(t, 25.0, out["savings_rate_percent"])
	assert.Equal(t, 42_000_000.0, out["total_wallet_balance"])
	require.NotNil(t, res.ActionCard)
	assert.Equal(t, "FINANCIAL_SUMMARY", res.ActionCard.ActionType)
	assert.Contains(t, res.ActionCard.Description, "20.000.000")

	// No income: savings rate is 0, not NaN/Inf; a wallet failure is not fatal.
	e.report.summaries["2026-07-01"] = &dto.SummaryReport{TotalExpense: 100}
	e.wallets.listErr = errInternalRepo
	out = decodeTool(t, e.call(t, "get_financial_overview", map[string]interface{}{"month": "2026-07"}))
	assert.Equal(t, 0.0, out["savings_rate_percent"])
	assert.Equal(t, 0.0, out["total_wallet_balance"])
	assert.Equal(t, -100.0, out["net_savings"])

	e.report.summaryErr = errInternalRepo
	_, err := e.svc.toolGetFinancialOverview(context.Background(), e.userID, map[string]interface{}{})
	require.ErrorIs(t, err, errInternalRepo)
}

func TestToolListRecentTransactions(t *testing.T) {
	e := newToolEnv(t)
	desc := "Phở"
	e.txnSvc.list = []dto.TransactionResponse{
		{ID: uuid.New(), TransactionDate: "2026-11-07", Type: "EXPENSE", Amount: 50_000, CategoryName: "Ăn uống", Description: &desc},
		{ID: uuid.New(), TransactionDate: "2026-11-06", Type: "INCOME", Amount: 1, CategoryName: "Lương"},
	}

	out := decodeTool(t, e.call(t, "list_recent_transactions", map[string]interface{}{}))
	assert.Equal(t, 5, e.txnSvc.lastLimit, "default limit")
	assert.Empty(t, e.txnSvc.lastFilters)
	assert.Equal(t, 2.0, out["return_count"])
	items := out["transactions"].([]interface{})
	assert.Equal(t, "Phở", items[0].(map[string]interface{})["description"])
	assert.Equal(t, "", items[1].(map[string]interface{})["description"])

	e.call(t, "list_recent_transactions", map[string]interface{}{"limit": 500.0, "type": "ALL", "startDate": "2026-11-01", "endDate": "2026-11-30"})
	assert.Equal(t, 20, e.txnSvc.lastLimit, "limit is capped at 20")
	assert.Equal(t, map[string]interface{}{"startDate": "2026-11-01", "endDate": "2026-11-30"}, e.txnSvc.lastFilters, "type ALL is not a filter")

	e.call(t, "list_recent_transactions", map[string]interface{}{"limit": -3.0, "type": "INCOME"})
	assert.Equal(t, 5, e.txnSvc.lastLimit)
	assert.Equal(t, "INCOME", e.txnSvc.lastFilters["type"])

	e.txnSvc.listErr = errInternalRepo
	_, err := e.svc.toolListRecentTransactions(context.Background(), e.userID, map[string]interface{}{})
	require.ErrorIs(t, err, errInternalRepo)
}

func TestToolCreateCategory(t *testing.T) {
	e := newToolEnv(t)
	existing := e.addCategory("Ăn uống", model.CategoryTypeExpense)
	e.addCategory("Thú cưng", model.CategoryTypeExpense, uuid.New()) // another user's

	assert.NotEmpty(t, toolError(t, e.call(t, "create_category", map[string]interface{}{"name": "   "})))
	assert.Empty(t, e.cats.created)

	// Duplicate (case-insensitive) is reported, not created again.
	res := e.call(t, "create_category", map[string]interface{}{"name": "ĂN UỐNG", "type": "expense"})
	out := decodeTool(t, res)
	assert.Equal(t, true, out["isDuplicate"])
	assert.Equal(t, existing.ID.String(), out["categoryId"])
	assert.Empty(t, e.cats.created)

	// Another user's category with the same name does not count as a duplicate.
	res = e.call(t, "create_category", map[string]interface{}{"name": "Thú cưng", "description": "  chó mèo  "})
	out = decodeTool(t, res)
	assert.Nil(t, out["isDuplicate"])
	require.Len(t, e.cats.created, 1)
	assert.Equal(t, e.userID, e.cats.created[0].UserID)
	assert.Equal(t, model.CategoryTypeExpense, e.cats.created[0].Type)
	assert.Equal(t, "chó mèo", *e.cats.created[0].Description)
	assert.Equal(t, "CATEGORY_CREATED", res.ActionCard.ActionType)

	for _, tc := range []struct{ in, want, vi string }{
		{"income", "INCOME", "Thu nhập"},
		{" Investment ", "INVESTMENT", "Đầu tư"},
		{"SAVINGS", "EXPENSE", "Chi tiêu"},
	} {
		res = e.call(t, "create_category", map[string]interface{}{"name": "Mục " + tc.in, "type": tc.in})
		out = decodeTool(t, res)
		assert.Equal(t, tc.want, out["type"], tc.in)
		assert.Contains(t, res.ActionCard.Description, tc.vi)
	}

	e.cats.createErr = errors.New(`pq: duplicate key value violates unique constraint "idx_categories"`)
	res = e.call(t, "create_category", map[string]interface{}{"name": "Mới"})
	msg := toolError(t, res)
	assert.Contains(t, msg, hiddenSystemError)
	assert.NotContains(t, msg, "pq:", "DB details must not reach the model")
}

func TestToolGetBudgetStatus(t *testing.T) {
	e := newToolEnv(t)
	food, coffee, rent := uuid.New(), uuid.New(), uuid.New()
	e.budget.budgets = []dto.BudgetResponse{
		{CategoryID: food, CategoryName: "Ăn uống", Amount: 1_000_000},
		{CategoryID: coffee, CategoryName: "Cà phê", Amount: 500_000},
		{CategoryID: rent, CategoryName: "Nhà", Amount: 0},
	}
	e.report.breakdown = &dto.CategoryBreakdownReport{Items: []dto.CategoryBreakdownItem{
		{CategoryID: food, TotalAmount: 1_200_000},
		{CategoryID: coffee, TotalAmount: 400_000},
	}}

	res := e.call(t, "get_budget_status", map[string]interface{}{"month": "2026-10"})
	out := decodeTool(t, res)
	assert.Equal(t, "2026-10-01", out["start_date"])
	assert.Equal(t, 3.0, out["budget_count"])
	items := out["budgets"].([]interface{})
	byName := map[string]map[string]interface{}{}
	for _, it := range items {
		m := it.(map[string]interface{})
		byName[m["category"].(string)] = m
	}
	assert.Equal(t, "EXCEEDED", byName["Ăn uống"]["status"])
	assert.Equal(t, 120.0, byName["Ăn uống"]["percentage"])
	assert.Equal(t, -200_000.0, byName["Ăn uống"]["remaining"])
	assert.Equal(t, "WARNING", byName["Cà phê"]["status"])
	assert.Equal(t, 80.0, byName["Cà phê"]["percentage"])
	assert.Equal(t, "OK", byName["Nhà"]["status"], "a zero budget never divides by zero")
	require.NotNil(t, res.ActionCard)
	assert.Equal(t, "BUDGET_ALERT", res.ActionCard.ActionType)
	assert.Contains(t, res.ActionCard.Description, "Ăn uống")
	assert.NotContains(t, res.ActionCard.Description, "Cà phê")

	// Nothing exceeded: no alert card.
	e.report.breakdown = &dto.CategoryBreakdownReport{}
	res = e.call(t, "get_budget_status", map[string]interface{}{})
	assert.Nil(t, res.ActionCard)

	e.report.breakErr = errInternalRepo
	_, err := e.svc.toolGetBudgetStatus(context.Background(), e.userID, map[string]interface{}{})
	require.ErrorIs(t, err, errInternalRepo)

	e.budget.listErr = errInternalRepo
	_, err = e.svc.toolGetBudgetStatus(context.Background(), e.userID, map[string]interface{}{})
	require.ErrorIs(t, err, errInternalRepo)
}

func TestToolGetDebtSummaryListWalletsAndSpending(t *testing.T) {
	e := newToolEnv(t)
	e.debt.summary = &dto.DebtSummaryResponse{TotalPayable: 3_000_000, TotalReceivable: 1_000_000, OverdueCount: 1, PendingCount: 2}
	e.debt.debts = []dto.DebtResponse{{ID: uuid.New(), Title: "Vay Nam"}, {ID: uuid.New(), Title: "Cho Lan vay"}}
	out := decodeTool(t, e.call(t, "get_debt_summary", nil))
	assert.Equal(t, 3_000_000.0, out["total_payable"])
	assert.Equal(t, 1_000_000.0, out["total_receivable"])
	assert.Equal(t, 1.0, out["overdue_count"])
	assert.Equal(t, 2.0, out["active_debts_count"])

	e.debt.summaryErr = errInternalRepo
	_, err := e.svc.toolGetDebtSummary(context.Background(), e.userID, nil)
	require.ErrorIs(t, err, errInternalRepo)

	e.wallet.summary = &dto.WalletSummaryResponse{TotalBalance: 9_000_000, BaseCurrency: "VND", TotalWallets: 1}
	out = decodeTool(t, e.call(t, "list_wallets", nil))
	assert.Equal(t, 9_000_000.0, out["totalBalance"])
	e.wallet.summaryErr = errInternalRepo
	_, err = e.svc.toolListWallets(context.Background(), e.userID, nil)
	require.ErrorIs(t, err, errInternalRepo)

	e.report.breakdown = &dto.CategoryBreakdownReport{TotalExpense: 700, Items: []dto.CategoryBreakdownItem{{CategoryName: "Ăn uống", TotalAmount: 700}}}
	out = decodeTool(t, e.call(t, "get_spending_by_category", map[string]interface{}{"startDate": "2026-11-01", "endDate": "2026-11-05"}))
	assert.Equal(t, 700.0, out["total_expense"])
	assert.Equal(t, "2026-11-05", out["end_date"])
	assert.Len(t, out["categories"], 1)
	e.report.breakErr = errInternalRepo
	_, err = e.svc.toolGetSpendingByCategory(context.Background(), e.userID, nil)
	require.ErrorIs(t, err, errInternalRepo)
}

func TestToolTransferBetweenWallets(t *testing.T) {
	e := newToolEnv(t)
	wallets := e.wallets.wallets

	res := e.call(t, "transfer_between_wallets", map[string]interface{}{
		"amount": "1500000", "fee": 3300.0, "fromWalletName": "techcombank", "toWalletName": "tien mat", "notes": "rút tiền",
	})
	require.Empty(t, toolError(t, res))
	require.Len(t, e.wallet.transfers, 1)
	tr := e.wallet.transfers[0]
	assert.Equal(t, wallets[3].ID, tr.FromWalletID, "exact match 'Techcombank' wins over 'Techcombank Visa'")
	assert.Equal(t, wallets[0].ID, tr.ToWalletID)
	assert.Equal(t, 1_500_000.0, tr.Amount)
	assert.Equal(t, 3_300.0, tr.Fee)
	assert.Equal(t, "rút tiền", *tr.Note)
	assert.Equal(t, "WALLET_TRANSFER", res.ActionCard.ActionType)
	assert.Contains(t, res.ActionCard.Description, "1.500.000")

	// Without notes the note is nil.
	e.call(t, "transfer_between_wallets", map[string]interface{}{"amount": 1.0, "fromWalletName": "ví điện tử", "toWalletName": "ngân hàng"})
	assert.Nil(t, e.wallet.transfers[1].Note)

	// Missing one side lists the wallets.
	msg := toolError(t, e.call(t, "transfer_between_wallets", map[string]interface{}{"amount": 1.0, "fromWalletName": "tiền mặt"}))
	assert.Contains(t, msg, "Techcombank Visa")
	msg = toolError(t, e.call(t, "transfer_between_wallets", map[string]interface{}{"amount": 1.0, "fromWalletName": "tiền mặt", "toWalletName": "MoMo"}))
	assert.True(t, strings.HasPrefix(msg, "Ví đích"), msg)
	assert.Len(t, e.wallet.transfers, 2, "rejected transfers never reach the wallet service")

	// Business errors are shown, internal ones hidden.
	e.wallet.transferErr = constant.ErrInsufficientBalance
	msg = toolError(t, e.call(t, "transfer_between_wallets", map[string]interface{}{"amount": 1.0, "fromWalletName": "tiền mặt", "toWalletName": "ngân hàng"}))
	assert.Equal(t, constant.ErrInsufficientBalance.Error(), msg)
	e.wallet.transferErr = errors.New("pq: deadlock detected")
	msg = toolError(t, e.call(t, "transfer_between_wallets", map[string]interface{}{"amount": 1.0, "fromWalletName": "tiền mặt", "toWalletName": "ngân hàng"}))
	assert.Equal(t, hiddenSystemError, msg)

	// No wallets at all is a friendly tool error (it used to be a system error).
	e.wallets.wallets = nil
	msg = toolError(t, e.call(t, "transfer_between_wallets", map[string]interface{}{"amount": 1.0, "fromWalletName": "a", "toWalletName": "b"}))
	assert.Equal(t, "Bạn chưa có ví nào để chuyển tiền.", msg)
}

func TestToolCreateDebt(t *testing.T) {
	e := newToolEnv(t)

	res := e.call(t, "create_debt", map[string]interface{}{"title": "Cho Lan vay", "totalAmount": 2_000_000.0, "type": "receivable", "dueDate": "2026-12-31", "notes": "trả cuối năm"})
	require.Len(t, e.debt.created, 1)
	req := e.debt.created[0]
	assert.Equal(t, model.DebtTypeReceivable, req.Type)
	assert.Equal(t, "2026-12-31", req.DueDate.Format("2006-01-02"))
	assert.Equal(t, "trả cuối năm", req.Notes)
	assert.Nil(t, req.WalletID, "AI-created debts never move a wallet balance")
	assert.Equal(t, "DEBT_CREATED", res.ActionCard.ActionType)
	assert.Contains(t, res.ActionCard.Title, "cho vay")

	res = e.call(t, "create_debt", map[string]interface{}{"title": "Vay Nam", "totalAmount": 500.0, "dueDate": "31/12/2026"})
	req = e.debt.created[1]
	assert.Equal(t, model.DebtTypePayable, req.Type)
	assert.Nil(t, req.DueDate, "an unparseable due date is dropped")
	assert.Contains(t, res.ActionCard.Title, "phải trả")

	assert.NotEmpty(t, toolError(t, e.call(t, "create_debt", map[string]interface{}{"title": "  ", "totalAmount": 1.0})))
	assert.Len(t, e.debt.created, 2)

	e.debt.createErr = errInternalRepo
	_, err := e.svc.previewThenExecute(context.Background(), e.userID, toolCall(t, "c", "create_debt", map[string]interface{}{"title": "x", "totalAmount": 1.0}))
	require.ErrorIs(t, err, errInternalRepo)
}

func TestToolRecordDebtRepayment(t *testing.T) {
	e := newToolEnv(t)

	msg := toolError(t, e.call(t, "record_debt_repayment", map[string]interface{}{"debtTitle": "Nam", "amount": 1.0}))
	assert.Contains(t, msg, "không có khoản nợ")

	nam := dto.DebtResponse{ID: uuid.New(), Title: "Vay anh Nam", TotalAmount: 1_000_000, Remaining: 1_000_000}
	lan := dto.DebtResponse{ID: uuid.New(), Title: "Lan", TotalAmount: 500_000, Remaining: 500_000}
	e.debt.debts = []dto.DebtResponse{nam, lan}

	msg = toolError(t, e.call(t, "record_debt_repayment", map[string]interface{}{"debtTitle": "Hùng", "amount": 1.0}))
	assert.Contains(t, msg, "Hùng")
	assert.Empty(t, e.debt.repaid)

	res := e.call(t, "record_debt_repayment", map[string]interface{}{"debtTitle": "anh nam", "amount": 400_000.0, "notes": "lần 1"})
	require.Len(t, e.debt.repaid, 1)
	assert.Equal(t, nam.ID, e.debt.repaid[0].debtID)
	assert.Equal(t, 400_000.0, e.debt.repaid[0].req.Amount)
	assert.Nil(t, e.debt.repaid[0].req.WalletID)
	assert.Equal(t, "DEBT_REPAID", res.ActionCard.ActionType)
	assert.Contains(t, res.ActionCard.Description, "600.000")

	e.debt.repayErr = ErrInvalidRepayment
	_, err := e.svc.previewThenExecute(context.Background(), e.userID, toolCall(t, "c", "record_debt_repayment", map[string]interface{}{"debtTitle": "Lan", "amount": 1.0}))
	require.ErrorIs(t, err, ErrInvalidRepayment)

	// A failed debt lookup is a system error (hidden from the model by the stream).
	e.debt.getErr = errInternalRepo
	_, err = e.svc.previewThenExecute(context.Background(), e.userID, toolCall(t, "c", "record_debt_repayment", map[string]interface{}{"debtTitle": "Lan", "amount": 1.0}))
	require.ErrorIs(t, err, errInternalRepo)
}

func TestToolSetBudget(t *testing.T) {
	e := newToolEnv(t)
	msg := toolError(t, e.call(t, "set_budget", map[string]interface{}{"categoryName": "Ăn uống", "amount": 1.0}))
	assert.Contains(t, msg, "Không tìm thấy danh mục 'Ăn uống'", "no categories at all is a friendly tool error")

	food := e.addCategory("Ăn uống", model.CategoryTypeExpense)
	e.addCategory("Du lịch", model.CategoryTypeExpense, uuid.New()) // another user's

	msg = toolError(t, e.call(t, "set_budget", map[string]interface{}{"categoryName": "Du lịch", "amount": 1.0}))
	assert.Contains(t, msg, "Du lịch", "another user's category is never used")
	assert.Empty(t, e.budget.created)

	res := e.call(t, "set_budget", map[string]interface{}{"categoryName": "ăn", "amount": 3_000_000.0})
	require.Len(t, e.budget.created, 1)
	req := e.budget.created[0]
	assert.Equal(t, food.ID, req.CategoryID)
	assert.Equal(t, 3_000_000.0, req.Amount)
	wantStart, _ := util.MonthRange(*e.clock)
	assert.Equal(t, wantStart.Format("2006-01-02"), req.PeriodStart.Format("2006-01-02"), "default period is the current month in the app timezone")
	assert.Equal(t, "BUDGET_SET", res.ActionCard.ActionType)

	e.call(t, "set_budget", map[string]interface{}{"categoryName": "Ăn uống", "amount": 1.0, "periodStart": "2026-12-01"})
	assert.Equal(t, "2026-12-01", e.budget.created[1].PeriodStart.Format("2006-01-02"))

	e.budget.createErr = errInternalRepo
	_, err := e.svc.previewThenExecute(context.Background(), e.userID, toolCall(t, "c", "set_budget", map[string]interface{}{"categoryName": "Ăn uống", "amount": 1.0}))
	require.ErrorIs(t, err, errInternalRepo)
}

// The budget repository, the DB check constraint (model.Budget) and the DTO
// validator all use lowercase "monthly"/"yearly"; set_budget must too.
func TestToolSetBudget_UsesStoredPeriodType(t *testing.T) {
	e := newToolEnv(t)
	e.addCategory("Ăn uống", model.CategoryTypeExpense)
	e.call(t, "set_budget", map[string]interface{}{"categoryName": "Ăn uống", "amount": 1.0})
	require.Len(t, e.budget.created, 1)
	assert.Equal(t, "monthly", e.budget.created[0].PeriodType)
}

func TestToolCompareFinancialPeriods(t *testing.T) {
	e := newToolEnv(t)
	e.report.summaries["2026-09-01"] = &dto.SummaryReport{TotalIncome: 20_000_000, TotalExpense: 12_000_000}
	e.report.summaries["2026-10-01"] = &dto.SummaryReport{TotalIncome: 18_000_000, TotalExpense: 15_000_000}

	res := e.call(t, "compare_financial_periods", map[string]interface{}{"firstMonth": "2026-09", "secondMonth": "2026-10"})
	out := decodeTool(t, res)
	diff := out["difference"].(map[string]interface{})
	assert.Equal(t, 3_000_000.0, diff["expense_change"])
	assert.Equal(t, -2_000_000.0, diff["income_change"])
	assert.Equal(t, 8_000_000.0, out["first_month"].(map[string]interface{})["net_savings"])
	assert.Contains(t, e.report.ranges, [2]string{"2026-10-01", "2026-10-31"})

	// Defaults: previous month vs current month (app timezone).
	now := util.Now()
	out = decodeTool(t, e.call(t, "compare_financial_periods", nil))
	assert.Equal(t, now.Format("2006-01"), out["second_month"].(map[string]interface{})["month"])
	assert.Equal(t, now.AddDate(0, -1, 0).Format("2006-01"), out["first_month"].(map[string]interface{})["month"])

	e.report.summaryErr = errInternalRepo
	_, err := e.svc.toolCompareFinancialPeriods(context.Background(), e.userID, nil)
	require.Error(t, err)
}

func TestToolFinancialTargets(t *testing.T) {
	e := newToolEnv(t)
	e.target.summary = &dto.TargetSummaryResponse{Month: 10, Year: 2026,
		Expense: dto.ExpenseSummary{TargetAmount: 10_000_000, SpentAmount: 4_000_000}, Investment: dto.InvestmentSummary{InvestedAmount: 2_000_000}}

	res := e.call(t, "get_financial_targets", map[string]interface{}{"month": 10.0, "year": 2026.0})
	assert.Equal(t, 10, e.target.gotMonth)
	assert.Equal(t, 2026, e.target.gotYear)
	assert.Equal(t, "TARGET_STATUS", res.ActionCard.ActionType)
	assert.Contains(t, res.ActionCard.Description, "4.000.000")

	now := util.Now()
	e.call(t, "get_financial_targets", map[string]interface{}{"month": -1.0})
	assert.Equal(t, int(now.Month()), e.target.gotMonth, "defaults to the current month")
	assert.Equal(t, now.Year(), e.target.gotYear)

	e.target.summaryErr = errInternalRepo
	_, err := e.svc.toolGetFinancialTargets(context.Background(), e.userID, nil)
	require.ErrorIs(t, err, errInternalRepo)
}

func TestToolSetFinancialTarget(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"spending_limit", "EXPENSE"}, {"SPENDING", "EXPENSE"}, {"expense", "EXPENSE"},
		{"savings", "INVESTMENT"}, {"Investment", "INVESTMENT"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			e := newToolEnv(t)
			res := e.call(t, "set_financial_target", map[string]interface{}{"targetType": tc.in, "targetAmount": 5_000_000.0, "month": 12.0, "year": 2026.0})
			require.Len(t, e.target.upserts, 1)
			assert.Equal(t, dto.UpsertTargetRequest{TargetType: tc.want, TargetAmount: 5_000_000, Month: 12, Year: 2026}, e.target.upserts[0])
			out := decodeTool(t, res)
			assert.Equal(t, true, out["success"])
			assert.Equal(t, "TARGET_SET", res.ActionCard.ActionType)
		})
	}

	e := newToolEnv(t)
	now := *e.clock
	e.call(t, "set_financial_target", map[string]interface{}{"targetType": "EXPENSE", "targetAmount": 1.0})
	assert.Equal(t, int(now.Month()), e.target.upserts[0].Month)
	assert.Equal(t, now.Year(), e.target.upserts[0].Year)

	// Invalid targets are rejected by the preview with a tool error.
	assert.NotEmpty(t, toolError(t, e.call(t, "set_financial_target", map[string]interface{}{"targetType": "LOTTERY", "targetAmount": 1.0})))
	assert.NotEmpty(t, toolError(t, e.call(t, "set_financial_target", map[string]interface{}{"targetType": "EXPENSE", "targetAmount": 1.0, "month": 13.0})))
	assert.NotEmpty(t, toolError(t, e.call(t, "set_financial_target", map[string]interface{}{"targetAmount": 1.0})))
	assert.Len(t, e.target.upserts, 1)
}

func TestMatchDebtByTitle(t *testing.T) {
	debts := []dto.DebtResponse{
		{Title: "Vay anh Nam mua xe"},
		{Title: "Nam"},
		{Title: "Cho Lan vay"},
	}
	tests := []struct {
		query string
		want  string
	}{
		{"nam", "Nam"},         // exact (case-insensitive) wins
		{"  NAM  ", "Nam"},     // trimmed
		{"lan", "Cho Lan vay"}, // title contains query
		{"tiền cho lan vay tháng 5", "Cho Lan vay"}, // query contains title
		{"mua xe", "Vay anh Nam mua xe"},
		{"Hùng", ""},
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range tests {
		got := matchDebtByTitle(debts, tc.query)
		if tc.want == "" {
			assert.Nil(t, got, tc.query)
			continue
		}
		require.NotNil(t, got, tc.query)
		assert.Equal(t, tc.want, got.Title, tc.query)
	}
	assert.Nil(t, matchDebtByTitle(nil, "Nam"))
	assert.Nil(t, matchDebtByTitle([]dto.DebtResponse{{Title: ""}, {Title: "  "}}, "bất kỳ"), "untitled debts never match loosely")
}

func TestCategoryNamesAndWalletNames(t *testing.T) {
	assert.Equal(t, []string{}, categoryNames(nil))
	assert.Equal(t, []string{"A", "B"}, categoryNames([]model.Category{{Name: "A"}, {Name: "B"}}))
	assert.Equal(t, []string{"Ví Tiền mặt"}, walletNames(defaultWallets()[:1]))
}

func TestToolErrorJSONEscapesQuotes(t *testing.T) {
	res := &FinancialToolResult{ResultJSON: toolErrorJSON(`Không tìm thấy ví "MoMo"`)}
	assert.Equal(t, `Không tìm thấy ví "MoMo"`, toolError(t, res))
	assert.Equal(t, "", toolResultError("not json"))
	assert.Equal(t, "", toolResultError(`{"ok":true}`))
}

func TestIsBusinessError(t *testing.T) {
	for _, err := range []error{constant.ErrInvalidInput, constant.ErrInsufficientBalance, constant.ErrNotFound, constant.ErrAlreadyExists,
		ErrSameWalletTransfer, fmt.Errorf("wrapped: %w", constant.ErrNotFound), ErrInvalidTransfer} {
		assert.True(t, isBusinessError(err), "%v", err)
	}
	for _, err := range []error{errors.New("pq: boom"), context.DeadlineExceeded} {
		assert.False(t, isBusinessError(err), "%v", err)
		assert.Equal(t, hiddenSystemError, toolErrorMessage(err))
	}
}

func TestToolCreateTransaction_ExecutesWithResolvedWalletAndCategory(t *testing.T) {
	e := newToolEnv(t)
	food := e.addCategory("Ăn uống", model.CategoryTypeExpense)
	e.addCategory("Lương", model.CategoryTypeIncome)

	res := e.call(t, "create_transaction", map[string]interface{}{"amount": 50_000.0, "categoryName": "ăn", "walletName": "tiền mặt", "transactionDate": "2026-11-01"})
	require.Len(t, e.txnSvc.created, 1)
	req := e.txnSvc.created[0]
	assert.Equal(t, food.ID, req.CategoryID)
	assert.Equal(t, e.wallets.wallets[0].ID, *req.WalletID)
	assert.Equal(t, "EXPENSE", req.Type)
	assert.Equal(t, "Ăn uống", *req.Description, "description defaults to the category name")
	assert.Equal(t, "2026-11-01", req.TransactionDate.Format("2006-01-02"))
	assert.Equal(t, "TRANSACTION_CREATED", res.ActionCard.ActionType)
	assert.Empty(t, e.cats.created)

	// Unknown category name is created for the user; income type honoured.
	res = e.call(t, "create_transaction", map[string]interface{}{"amount": 1.0, "type": "income", "categoryName": "Thưởng", "walletName": "tiền mặt", "transactionDate": "bad"})
	require.Len(t, e.cats.created, 1)
	assert.Equal(t, "Thưởng", e.cats.created[0].Name)
	assert.Equal(t, model.CategoryTypeIncome, e.cats.created[0].Type)
	assert.Equal(t, e.userID, e.cats.created[0].UserID)
	assert.Equal(t, "INCOME", e.txnSvc.created[1].Type)
	assert.Equal(t, util.StartOfDay(*e.clock).Format("2006-01-02"), e.txnSvc.created[1].TransactionDate.Format("2006-01-02"), "invalid date falls back to today")
	assert.Contains(t, res.ActionCard.Description, "Tạo mới danh mục")

	// No category given: first existing one of the type.
	e.call(t, "create_transaction", map[string]interface{}{"amount": 1.0, "type": "INCOME", "walletName": "tiền mặt"})
	assert.Equal(t, "Lương", *e.txnSvc.created[2].Description)

	// No category of that type at all: "Khác" is created.
	e2 := newToolEnv(t)
	e2.call(t, "create_transaction", map[string]interface{}{"amount": 1.0, "walletName": "tiền mặt"})
	require.Len(t, e2.cats.created, 1)
	assert.Equal(t, "Khác", e2.cats.created[0].Name)

	// No wallets: transaction without a wallet (no balance effect).
	e3 := newToolEnv(t)
	e3.wallets.wallets = nil
	e3.addCategory("Ăn uống", model.CategoryTypeExpense)
	e3.call(t, "create_transaction", map[string]interface{}{"amount": 1.0})
	require.Len(t, e3.txnSvc.created, 1)
	assert.Nil(t, e3.txnSvc.created[0].WalletID)
}
