package service

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

// Characterization (golden) tests: they lock down the AI chat behaviour —
// SSE event sequence and payloads, what the model sees, stored messages, the
// tool definitions and every write tool's preview and confirm results — so
// refactors of the chat code cannot change it silently. The golden files were
// generated from the pre-refactor code; regenerate only for an intended
// behaviour change: go test ./internal/service -run Characterization -update-golden
var updateGolden = flag.Bool("update-golden", false, "rewrite the characterization golden files")

var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// normalizeGolden pretty-prints v and replaces volatile values: every UUID
// becomes <id-N> (numbered by first appearance) and today's date (the system
// prompt uses the real clock) becomes <today>.
func normalizeGolden(t *testing.T, v interface{}) string {
	t.Helper()
	raw, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	ids := map[string]string{}
	out := uuidPattern.ReplaceAllStringFunc(string(raw), func(id string) string {
		if id == uuid.Nil.String() {
			return "<nil-id>"
		}
		if _, ok := ids[id]; !ok {
			ids[id] = fmt.Sprintf("<id-%d>", len(ids)+1)
		}
		return ids[id]
	})
	return strings.ReplaceAll(out, util.Now().Format("2006-01-02"), "<today>") + "\n"
}

func assertGolden(t *testing.T, name string, v interface{}) {
	t.Helper()
	got := normalizeGolden(t, v)
	path := filepath.Join("testdata", "characterization", name+".json")
	if *updateGolden {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden file %s (run with -update-golden)", path)
	require.Equal(t, string(want), got, "behaviour changed vs golden %s", path)
}

type goldenMessage struct {
	Role, Content, Status string
	ToolResults           interface{} `json:",omitempty"`
}

func storedMessages(r *streamChatRepo) []goldenMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []goldenMessage
	for _, id := range r.order {
		m := r.messages[id]
		gm := goldenMessage{Role: string(m.Role), Content: m.Content, Status: string(m.Status)}
		if m.ToolResults != nil {
			var v interface{}
			_ = json.Unmarshal([]byte(*m.ToolResults), &v)
			gm.ToolResults = v
		}
		out = append(out, gm)
	}
	return out
}

func toolNamesOf(defs []ToolDefinition) []string {
	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Function.Name)
	}
	return names
}

func TestCharacterization_ToolDefinitions(t *testing.T) {
	assertGolden(t, "tool_definitions", GetFinancialToolDefinitions())
}

func TestCharacterization_StreamScenarios(t *testing.T) {
	debtID := uuid.New()
	scenarios := []struct {
		name  string
		setup func(t *testing.T, e *streamEnv)
		req   dto.SendMessageRequest
	}{
		{name: "not_configured", req: dto.SendMessageRequest{Message: "Xin chào"},
			setup: func(_ *testing.T, e *streamEnv) { e.ai.configured = false }},
		{name: "create_session_error", req: dto.SendMessageRequest{Message: "hi"},
			setup: func(_ *testing.T, e *streamEnv) { e.repo.createErr = errInternalRepo }},
		{name: "write_and_read_tools_empty_answer", req: dto.SendMessageRequest{Message: "vừa ăn phở 50k"},
			setup: func(t *testing.T, e *streamEnv) {
				e.addCategory("Ăn uống", model.CategoryTypeExpense)
				e.wallet.summary = &dto.WalletSummaryResponse{TotalWallets: 5}
				e.ai.chatResp = chatResponse(t, "đang xử lý",
					toolCall(t, "call-w", "create_transaction", map[string]interface{}{"amount": 50_000.0, "categoryName": "Ăn uống", "walletName": "tiền mặt", "transactionDate": "2026-11-07"}),
					toolCall(t, "call-r", "list_wallets", map[string]interface{}{}),
				)
			}},
		{name: "read_tool_card_empty_answer", req: dto.SendMessageRequest{Message: "tổng quan tháng 8"},
			setup: func(t *testing.T, e *streamEnv) {
				e.report.summaries["2026-08-01"] = &dto.SummaryReport{TotalIncome: 20_000_000, TotalExpense: 5_000_000, TotalInvestment: 1_000_000}
				e.wallets.wallets[0].Balance, e.wallets.wallets[0].IsIncludedInTotal = 7_000_000, true
				e.ai.chatResp = chatResponse(t, "", toolCall(t, "c1", "get_financial_overview", map[string]interface{}{"month": "2026-08"}))
			}},
		{name: "tool_then_streamed_answer", req: dto.SendMessageRequest{Message: "tôi có bao nhiêu ví"},
			setup: func(t *testing.T, e *streamEnv) {
				e.ai.chatResp = chatResponse(t, "", toolCall(t, "c1", "list_wallets", nil))
				e.ai.streamChunks = []string{"<think>nội bộ</think>", "Bạn có ", "5 ví.", "<think>cuối"}
			}},
		{name: "tool_then_stream_error", req: dto.SendMessageRequest{Message: "nợ"},
			setup: func(t *testing.T, e *streamEnv) {
				e.debt.summary = &dto.DebtSummaryResponse{TotalPayable: 100}
				e.ai.chatResp = chatResponse(t, "", toolCall(t, "c1", "get_debt_summary", nil))
				e.ai.streamChunks = []string{"Một phần"}
				e.ai.streamErr = errors.New("upstream reset")
			}},
		{name: "direct_answer", req: dto.SendMessageRequest{Message: "hi"},
			setup: func(t *testing.T, e *streamEnv) {
				e.ai.chatResp = chatResponse(t, "<think>plan</think>Xin chào bạn nhé")
			}},
		{name: "empty_choice_streams", req: dto.SendMessageRequest{Message: "hi"},
			setup: func(t *testing.T, e *streamEnv) {
				e.ai.chatResp = chatResponse(t, "")
				e.ai.streamChunks = []string{"Đây là ", "<think>x</think>câu trả lời"}
			}},
		{name: "chat_error_falls_back_to_stream_error", req: dto.SendMessageRequest{Message: "hi"},
			setup: func(t *testing.T, e *streamEnv) {
				e.ai.chatErr = errors.New("POST http://10.0.0.5: 401")
				e.ai.streamChunks = []string{"Một phần"}
				e.ai.streamErr = errors.New("upstream reset")
			}},
		{name: "chat_error_stream_ok", req: dto.SendMessageRequest{Message: "hi"},
			setup: func(t *testing.T, e *streamEnv) {
				e.ai.chatErr = errors.New("boom")
				e.ai.streamChunks = []string{"ok"}
			}},
		{name: "caps_tool_calls_and_hides_errors", req: dto.SendMessageRequest{Message: "tổng quan"},
			setup: func(t *testing.T, e *streamEnv) {
				e.report.summaryErr = errors.New("pq: connection refused")
				var calls []ToolCall
				for i := 0; i < maxToolCallsPerTurn+2; i++ {
					calls = append(calls, toolCall(t, fmt.Sprintf("c%d", i), "get_financial_overview", map[string]interface{}{"month": "2026-08"}))
				}
				e.ai.chatResp = chatResponse(t, "", calls...)
				e.ai.streamChunks = []string{"ok"}
			}},
		{name: "unknown_tool_and_business_error", req: dto.SendMessageRequest{Message: "x"},
			setup: func(t *testing.T, e *streamEnv) {
				e.debt.getErr = fmt.Errorf("debt %w", constant.ErrNotFound)
				e.ai.chatResp = chatResponse(t, "",
					toolCall(t, "c1", "no_such_tool", nil),
					toolCall(t, "c2", "get_debt_detail", map[string]interface{}{"debtTitle": "Nam"}),
					toolCall(t, "c3", "record_debt_repayment", map[string]interface{}{"debtTitle": "Nam", "amount": 1.0}),
				)
			}},
		{name: "proposal_errors_and_valid_proposals", req: dto.SendMessageRequest{Message: "đặt ngân sách"},
			setup: func(t *testing.T, e *streamEnv) {
				e.addCategory("Ăn uống", model.CategoryTypeExpense)
				e.debt.debts = []dto.DebtResponse{{ID: debtID, Title: "Vay Nam", Type: model.DebtTypePayable, TotalAmount: 1_000_000, Remaining: 1_000_000, Status: model.DebtStatusPending}}
				e.ai.chatResp = chatResponse(t, "",
					toolCall(t, "c1", "set_budget", map[string]interface{}{"categoryName": "Ăn uống", "amount": 3_000_000.0, "periodStart": "2026-11-01"}),
					toolCall(t, "c2", "set_budget", map[string]interface{}{"categoryName": "Du lịch", "amount": 1.0}),
					toolCall(t, "c3", "record_debt_repayment", map[string]interface{}{"debtTitle": "Nam", "amount": 100_000.0, "walletName": "techcombank visa"}),
					toolCall(t, "c4", "create_debt", map[string]interface{}{"title": "Cho Lan vay", "totalAmount": 2_000_000.0, "type": "RECEIVABLE", "dueDate": "2026-12-31"}),
				)
				e.ai.streamChunks = []string{"Bạn kiểm tra nhé"}
			}},
		{name: "auto_inject_knowledge", req: dto.SendMessageRequest{Message: "nên chia lương thế nào"},
			setup: func(t *testing.T, e *streamEnv) {
				e.rag.results = []KnowledgeSearchResult{
					{Topic: "budget", Title: "Quy tắc 50/30/20", Content: "Chia thu nhập 50/30/20", Score: 0.82},
					{Topic: "weak", Title: "Yếu", Content: "KHÔNG_DÙNG", Score: 0.1},
				}
				e.ai.chatResp = chatResponse(t, "Trả lời")
			}},
		{name: "knowledge_tool_and_model_choice", req: dto.SendMessageRequest{Message: "quỹ khẩn cấp"},
			setup: func(t *testing.T, e *streamEnv) {
				e.rag.results = []KnowledgeSearchResult{{Topic: "fb", Title: "Quỹ", Content: "6 tháng", Score: 0.1, Fallback: true}}
				e.ai.chatResp = chatResponse(t, "", toolCall(t, "c1", "search_financial_knowledge", map[string]interface{}{"query": "quỹ khẩn cấp"}))
				e.ai.streamChunks = []string{"Nên có 6 tháng"}
			}},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			e := newStreamEnv(t)
			// Deterministic wallets (names only matter; ids are normalized).
			if sc.setup != nil {
				sc.setup(t, e)
			}
			req := sc.req
			if sc.name == "knowledge_tool_and_model_choice" {
				m := "gpt-x"
				req.Model = &m
			}
			events, err := e.send(t, req)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			assertGolden(t, "stream_"+sc.name, map[string]interface{}{
				"err":            errStr,
				"events":         events,
				"chatModel":      e.ai.chatModel,
				"chatMessages":   e.ai.chatMessages,
				"chatTools":      toolNamesOf(e.ai.chatTools),
				"streamCalls":    e.ai.streamCalls,
				"streamModel":    e.ai.streamModel,
				"streamMessages": e.ai.streamMessages,
				"stored":         storedMessages(e.repo),
				"txnCreated":     e.txnSvc.created,
			})
		})
	}
}

// writeToolCase is one write tool call: its preview, then (when a proposal
// was made) the confirm result and every side effect recorded by the fakes.
type writeToolCase struct {
	name  string
	tool  string
	args  map[string]interface{}
	setup func(e *toolEnv)
	// between runs after the preview and before the confirm (data changed meanwhile).
	between func(e *toolEnv)
}

func TestCharacterization_WriteTools(t *testing.T) {
	debtID := uuid.New()
	recvID := uuid.New()
	paidID := uuid.New()
	debts := func() []dto.DebtResponse {
		return []dto.DebtResponse{
			{ID: debtID, Title: "Vay anh Nam", Type: model.DebtTypePayable, TotalAmount: 1_000_000, Remaining: 600_000, PaidAmount: 400_000, Status: model.DebtStatusPending},
			{ID: recvID, Title: "Cho Lan vay", Type: model.DebtTypeReceivable, TotalAmount: 500_000, Remaining: 500_000, Status: model.DebtStatusPending},
			{ID: paidID, Title: "Mua laptop", Type: model.DebtTypePayable, TotalAmount: 9, Remaining: 0, PaidAmount: 9, Status: model.DebtStatusCompleted},
		}
	}
	withCats := func(e *toolEnv) {
		e.addCategory("Ăn uống", model.CategoryTypeExpense)
		e.addCategory("Cafe sáng", model.CategoryTypeExpense)
		e.addCategory("Lương", model.CategoryTypeIncome)
		e.addCategory("Chứng khoán", model.CategoryTypeInvestment)
	}
	withDebts := func(e *toolEnv) { e.debt.debts = debts() }

	cases := []writeToolCase{
		// create_transaction
		{name: "txn_expense_existing_category", tool: "create_transaction", setup: withCats,
			args: map[string]interface{}{"amount": 50_000.0, "type": "EXPENSE", "categoryName": "ăn uống", "walletName": "tiền mặt", "description": "Phở", "transactionDate": "2026-11-07"}},
		{name: "txn_income_lowercase_type_new_category", tool: "create_transaction", setup: withCats,
			args: map[string]interface{}{"amount": "25000000", "type": "income", "categoryName": " Thưởng Tết ", "walletName": "techcombank"}},
		{name: "txn_partial_category_default_date", tool: "create_transaction", setup: withCats,
			args: map[string]interface{}{"amount": 35_000.0, "categoryName": "cafe", "walletName": "ví điện tử", "transactionDate": "07/11/2026"}},
		{name: "txn_no_category_uses_first", tool: "create_transaction", setup: withCats,
			args: map[string]interface{}{"amount": 1.0, "walletName": "ngân hàng"}},
		{name: "txn_no_categories_creates_khac", tool: "create_transaction",
			args: map[string]interface{}{"amount": 1.0, "walletName": "ngân hàng", "type": "INCOME"}},
		{name: "txn_no_wallets", tool: "create_transaction", setup: func(e *toolEnv) { e.wallets.wallets = nil },
			args: map[string]interface{}{"amount": 1.0, "categoryName": "X"}},
		{name: "txn_bad_amount", tool: "create_transaction", args: map[string]interface{}{"amount": -5.0}},
		{name: "txn_ambiguous_wallet", tool: "create_transaction", args: map[string]interface{}{"amount": 5.0, "walletName": "techcombank v"}},
		{name: "txn_ambiguous_wallet_partial", tool: "create_transaction", args: map[string]interface{}{"amount": 5.0, "walletName": "ví"}},
		{name: "txn_missing_wallet", tool: "create_transaction", args: map[string]interface{}{"amount": 5.0}},
		{name: "txn_service_error", tool: "create_transaction", setup: withCats,
			args: map[string]interface{}{"amount": 5.0, "walletName": "tiền mặt", "categoryName": "Ăn uống"}},
		// create_category
		{name: "cat_new_expense", tool: "create_category", setup: withCats,
			args: map[string]interface{}{"name": " Thú cưng ", "description": "  chó mèo  "}},
		{name: "cat_new_investment_lower", tool: "create_category", args: map[string]interface{}{"name": "Vàng", "type": "investment"}},
		{name: "cat_unknown_type_defaults_expense", tool: "create_category", args: map[string]interface{}{"name": "Khác", "type": "weird"}},
		{name: "cat_duplicate", tool: "create_category", setup: withCats, args: map[string]interface{}{"name": "LƯƠNG", "type": "INCOME"}},
		{name: "cat_empty_name", tool: "create_category", args: map[string]interface{}{"name": "  "}},
		{name: "cat_created_meanwhile", tool: "create_category", args: map[string]interface{}{"name": "Du lịch"},
			between: func(e *toolEnv) { e.addCategory("du lịch", model.CategoryTypeExpense) }},
		{name: "cat_create_error", tool: "create_category", args: map[string]interface{}{"name": "Lỗi"},
			between: func(e *toolEnv) { e.cats.createErr = fmt.Errorf("category %w", constant.ErrAlreadyExists) }},
		// transfer_between_wallets
		{name: "transfer_ok_with_fee", tool: "transfer_between_wallets",
			args: map[string]interface{}{"amount": 1_000_000.0, "fee": "3300", "fromWalletName": "ngân hàng", "toWalletName": "tiền mặt", "notes": "rút"}},
		{name: "transfer_ok_no_fee", tool: "transfer_between_wallets",
			args: map[string]interface{}{"amount": 1.0, "fromWalletName": "techcombank", "toWalletName": "ví điện tử"}},
		{name: "transfer_bad_amount", tool: "transfer_between_wallets", args: map[string]interface{}{"amount": 0.0}},
		{name: "transfer_bad_fee", tool: "transfer_between_wallets", args: map[string]interface{}{"amount": 1.0, "fee": -1.0}},
		{name: "transfer_missing_to", tool: "transfer_between_wallets", args: map[string]interface{}{"amount": 1.0, "fromWalletName": "tiền mặt"}},
		{name: "transfer_unknown_from", tool: "transfer_between_wallets", args: map[string]interface{}{"amount": 1.0, "fromWalletName": "MoMo", "toWalletName": "tiền mặt"}},
		{name: "transfer_unknown_to", tool: "transfer_between_wallets", args: map[string]interface{}{"amount": 1.0, "fromWalletName": "tiền mặt", "toWalletName": "MoMo"}},
		{name: "transfer_same_wallet", tool: "transfer_between_wallets", args: map[string]interface{}{"amount": 1.0, "fromWalletName": "tiền mặt", "toWalletName": "ví tiền mặt"}},
		{name: "transfer_no_wallets", tool: "transfer_between_wallets", setup: func(e *toolEnv) { e.wallets.wallets = nil },
			args: map[string]interface{}{"amount": 1.0, "fromWalletName": "a", "toWalletName": "b"}},
		{name: "transfer_insufficient_balance", tool: "transfer_between_wallets",
			args: map[string]interface{}{"amount": 1.0, "fromWalletName": "tiền mặt", "toWalletName": "ngân hàng"},
			between: func(e *toolEnv) {
				e.wallet.transferErr = fmt.Errorf("%w: số dư không đủ", constant.ErrInsufficientBalance)
			}},
		{name: "transfer_internal_error", tool: "transfer_between_wallets",
			args:    map[string]interface{}{"amount": 1.0, "fromWalletName": "tiền mặt", "toWalletName": "ngân hàng"},
			between: func(e *toolEnv) { e.wallet.transferErr = errors.New("pq: deadlock") }},
		// create_debt
		{name: "debt_receivable_with_due", tool: "create_debt",
			args: map[string]interface{}{"title": "Cho Lan vay", "totalAmount": 2_000_000.0, "type": "receivable", "dueDate": "2026-12-31", "notes": "trả cuối năm"}},
		{name: "debt_payable_bad_due", tool: "create_debt",
			args: map[string]interface{}{"title": "Vay Nam", "totalAmount": "500", "dueDate": "soon"}},
		{name: "debt_payable_vn_due", tool: "create_debt",
			args: map[string]interface{}{"title": "Vay Nam", "totalAmount": 500.0, "type": "PAYABLE", "dueDate": "31/12/2026"}},
		{name: "debt_missing_title", tool: "create_debt", args: map[string]interface{}{"title": " ", "totalAmount": 1.0}},
		{name: "debt_create_business_error", tool: "create_debt", args: map[string]interface{}{"title": "x", "totalAmount": 1.0},
			between: func(e *toolEnv) {
				e.debt.createErr = fmt.Errorf("%w: hạn trả không hợp lệ", constant.ErrInvalidInput)
			}},
		// record_debt_repayment
		{name: "repay_with_wallet", tool: "record_debt_repayment", setup: withDebts,
			args: map[string]interface{}{"debtTitle": "anh nam", "amount": 400_000.0, "notes": "lần 1", "walletName": "tiền mặt"}},
		{name: "repay_receivable_no_wallet", tool: "record_debt_repayment", setup: withDebts,
			args: map[string]interface{}{"debtTitle": "Lan", "amount": "50000"}},
		{name: "repay_exceeds_remaining", tool: "record_debt_repayment", setup: withDebts,
			args: map[string]interface{}{"debtTitle": "Nam", "amount": 600_001.0}},
		{name: "repay_completed_debt", tool: "record_debt_repayment", setup: withDebts,
			args: map[string]interface{}{"debtTitle": "laptop", "amount": 1.0}},
		{name: "repay_unknown_debt", tool: "record_debt_repayment", setup: withDebts,
			args: map[string]interface{}{"debtTitle": "Hùng", "amount": 1.0}},
		{name: "repay_no_debts", tool: "record_debt_repayment", args: map[string]interface{}{"debtTitle": "Hùng", "amount": 1.0}},
		{name: "repay_bad_args", tool: "record_debt_repayment", args: map[string]interface{}{"debtTitle": "Nam", "amount": 0.0}},
		{name: "repay_ambiguous_wallet", tool: "record_debt_repayment", setup: withDebts,
			args: map[string]interface{}{"debtTitle": "Nam", "amount": 1.0, "walletName": "techcombank v"}},
		{name: "repay_wallet_but_no_wallets", tool: "record_debt_repayment",
			setup: func(e *toolEnv) { withDebts(e); e.wallets.wallets = nil },
			args:  map[string]interface{}{"debtTitle": "Nam", "amount": 1.0, "walletName": "tiền mặt"}},
		{name: "repay_wallet_deleted_meanwhile", tool: "record_debt_repayment", setup: withDebts,
			args:    map[string]interface{}{"debtTitle": "Nam", "amount": 1.0, "walletName": "tiền mặt"},
			between: func(e *toolEnv) { e.wallets.wallets = e.wallets.wallets[1:] }},
		{name: "repay_service_rejects", tool: "record_debt_repayment", setup: withDebts,
			args: map[string]interface{}{"debtTitle": "Nam", "amount": 1.0},
			between: func(e *toolEnv) {
				e.debt.repayErr = fmt.Errorf("%w: vượt quá số còn lại", constant.ErrInvalidInput)
			}},
		// set_budget
		{name: "budget_ok_default_period", tool: "set_budget", setup: withCats,
			args: map[string]interface{}{"categoryName": "ăn uống", "amount": 2_000_000.0}},
		{name: "budget_ok_explicit_period", tool: "set_budget", setup: withCats,
			args: map[string]interface{}{"categoryName": "Cafe", "amount": "300000", "periodStart": "2027-01-15"}},
		{name: "budget_unknown_category", tool: "set_budget", setup: withCats,
			args: map[string]interface{}{"categoryName": "Du lịch", "amount": 1.0}},
		{name: "budget_no_categories", tool: "set_budget", args: map[string]interface{}{"categoryName": "Du lịch", "amount": 1.0}},
		{name: "budget_bad_args", tool: "set_budget", args: map[string]interface{}{"categoryName": "", "amount": 1.0}},
		{name: "budget_service_conflict", tool: "set_budget", setup: withCats,
			args:    map[string]interface{}{"categoryName": "Ăn uống", "amount": 1.0},
			between: func(e *toolEnv) { e.budget.createErr = fmt.Errorf("budget %w", constant.ErrAlreadyExists) }},
		// set_financial_target
		{name: "target_savings_alias", tool: "set_financial_target",
			args: map[string]interface{}{"targetType": "savings", "targetAmount": 5_000_000.0, "month": 12.0, "year": 2026.0}},
		{name: "target_spending_default_month", tool: "set_financial_target",
			args: map[string]interface{}{"targetType": "SPENDING_LIMIT", "targetAmount": "3000000"}},
		{name: "target_bad_month", tool: "set_financial_target",
			args: map[string]interface{}{"targetType": "EXPENSE", "targetAmount": 1.0, "month": 13.0}},
		{name: "target_bad_type", tool: "set_financial_target", args: map[string]interface{}{"targetType": "LOTTERY", "targetAmount": 1.0}},
		{name: "target_missing_amount", tool: "set_financial_target", args: map[string]interface{}{"targetType": "EXPENSE"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newToolEnv(t)
			if tc.setup != nil {
				tc.setup(e)
			}
			if tc.name == "txn_service_error" {
				// The fake transaction service never fails; swap in one that does.
				e.svc.transactionService = &failingTxnSvc{tfTxnSvc: e.txnSvc, err: fmt.Errorf("%w: danh mục không hợp lệ", constant.ErrInvalidInput)}
			}
			ctx := context.Background()
			sessionID, messageID := uuid.New(), uuid.New()
			preview, err := e.svc.proposeWriteTool(ctx, e.userID, sessionID, messageID, ToolCall{
				ID: "call-1", Type: "function", Function: ToolCallFunction{Name: tc.tool, Arguments: mustJSON(t, tc.args)},
			})
			record := map[string]interface{}{"preview": preview}
			if err != nil {
				record["previewErr"] = err.Error()
			}
			if preview != nil && preview.ActionCard != nil && preview.ActionCard.ActionType == ActionTypePendingConfirmation {
				if tc.between != nil {
					tc.between(e)
				}
				actionID := uuid.MustParse(preview.ActionCard.Data.(map[string]interface{})["actionId"].(string))
				res, confirmErr := e.svc.ConfirmAction(ctx, e.userID, actionID)
				record["confirm"] = res
				if confirmErr != nil {
					record["confirmErr"] = confirmErr.Error()
					record["confirmErrIsFailed"] = errors.Is(confirmErr, ErrPendingActionFailed)
				}
			}
			record["sideEffects"] = map[string]interface{}{
				"transactions":    e.txnSvc.created,
				"categories":      e.cats.created,
				"transfers":       e.wallet.transfers,
				"debts":           e.debt.created,
				"repayments":      repaymentRecords(e.debt.repaid),
				"budgets":         e.budget.created,
				"targets":         e.target.upserts,
				"categoryListTyp": e.cats.lastType,
			}
			assertGolden(t, "write_"+tc.name, record)
		})
	}
}

func repaymentRecords(calls []repaymentCall) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(calls))
	for _, c := range calls {
		out = append(out, map[string]interface{}{"debtId": c.debtID, "req": c.req})
	}
	return out
}

// failingTxnSvc is a transaction service whose CreateTransaction fails.
type failingTxnSvc struct {
	*tfTxnSvc
	err error
}

func (s *failingTxnSvc) CreateTransaction(context.Context, uuid.UUID, dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
	return nil, s.err
}
