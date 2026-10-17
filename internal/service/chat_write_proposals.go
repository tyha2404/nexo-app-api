package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

// ActionTypePendingConfirmation is the action card of an AI write proposal.
const ActionTypePendingConfirmation = "PENDING_CONFIRMATION"

// pendingToolResultMessage is what the model sees instead of a success result.
const pendingToolResultMessage = "Đã gửi đề xuất, chờ người dùng xác nhận"

// chatWriteTools are the AI tools that change user data. They never execute
// from the chat stream: the server validates the arguments, stores a pending
// action and the user confirms it via POST /chat/actions/{id}/confirm.
var chatWriteTools = map[string]bool{
	"create_transaction":       true,
	"create_category":          true,
	"transfer_between_wallets": true,
	"create_debt":              true,
	"record_debt_repayment":    true,
	"set_budget":               true,
	"set_financial_target":     true,
}

// IsChatWriteTool reports whether the tool needs user confirmation.
func IsChatWriteTool(name string) bool { return chatWriteTools[name] }

// writeProposal is the validated preview of a write tool call.
type writeProposal struct {
	args        map[string]interface{} // normalized arguments executed on confirm
	kind        string                 // e.g. "Ghi chi tiêu"
	description string
	data        map[string]interface{}
}

func proposalError(title, msg string) *FinancialToolResult {
	return &FinancialToolResult{ToolTitle: title, ResultJSON: toolErrorJSON(msg)}
}

// proposeWriteTool validates a write tool call without executing it. It returns
// either a PENDING_CONFIRMATION result (and stores the pending action) or the
// same validation error result the tool would have produced.
func (s *chatService) proposeWriteTool(ctx context.Context, userID, sessionID, messageID uuid.UUID, call ToolCall) (*FinancialToolResult, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil || args == nil {
		args = map[string]interface{}{}
	}

	var (
		p      *writeProposal
		errRes *FinancialToolResult
		err    error
	)
	switch call.Function.Name {
	case "create_transaction":
		p, errRes, err = s.previewCreateTransaction(ctx, userID, args)
	case "create_category":
		p, errRes, err = s.previewCreateCategory(ctx, userID, args)
	case "transfer_between_wallets":
		p, errRes, err = s.previewTransfer(ctx, userID, args)
	case "create_debt":
		p, errRes = previewCreateDebt(args)
	case "record_debt_repayment":
		p, errRes, err = s.previewDebtRepayment(ctx, userID, args)
	case "set_budget":
		p, errRes, err = s.previewSetBudget(ctx, userID, args)
	case "set_financial_target":
		p, errRes = previewSetTarget(args)
	default:
		return nil, fmt.Errorf("tool %s is not a write tool", call.Function.Name)
	}
	if err != nil || errRes != nil {
		return errRes, err
	}

	now := s.now()
	action := PendingAction{
		ID:        uuid.New(),
		UserID:    userID,
		SessionID: sessionID,
		MessageID: messageID,
		ToolName:  call.Function.Name,
		Args:      p.args,
		Status:    PendingActionPending,
		CreatedAt: now,
		ExpiresAt: now.Add(PendingActionTTL),
	}
	s.pendingActions.Save(action)

	data := p.data
	data["actionId"] = action.ID.String()
	data["toolName"] = action.ToolName
	data["kind"] = p.kind
	data["status"] = string(PendingActionPending)
	data["expiresAt"] = action.ExpiresAt.UTC().Format(time.RFC3339)

	card := &dto.ActionCard{
		ActionType:  ActionTypePendingConfirmation,
		Title:       "Xác nhận: " + p.kind,
		Description: p.description,
		Data:        data,
	}
	result, _ := json.Marshal(map[string]interface{}{
		"status":   "pending_confirmation",
		"actionId": action.ID.String(),
		"summary":  p.kind + " — " + p.description,
		"message":  pendingToolResultMessage + ". Chưa có dữ liệu nào được lưu; hãy nhắc người dùng kiểm tra và bấm \"Xác nhận\" trên thẻ đề xuất.",
	})
	return &FinancialToolResult{ToolTitle: "Đang chuẩn bị đề xuất...", ResultJSON: string(result), ActionCard: card}, nil
}

func parseToolDate(raw string, fallback time.Time) time.Time {
	if raw != "" {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			return t
		}
	}
	return fallback
}

func (s *chatService) previewCreateTransaction(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*writeProposal, *FinancialToolResult, error) {
	const title = "Đang ghi nhận giao dịch mới..."
	amount, ok := toolPositiveAmount(args, "amount")
	if !ok {
		return nil, proposalError(title, "Số tiền giao dịch (amount) phải là số lớn hơn 0"), nil
	}
	txnType := "EXPENSE"
	if t, ok := args["type"].(string); ok && strings.ToUpper(t) == "INCOME" {
		txnType = "INCOME"
	}
	categoryName, _ := args["categoryName"].(string)
	walletName, _ := args["walletName"].(string)
	description, _ := args["description"].(string)
	dateStr, _ := args["transactionDate"].(string)
	txnDate := parseToolDate(dateStr, util.StartOfDay(s.now()))

	wallets, err := s.walletRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	resolvedWallet := ""
	if len(wallets) > 0 {
		matched, matchErr := matchWalletByName(wallets, walletName)
		if matchErr != nil {
			return nil, proposalError("Cần xác nhận ví giao dịch", matchErr.Error()), nil
		}
		resolvedWallet = matched.Name
	}

	categories, err := s.categoryRepo.List(ctx, userID, txnType, 100, 0)
	if err != nil {
		return nil, nil, err
	}
	resolvedCategory, isNew := "", false
	if c := matchCategoryByName(categories, categoryName); c != nil {
		resolvedCategory = c.Name
	} else if strings.TrimSpace(categoryName) != "" {
		resolvedCategory, isNew = strings.TrimSpace(categoryName), true
	} else if len(categories) > 0 {
		resolvedCategory = categories[0].Name
	} else {
		resolvedCategory, isNew = "Khác", true
	}
	if strings.TrimSpace(description) == "" {
		description = resolvedCategory
	}

	kind := "Ghi chi tiêu"
	if txnType == "INCOME" {
		kind = "Ghi thu nhập"
	}
	date := txnDate.Format("2006-01-02")
	desc := fmt.Sprintf("%s ₫ - %s (%s)", formatVND(amount), description, resolvedCategory)
	if isNew {
		desc = fmt.Sprintf("%s ₫ - %s (Tạo mới danh mục: %s)", formatVND(amount), description, resolvedCategory)
	}
	return &writeProposal{
		args: map[string]interface{}{
			"amount": amount, "type": txnType, "categoryName": resolvedCategory,
			"walletName": resolvedWallet, "description": description, "transactionDate": date,
		},
		kind:        kind,
		description: desc,
		data: map[string]interface{}{
			"amount": amount, "type": txnType, "categoryName": resolvedCategory, "walletName": resolvedWallet,
			"description": description, "date": date, "isNewCategoryCreated": isNew,
		},
	}, nil, nil
}

func normalizeCategoryType(raw interface{}) string {
	if t, ok := raw.(string); ok {
		switch upper := strings.ToUpper(strings.TrimSpace(t)); upper {
		case "INCOME", "INVESTMENT", "EXPENSE":
			return upper
		}
	}
	return "EXPENSE"
}

func categoryTypeVi(t string) string {
	switch t {
	case string(model.CategoryTypeIncome):
		return "Thu nhập"
	case string(model.CategoryTypeInvestment):
		return "Đầu tư"
	default:
		return "Chi tiêu"
	}
}

func (s *chatService) previewCreateCategory(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*writeProposal, *FinancialToolResult, error) {
	name, _ := args["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, proposalError("Tạo danh mục mới", "Tên danh mục (name) không được để trống"), nil
	}
	catType := normalizeCategoryType(args["type"])
	existing, err := s.categoryRepo.List(ctx, userID, catType, 200, 0)
	if err == nil {
		for _, c := range existing {
			if strings.EqualFold(c.Name, name) {
				// Nothing to write: report the existing category right away.
				res, toolErr := s.toolCreateCategory(ctx, userID, args)
				return nil, res, toolErr
			}
		}
	}
	description, _ := args["description"].(string)
	description = strings.TrimSpace(description)
	return &writeProposal{
		args:        map[string]interface{}{"name": name, "type": catType, "description": description},
		kind:        "Tạo danh mục",
		description: fmt.Sprintf("Danh mục: %s (%s)", name, categoryTypeVi(catType)),
		data:        map[string]interface{}{"name": name, "type": catType, "categoryName": name, "description": description},
	}, nil, nil
}

func (s *chatService) previewTransfer(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*writeProposal, *FinancialToolResult, error) {
	const errTitle = "Lỗi tham số chuyển tiền"
	amount, ok := toolPositiveAmount(args, "amount")
	if !ok {
		return nil, proposalError(errTitle, "Số tiền chuyển phải là số lớn hơn 0"), nil
	}
	fee, ok := toolNonNegativeAmount(args, "fee")
	if !ok {
		return nil, proposalError(errTitle, "Phí chuyển khoản (fee) phải là số không âm"), nil
	}
	fromName, _ := args["fromWalletName"].(string)
	toName, _ := args["toWalletName"].(string)
	notes, _ := args["notes"].(string)

	wallets, err := s.walletRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if len(wallets) == 0 {
		return nil, proposalError("Chuyển tiền nội bộ", "Bạn chưa có ví nào để chuyển tiền."), nil
	}
	if strings.TrimSpace(fromName) == "" || strings.TrimSpace(toName) == "" {
		return nil, proposalError("Chuyển tiền nội bộ", fmt.Sprintf("Cần nêu rõ cả ví nguồn và ví đích. Các ví hiện có: %s.", strings.Join(walletNames(wallets), ", "))), nil
	}
	from, fromErr := matchWalletByName(wallets, fromName)
	if fromErr != nil {
		return nil, proposalError("Chuyển tiền nội bộ", "Ví nguồn: "+fromErr.Error()), nil
	}
	to, toErr := matchWalletByName(wallets, toName)
	if toErr != nil {
		return nil, proposalError("Chuyển tiền nội bộ", "Ví đích: "+toErr.Error()), nil
	}
	if from.ID == to.ID {
		return nil, proposalError("Chuyển tiền nội bộ", "Ví nguồn và ví đích phải khác nhau"), nil
	}
	desc := fmt.Sprintf("Chuyển %s ₫ từ ví %s sang ví %s", formatVND(amount), from.Name, to.Name)
	if fee > 0 {
		desc += fmt.Sprintf(" (phí %s ₫)", formatVND(fee))
	}
	return &writeProposal{
		args: map[string]interface{}{
			"amount": amount, "fee": fee, "fromWalletName": from.Name, "toWalletName": to.Name, "notes": notes,
		},
		kind:        "Chuyển tiền giữa ví",
		description: desc,
		data: map[string]interface{}{
			"amount": amount, "fee": fee, "walletName": from.Name, "fromWalletName": from.Name,
			"toWalletName": to.Name, "description": notes,
		},
	}, nil, nil
}

func previewCreateDebt(args map[string]interface{}) (*writeProposal, *FinancialToolResult) {
	title, _ := args["title"].(string)
	amount, amountOK := toolPositiveAmount(args, "totalAmount")
	typeStr, _ := args["type"].(string)
	notes, _ := args["notes"].(string)
	dueDateStr, _ := args["dueDate"].(string)
	if strings.TrimSpace(title) == "" || !amountOK {
		return nil, proposalError("Lỗi tham số khoản nợ", "Cần cung cấp tên khoản nợ và số tiền lớn hơn 0")
	}
	debtType, kind := model.DebtTypePayable, "Ghi khoản đi vay"
	if strings.ToUpper(typeStr) == "RECEIVABLE" {
		debtType, kind = model.DebtTypeReceivable, "Ghi khoản cho vay"
	}
	dueDate := ""
	if t, err := time.Parse("2006-01-02", dueDateStr); err == nil {
		dueDate = t.Format("2006-01-02")
	}
	desc := fmt.Sprintf("%s ₫ - %s", formatVND(amount), title)
	if dueDate != "" {
		desc += ", hạn " + dueDate
	}
	return &writeProposal{
		args: map[string]interface{}{
			"title": title, "totalAmount": amount, "type": string(debtType), "notes": notes, "dueDate": dueDate,
		},
		kind:        kind,
		description: desc,
		data: map[string]interface{}{
			"amount": amount, "debtType": string(debtType), "title": title, "dueDate": dueDate, "date": dueDate, "description": notes,
		},
	}, nil
}

func (s *chatService) previewDebtRepayment(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*writeProposal, *FinancialToolResult, error) {
	debtTitle, _ := args["debtTitle"].(string)
	amount, amountOK := toolPositiveAmount(args, "amount")
	notes, _ := args["notes"].(string)
	if strings.TrimSpace(debtTitle) == "" || !amountOK {
		return nil, proposalError("Lỗi tham số trả nợ", "Cần cung cấp tên khoản nợ và số tiền trả nợ lớn hơn 0"), nil
	}
	debts, err := s.debtService.GetDebts(ctx, userID, "", "")
	if err != nil {
		return nil, nil, err
	}
	if len(debts) == 0 {
		return nil, proposalError("Ghi nhận trả nợ", "Bạn không có khoản nợ nào trong hệ thống."), nil
	}
	debt := matchDebtByTitle(debts, debtTitle)
	if debt == nil {
		return nil, proposalError("Ghi nhận trả nợ", fmt.Sprintf("Không tìm thấy khoản nợ khớp với '%s'", debtTitle)), nil
	}
	kind := "Ghi trả nợ"
	if debt.Type == model.DebtTypeReceivable {
		kind = "Ghi thu nợ"
	}
	return &writeProposal{
		args:        map[string]interface{}{"debtTitle": debt.Title, "amount": amount, "notes": notes},
		kind:        kind,
		description: fmt.Sprintf("%s ₫ cho '%s' (còn lại hiện tại: %s ₫)", formatVND(amount), debt.Title, formatVND(debt.Remaining)),
		data: map[string]interface{}{
			"amount": amount, "title": debt.Title, "debtId": debt.ID.String(), "remaining": debt.Remaining, "description": notes,
		},
	}, nil, nil
}

func (s *chatService) previewSetBudget(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*writeProposal, *FinancialToolResult, error) {
	categoryName, _ := args["categoryName"].(string)
	amount, amountOK := toolPositiveAmount(args, "amount")
	periodStartStr, _ := args["periodStart"].(string)
	if strings.TrimSpace(categoryName) == "" || !amountOK {
		return nil, proposalError("Lỗi đặt hạn mức ngân sách", "Cần cung cấp tên danh mục và hạn mức ngân sách lớn hơn 0"), nil
	}
	categories, err := s.categoryRepo.List(ctx, userID, "", 100, 0)
	if err != nil {
		return nil, nil, err
	}
	category := matchCategoryByName(categories, categoryName)
	if category == nil {
		return nil, proposalError("Đặt ngân sách", fmt.Sprintf("Không tìm thấy danh mục '%s' trong hệ thống của bạn", categoryName)), nil
	}
	monthStart, _ := util.MonthRange(s.now())
	periodStart := parseToolDate(periodStartStr, monthStart).Format("2006-01-02")
	return &writeProposal{
		args:        map[string]interface{}{"categoryName": category.Name, "amount": amount, "periodStart": periodStart},
		kind:        "Đặt ngân sách",
		description: fmt.Sprintf("Hạn mức danh mục %s: %s ₫/tháng từ %s", category.Name, formatVND(amount), periodStart),
		data: map[string]interface{}{
			"amount": amount, "categoryName": category.Name, "date": periodStart,
		},
	}, nil, nil
}

func previewSetTarget(args map[string]interface{}) (*writeProposal, *FinancialToolResult) {
	targetType, _ := args["targetType"].(string)
	amount, amountOK := toolPositiveAmount(args, "targetAmount")
	if targetType == "" || !amountOK {
		return nil, proposalError("Lỗi thiết lập mục tiêu", "Cần cung cấp loại mục tiêu (EXPENSE/INVESTMENT) và số tiền lớn hơn 0")
	}
	switch strings.ToUpper(targetType) {
	case "SPENDING_LIMIT", "SPENDING":
		targetType = "EXPENSE"
	case "SAVINGS":
		targetType = "INVESTMENT"
	}
	now := util.Now()
	month, year := int(now.Month()), now.Year()
	if v, ok := args["month"].(float64); ok && v > 0 {
		month = int(v)
	}
	if v, ok := args["year"].(float64); ok && v > 0 {
		year = int(v)
	}
	typeVi := "Hạn mức chi tiêu tối đa"
	if targetType == "INVESTMENT" {
		typeVi = "Mục tiêu đầu tư / tiết kiệm"
	}
	return &writeProposal{
		args:        map[string]interface{}{"targetType": targetType, "targetAmount": amount, "month": float64(month), "year": float64(year)},
		kind:        "Đặt mục tiêu tài chính",
		description: fmt.Sprintf("%s Tháng %d/%d: %s ₫", typeVi, month, year, formatVND(amount)),
		data: map[string]interface{}{
			"amount": amount, "targetType": targetType, "month": month, "year": year,
		},
	}, nil
}

// toolResultError returns the "error" message of a tool result JSON, if any.
func toolResultError(resultJSON string) string {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(resultJSON), &payload); err != nil {
		return ""
	}
	msg, _ := payload["error"].(string)
	return msg
}

// isBusinessError reports errors caused by the request (not the server).
func isBusinessError(err error) bool {
	return errors.Is(err, constant.ErrInvalidInput) || errors.Is(err, constant.ErrInsufficientBalance) ||
		errors.Is(err, constant.ErrNotFound) || errors.Is(err, constant.ErrAlreadyExists)
}

// ConfirmAction executes a pending AI write proposal through the same tool
// code path the chat used before confirmation existed.
func (s *chatService) ConfirmAction(ctx context.Context, userID, actionID uuid.UUID) (*dto.ChatActionResult, error) {
	action, err := s.pendingActions.Begin(userID, actionID)
	if err != nil {
		return nil, err
	}
	argsJSON, _ := json.Marshal(action.Args)
	res, execErr := s.executeFinancialTool(ctx, userID, ToolCall{
		ID:       action.ID.String(),
		Type:     "function",
		Function: ToolCallFunction{Name: action.ToolName, Arguments: string(argsJSON)},
	})
	if execErr == nil && res != nil {
		if msg := toolResultError(res.ResultJSON); msg != "" {
			execErr = fmt.Errorf("%w: %s", ErrPendingActionFailed, msg)
		}
	}
	if execErr == nil && res == nil {
		execErr = fmt.Errorf("tool %s returned no result", action.ToolName)
	}
	if execErr != nil {
		s.pendingActions.Revert(action.ID)
		if isBusinessError(execErr) && !errors.Is(execErr, ErrPendingActionFailed) {
			execErr = fmt.Errorf("%w: %s", ErrPendingActionFailed, execErr.Error())
		}
		return nil, execErr
	}

	s.pendingActions.Complete(action.ID, res.ActionCard)
	s.persistActionOutcome(ctx, userID, action.MessageID)

	var result interface{}
	_ = json.Unmarshal([]byte(res.ResultJSON), &result)
	return &dto.ChatActionResult{
		ActionID:   action.ID,
		Status:     string(PendingActionConfirmed),
		ToolName:   action.ToolName,
		ActionCard: res.ActionCard,
		Result:     result,
	}, nil
}

// CancelAction discards a pending AI write proposal.
func (s *chatService) CancelAction(ctx context.Context, userID, actionID uuid.UUID) (*dto.ChatActionResult, error) {
	action, err := s.pendingActions.Cancel(userID, actionID)
	if err != nil {
		return nil, err
	}
	s.persistActionOutcome(ctx, userID, action.MessageID)
	return &dto.ChatActionResult{ActionID: action.ID, Status: string(PendingActionCancelled), ToolName: action.ToolName}, nil
}

// applyPendingStatuses rewrites the status (and result card) of every
// PENDING_CONFIRMATION card from the store, so stored history matches it.
func (s *chatService) applyPendingStatuses(userID uuid.UUID, cards []dto.ActionCard) {
	for i := range cards {
		if cards[i].ActionType != ActionTypePendingConfirmation {
			continue
		}
		data, ok := cards[i].Data.(map[string]interface{})
		if !ok {
			continue
		}
		idStr, _ := data["actionId"].(string)
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		action, err := s.pendingActions.Get(userID, id)
		if err != nil {
			continue
		}
		data["status"] = string(action.Status)
		if action.ResultCard != nil {
			data["resultCard"] = action.ResultCard
		}
	}
}

// persistActionCards stores the turn's action cards on the AI message, with
// pending statuses refreshed from the store.
func (s *chatService) persistActionCards(ctx context.Context, userID, messageID uuid.UUID, cards []dto.ActionCard) {
	if len(cards) == 0 {
		return
	}
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	s.applyPendingStatuses(userID, cards)
	raw, err := json.Marshal(cards)
	if err != nil {
		return
	}
	str := string(raw)
	if err := s.chatRepo.UpdateMessageToolResults(ctx, messageID, &str); err != nil {
		s.logger.Warn("failed to persist chat action cards", zap.Error(err))
	}
}

// persistActionOutcome refreshes the stored cards of a message after a
// confirm/cancel so a reloaded session shows "Đã xác nhận" / "Đã huỷ". When the
// stream has not stored the cards yet, it will store the current status itself.
func (s *chatService) persistActionOutcome(ctx context.Context, userID, messageID uuid.UUID) {
	if messageID == uuid.Nil {
		return
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	msg, err := s.chatRepo.GetMessageByID(persistCtx, messageID)
	if err != nil || msg == nil || msg.ToolResults == nil || *msg.ToolResults == "" {
		return
	}
	var cards []dto.ActionCard
	if err := json.Unmarshal([]byte(*msg.ToolResults), &cards); err != nil {
		return
	}
	s.applyPendingStatuses(userID, cards)
	raw, err := json.Marshal(cards)
	if err != nil {
		return
	}
	str := string(raw)
	if err := s.chatRepo.UpdateMessageToolResults(persistCtx, messageID, &str); err != nil {
		s.logger.Warn("failed to persist chat action outcome", zap.Error(err))
	}
}
