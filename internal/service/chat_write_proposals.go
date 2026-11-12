package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

// writeProposal is the validated preview of a write tool call.
type writeProposal struct {
	args        writeArgs // normalized arguments executed on confirm
	kind        string    // e.g. "Ghi chi tiêu"
	description string
	data        map[string]interface{}
}

func proposalError(title, msg string) *FinancialToolResult {
	return &FinancialToolResult{ToolTitle: title, ResultJSON: toolErrorJSON(msg)}
}

// proposeWriteTool validates a write tool call without executing it. It returns
// either a PENDING_CONFIRMATION result (and stores the pending action) or the
// validation error result shown to the model.
func (s *chatService) proposeWriteTool(ctx context.Context, userID, sessionID, messageID uuid.UUID, call ToolCall) (*FinancialToolResult, error) {
	tool := chatToolRegistry.lookup(call.Function.Name)
	if tool == nil || !tool.isWrite() {
		return nil, fmt.Errorf("tool %s is not a write tool", call.Function.Name)
	}
	p, errRes, err := tool.preview(s, ctx, userID, parseToolArgs(call.Function.Arguments))
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

// parseToolDate parses a YYYY-MM-DD tool argument, fallback when it is
// missing or invalid.
func parseToolDate(raw string, fallback time.Time) time.Time {
	if raw != "" {
		if t, err := util.ParseDate(raw); err == nil {
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
	txnType := normalizeTransactionType(args["type"])
	categoryName, _ := args["categoryName"].(string)
	walletName, _ := args["walletName"].(string)
	description, _ := args["description"].(string)
	dateStr, _ := args["transactionDate"].(string)
	txnDate := calendarDay(parseToolDate(dateStr, util.StartOfDay(s.now())))

	// Resolve the wallet before anything else; never guess one: a wrong
	// wallet silently corrupts balances, so ask the user instead.
	wallets, err := s.walletRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	var walletID *uuid.UUID
	resolvedWallet := ""
	if len(wallets) > 0 {
		matched, matchErr := matchWalletByName(wallets, walletName)
		if matchErr != nil {
			return nil, proposalError("Cần xác nhận ví giao dịch", matchErr.Error()), nil
		}
		id := matched.ID
		walletID, resolvedWallet = &id, matched.Name
	}

	// An existing category, else the named one (created on confirm), else
	// the first category of the type, else "Khác" (created on confirm).
	categories, err := s.categoryRepo.List(ctx, userID, txnType, toolCategoryListLimit, 0)
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
	if txnType == string(model.TransactionTypeIncome) {
		kind = "Ghi thu nhập"
	}
	date := txnDate.Format(time.DateOnly)
	desc := fmt.Sprintf("%s ₫ - %s (%s)", formatVND(amount), description, resolvedCategory)
	if isNew {
		desc = fmt.Sprintf("%s ₫ - %s (Tạo mới danh mục: %s)", formatVND(amount), description, resolvedCategory)
	}
	return &writeProposal{
		args: &createTransactionArgs{
			Amount: amount, Type: txnType, CategoryName: resolvedCategory,
			WalletID: walletID, Description: description, Date: txnDate,
		},
		kind:        kind,
		description: desc,
		data: map[string]interface{}{
			"amount": amount, "type": txnType, "categoryName": resolvedCategory, "walletName": resolvedWallet,
			"description": description, "date": date, "isNewCategoryCreated": isNew,
		},
	}, nil, nil
}

// normalizeCategoryType returns the upper-cased category type from a tool
// argument, EXPENSE when it is missing or unknown.
func normalizeCategoryType(raw interface{}) string {
	if t, ok := raw.(string); ok {
		switch upper := model.CategoryType(strings.ToUpper(strings.TrimSpace(t))); upper {
		case model.CategoryTypeIncome, model.CategoryTypeInvestment, model.CategoryTypeExpense:
			return string(upper)
		}
	}
	return string(model.CategoryTypeExpense)
}

// normalizeTransactionType returns INCOME when the tool argument says so
// (any case) and EXPENSE otherwise.
func normalizeTransactionType(raw interface{}) string {
	if t, ok := raw.(string); ok && strings.ToUpper(t) == string(model.TransactionTypeIncome) {
		return string(model.TransactionTypeIncome)
	}
	return string(model.TransactionTypeExpense)
}

// normalizeTargetType upper-cases a target type and maps the model's aliases
// (SPENDING_LIMIT/SPENDING → EXPENSE, SAVINGS → INVESTMENT). Other values are
// returned as-is for validateTarget to reject.
func normalizeTargetType(raw string) string {
	switch t := strings.ToUpper(raw); t {
	case "SPENDING_LIMIT", "SPENDING":
		return string(model.TargetTypeExpense)
	case "SAVINGS":
		return string(model.TargetTypeInvestment)
	default:
		return t
	}
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
	if existing := s.findCategoryNamed(ctx, userID, catType, name); existing != nil {
		// Nothing to write: report the existing category right away,
		// without going through any write path.
		return nil, existingCategoryResult(existing), nil
	}
	description, _ := args["description"].(string)
	description = strings.TrimSpace(description)
	return &writeProposal{
		args:        &createCategoryArgs{Name: name, Type: catType, Description: description},
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
		args:        &transferArgs{Amount: amount, Fee: fee, FromWalletID: from.ID, ToWalletID: to.ID, Notes: notes},
		kind:        "Chuyển tiền giữa ví",
		description: desc,
		data: map[string]interface{}{
			"amount": amount, "fee": fee, "walletName": from.Name, "fromWalletName": from.Name,
			"toWalletName": to.Name, "description": notes,
		},
	}, nil, nil
}

func (s *chatService) previewCreateDebt(_ context.Context, _ uuid.UUID, args map[string]interface{}) (*writeProposal, *FinancialToolResult, error) {
	title, _ := args["title"].(string)
	amount, amountOK := toolPositiveAmount(args, "totalAmount")
	typeStr, _ := args["type"].(string)
	notes, _ := args["notes"].(string)
	dueDateStr, _ := args["dueDate"].(string)
	if strings.TrimSpace(title) == "" || !amountOK {
		return nil, proposalError("Lỗi tham số khoản nợ", "Cần cung cấp tên khoản nợ và số tiền lớn hơn 0"), nil
	}
	debtType, kind := model.DebtTypePayable, "Ghi khoản đi vay"
	if strings.ToUpper(typeStr) == "RECEIVABLE" {
		debtType, kind = model.DebtTypeReceivable, "Ghi khoản cho vay"
	}
	var dueDate *time.Time
	dueDateLabel := ""
	if t, err := util.ParseDate(dueDateStr); err == nil {
		dueDate, dueDateLabel = &t, t.Format(time.DateOnly)
	}
	desc := fmt.Sprintf("%s ₫ - %s", formatVND(amount), title)
	if dueDateLabel != "" {
		desc += ", hạn " + dueDateLabel
	}
	return &writeProposal{
		args:        &createDebtArgs{Title: title, TotalAmount: amount, Type: debtType, DueDate: dueDate, Notes: notes},
		kind:        kind,
		description: desc,
		data: map[string]interface{}{
			"amount": amount, "debtType": string(debtType), "title": title, "dueDate": dueDateLabel, "date": dueDateLabel, "description": notes,
		},
	}, nil, nil
}

// resolveRepaymentWallet resolves the optional wallet name of a debt
// repayment. No name means the repayment moves no balance. msg is a
// user-facing error when the name cannot be resolved.
func (s *chatService) resolveRepaymentWallet(ctx context.Context, userID uuid.UUID, walletName string) (wallet *model.Wallet, msg string, err error) {
	if strings.TrimSpace(walletName) == "" {
		return nil, "", nil
	}
	wallets, err := s.walletRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if len(wallets) == 0 {
		return nil, fmt.Sprintf("Bạn chưa có ví nào nên không thể dùng ví '%s'. Có thể ghi nhận trả nợ mà không chọn ví.", walletName), nil
	}
	matched, matchErr := matchWalletByName(wallets, walletName)
	if matchErr != nil {
		return nil, matchErr.Error(), nil
	}
	return matched, "", nil
}

func (s *chatService) previewDebtRepayment(ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*writeProposal, *FinancialToolResult, error) {
	debtTitle, _ := args["debtTitle"].(string)
	amount, amountOK := toolPositiveAmount(args, "amount")
	notes, _ := args["notes"].(string)
	walletName, _ := args["walletName"].(string)
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
	// Reject what DebtService.AddRepayment would reject, so the user never
	// confirms a card that can only fail.
	if debt.Status == model.DebtStatusCompleted || debt.Remaining <= 0 {
		return nil, proposalError("Ghi nhận trả nợ", fmt.Sprintf("Khoản nợ '%s' đã được thanh toán đủ, không thể ghi nhận thêm.", debt.Title)), nil
	}
	if math.Round(amount*100) > math.Round(debt.Remaining*100) {
		return nil, proposalError("Ghi nhận trả nợ", fmt.Sprintf("Số tiền %s ₫ vượt quá số còn lại của khoản nợ '%s' (%s ₫).", formatVND(amount), debt.Title, formatVND(debt.Remaining))), nil
	}

	wallet, walletMsg, err := s.resolveRepaymentWallet(ctx, userID, walletName)
	if err != nil {
		return nil, nil, err
	}
	if walletMsg != "" {
		return nil, proposalError("Cần xác nhận ví thanh toán", walletMsg), nil
	}
	var walletID *uuid.UUID
	resolvedWalletName := ""
	if wallet != nil {
		id := wallet.ID
		walletID, resolvedWalletName = &id, wallet.Name
	}

	kind := "Ghi trả nợ"
	walletVerb := "trừ từ ví"
	if debt.Type == model.DebtTypeReceivable {
		kind = "Ghi thu nợ"
		walletVerb = "cộng vào ví"
	}
	desc := fmt.Sprintf("%s ₫ cho '%s' (còn lại hiện tại: %s ₫)", formatVND(amount), debt.Title, formatVND(debt.Remaining))
	if resolvedWalletName != "" {
		desc += fmt.Sprintf(", %s %s", walletVerb, resolvedWalletName)
	}
	return &writeProposal{
		args:        &debtRepaymentArgs{DebtID: debt.ID, Amount: amount, Notes: notes, WalletID: walletID},
		kind:        kind,
		description: desc,
		data: map[string]interface{}{
			"amount": amount, "title": debt.Title, "debtId": debt.ID.String(), "remaining": debt.Remaining, "description": notes,
			"walletName": resolvedWalletName,
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
	categories, err := s.categoryRepo.List(ctx, userID, "", toolCategoryListLimit, 0)
	if err != nil {
		return nil, nil, err
	}
	category := matchCategoryByName(categories, categoryName)
	if category == nil {
		return nil, proposalError("Đặt ngân sách", fmt.Sprintf("Không tìm thấy danh mục '%s' trong hệ thống của bạn", categoryName)), nil
	}
	monthStart, _ := util.MonthRange(s.now())
	periodStart := calendarDay(parseToolDate(periodStartStr, monthStart))
	label := periodStart.Format(time.DateOnly)
	return &writeProposal{
		args:        &setBudgetArgs{CategoryID: category.ID, CategoryName: category.Name, Amount: amount, PeriodStart: periodStart},
		kind:        "Đặt ngân sách",
		description: fmt.Sprintf("Hạn mức danh mục %s: %s ₫/tháng từ %s", category.Name, formatVND(amount), label),
		data: map[string]interface{}{
			"amount": amount, "categoryName": category.Name, "date": label,
		},
	}, nil, nil
}

func (s *chatService) previewSetTarget(_ context.Context, _ uuid.UUID, args map[string]interface{}) (*writeProposal, *FinancialToolResult, error) {
	targetType, _ := args["targetType"].(string)
	amount, amountOK := toolPositiveAmount(args, "targetAmount")
	if targetType == "" || !amountOK {
		return nil, proposalError("Lỗi thiết lập mục tiêu", "Cần cung cấp loại mục tiêu (EXPENSE/INVESTMENT) và số tiền lớn hơn 0"), nil
	}
	targetType = normalizeTargetType(targetType)
	now := s.now()
	month, year := int(now.Month()), now.Year()
	if v, ok := args["month"].(float64); ok && v > 0 {
		month = int(v)
	}
	if v, ok := args["year"].(float64); ok && v > 0 {
		year = int(v)
	}
	if err := validateTarget(targetType, amount, month, year); err != nil {
		return nil, proposalError("Lỗi thiết lập mục tiêu", err.Error()), nil
	}
	return &writeProposal{
		args:        &setTargetArgs{TargetType: targetType, Amount: amount, Month: month, Year: year},
		kind:        "Đặt mục tiêu tài chính",
		description: fmt.Sprintf("%s Tháng %d/%d: %s ₫", targetTypeVi(targetType), month, year, formatVND(amount)),
		data: map[string]interface{}{
			"amount": amount, "targetType": targetType, "month": month, "year": year,
		},
	}, nil, nil
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
		errors.Is(err, constant.ErrNotFound) || errors.Is(err, constant.ErrAlreadyExists) ||
		errors.Is(err, ErrSameWalletTransfer)
}

// toolErrorMessage is the error text shown to the model and the user. Only
// business errors are passed through; anything else (DB, driver, upstream)
// may carry internal details, so it is logged by the caller and replaced.
func toolErrorMessage(err error) string {
	if isBusinessError(err) {
		return err.Error()
	}
	return "Có lỗi hệ thống, vui lòng thử lại sau"
}

// ConfirmAction executes a pending AI write proposal: the typed arguments its
// preview validated.
func (s *chatService) ConfirmAction(ctx context.Context, userID, actionID uuid.UUID) (*dto.ChatActionResult, error) {
	action, err := s.pendingActions.Begin(userID, actionID)
	if err != nil {
		return nil, err
	}
	var (
		res     *FinancialToolResult
		execErr error
	)
	if action.Args == nil {
		execErr = fmt.Errorf("pending action %s has no arguments", action.ID)
	} else {
		res, execErr = action.Args.execute(ctx, s, userID)
	}
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
	s.storeCardsLocked(ctx, userID, messageID, cards, "failed to persist chat action cards")
}

// storeCardsLocked refreshes the pending statuses of cards and stores them as
// the message's tool results; a failure is only logged (as logMsg). The
// caller holds s.historyMu.
func (s *chatService) storeCardsLocked(ctx context.Context, userID, messageID uuid.UUID, cards []dto.ActionCard, logMsg string) {
	s.applyPendingStatuses(userID, cards)
	raw, err := json.Marshal(cards)
	if err != nil {
		return
	}
	str := string(raw)
	if err := s.chatRepo.UpdateMessageToolResults(ctx, userID, messageID, &str); err != nil {
		s.logger.Warn(logMsg, zap.Error(err))
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
	msg, err := s.chatRepo.GetMessageByID(persistCtx, userID, messageID)
	if err != nil || msg == nil || msg.ToolResults == nil || *msg.ToolResults == "" {
		return
	}
	var cards []dto.ActionCard
	if err := json.Unmarshal([]byte(*msg.ToolResults), &cards); err != nil {
		return
	}
	s.storeCardsLocked(persistCtx, userID, messageID, cards, "failed to persist chat action outcome")
}
