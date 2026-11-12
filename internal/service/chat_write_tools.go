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
	"go.uber.org/zap"
)

// writeArgs are the validated, normalized arguments of a write tool call,
// produced by the tool's preview and stored with the pending action. execute
// runs them once the user confirms. It does not parse or validate the model's
// arguments again; it only re-checks what may have changed since the preview
// (wallets/categories/debts deleted, balances, remaining debt — the services
// enforce the latter).
type writeArgs interface {
	execute(ctx context.Context, s *chatService, userID uuid.UUID) (*FinancialToolResult, error)
}

// calendarDay returns t's calendar date (in t's location) as UTC midnight,
// the value util.ParseDate returns for a YYYY-MM-DD string.
func calendarDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// staleWalletMessage is the error of a confirmed proposal whose wallet was
// deleted after the preview.
func staleWalletMessage(proposal string) string {
	return fmt.Sprintf("Ví đã chọn không còn tồn tại. Hãy yêu cầu trợ lý tạo lại đề xuất %s.", proposal)
}

// currentWallets returns the user's wallets with the given ids (nil for an
// id that no longer exists).
func (s *chatService) currentWallets(ctx context.Context, userID uuid.UUID, ids ...uuid.UUID) ([]*model.Wallet, error) {
	wallets, err := s.walletRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Wallet, len(ids))
	for i, id := range ids {
		for j := range wallets {
			if wallets[j].ID == id {
				out[i] = &wallets[j]
				break
			}
		}
	}
	return out, nil
}

// --- create_transaction ---

type createTransactionArgs struct {
	Amount float64
	Type   string // INCOME | EXPENSE
	// CategoryName is the matched category, or the one to create.
	CategoryName string
	WalletID     *uuid.UUID // nil: the user had no wallet
	Description  string
	Date         time.Time // calendar day (UTC midnight)
}

func (a *createTransactionArgs) execute(ctx context.Context, s *chatService, userID uuid.UUID) (*FinancialToolResult, error) {
	const title = "Đang ghi nhận giao dịch mới..."
	walletName := ""
	if a.WalletID != nil {
		found, err := s.currentWallets(ctx, userID, *a.WalletID)
		if err != nil {
			return nil, err
		}
		if found[0] == nil {
			return proposalError("Cần xác nhận ví giao dịch", staleWalletMessage("giao dịch")), nil
		}
		walletName = found[0].Name
	}

	// The category may have been created or deleted since the preview.
	categories, err := s.categoryRepo.List(ctx, userID, a.Type, toolCategoryListLimit, 0)
	if err != nil {
		return nil, err
	}
	category := matchCategoryByName(categories, a.CategoryName)
	isNew := false
	if category == nil {
		category = &model.Category{UserID: userID, Name: a.CategoryName, Type: model.CategoryType(a.Type)}
		if err := s.categoryRepo.Create(ctx, category); err != nil {
			return nil, err
		}
		isNew = true
	}

	description := a.Description
	created, err := s.transactionService.CreateTransaction(ctx, userID, dto.CreateTransactionRequest{
		CategoryID:      category.ID,
		WalletID:        a.WalletID,
		Amount:          a.Amount,
		Type:            a.Type,
		Description:     &description,
		TransactionDate: dto.CustomTime{Time: a.Date},
	})
	if err != nil {
		return nil, err
	}

	typeVi := "Chi tiêu"
	if a.Type == string(model.TransactionTypeIncome) {
		typeVi = "Thu nhập"
	}
	cardDesc := fmt.Sprintf("%s ₫ - %s (%s)", formatVND(a.Amount), description, category.Name)
	if isNew {
		cardDesc = fmt.Sprintf("%s ₫ - %s (Tạo mới danh mục: %s)", formatVND(a.Amount), description, category.Name)
	}
	date := a.Date.Format(time.DateOnly)
	result, _ := json.Marshal(map[string]interface{}{
		"success": true, "transactionId": created.ID, "amount": a.Amount, "type": a.Type,
		"category": category.Name, "wallet": walletName, "description": description, "date": date,
		"isNewCategoryCreated": isNew, "message": "Giao dịch đã được lưu thành công vào hệ thống Nexo.",
	})
	return &FinancialToolResult{
		ToolTitle:  title,
		ResultJSON: string(result),
		ActionCard: &dto.ActionCard{
			ActionType:  "TRANSACTION_CREATED",
			Title:       fmt.Sprintf("Đã ghi nhận %s", typeVi),
			Description: cardDesc,
			Data: map[string]interface{}{
				"id": created.ID, "amount": a.Amount, "type": a.Type, "category": category.Name,
				"wallet": walletName, "description": description, "date": date, "isNewCategoryCreated": isNew,
			},
		},
	}, nil
}

// --- create_category ---

type createCategoryArgs struct {
	Name        string
	Type        string // INCOME | EXPENSE | INVESTMENT
	Description string // trimmed, may be empty
}

// findCategoryNamed returns the user's category of that type whose name
// equals name (case-insensitive), or nil. A listing error counts as none.
func (s *chatService) findCategoryNamed(ctx context.Context, userID uuid.UUID, catType, name string) *model.Category {
	existing, err := s.categoryRepo.List(ctx, userID, catType, toolCategoryListLimit, 0)
	if err != nil {
		return nil
	}
	for i := range existing {
		if strings.EqualFold(existing[i].Name, name) {
			return &existing[i]
		}
	}
	return nil
}

// existingCategoryResult reports that the category to create already exists
// (nothing is written).
func existingCategoryResult(cat *model.Category) *FinancialToolResult {
	resBytes, _ := json.Marshal(map[string]interface{}{
		"success": true, "categoryId": cat.ID, "name": cat.Name, "type": string(cat.Type), "isDuplicate": true,
		"message": fmt.Sprintf("Danh mục \"%s\" đã tồn tại sẵn trong hệ thống.", cat.Name),
	})
	return &FinancialToolResult{
		ToolTitle:  "Đang kiểm tra danh mục...",
		ResultJSON: string(resBytes),
		ActionCard: &dto.ActionCard{
			ActionType:  "CATEGORY_CREATED",
			Title:       "Danh mục đã tồn tại",
			Description: fmt.Sprintf("Danh mục \"%s\" (%s) đã có trong hệ thống", cat.Name, categoryTypeVi(string(cat.Type))),
			Data:        map[string]interface{}{"id": cat.ID, "name": cat.Name, "type": string(cat.Type), "isDuplicate": true},
		},
	}
}

func (a *createCategoryArgs) execute(ctx context.Context, s *chatService, userID uuid.UUID) (*FinancialToolResult, error) {
	// It may have been created since the preview.
	if existing := s.findCategoryNamed(ctx, userID, a.Type, a.Name); existing != nil {
		return existingCategoryResult(existing), nil
	}
	cat := &model.Category{UserID: userID, Name: a.Name, Type: model.CategoryType(a.Type)}
	if a.Description != "" {
		desc := a.Description
		cat.Description = &desc
	}
	if err := s.categoryRepo.Create(ctx, cat); err != nil {
		s.logger.Error("failed to create category from AI tool", zap.Error(err))
		return proposalError("Tạo danh mục mới thất bại", "Không thể tạo danh mục: "+toolErrorMessage(err)), nil
	}

	typeVi := categoryTypeVi(string(cat.Type))
	resBytes, _ := json.Marshal(map[string]interface{}{
		"success": true, "categoryId": cat.ID, "name": cat.Name, "type": string(cat.Type), "description": a.Description,
		"message": fmt.Sprintf("Đã tạo thành công danh mục \"%s\" (%s).", cat.Name, typeVi),
	})
	return &FinancialToolResult{
		ToolTitle:  "Đang tạo danh mục mới...",
		ResultJSON: string(resBytes),
		ActionCard: &dto.ActionCard{
			ActionType:  "CATEGORY_CREATED",
			Title:       "Đã tạo danh mục mới",
			Description: fmt.Sprintf("Danh mục: %s (%s)", cat.Name, typeVi),
			Data:        map[string]interface{}{"id": cat.ID, "name": cat.Name, "type": string(cat.Type), "description": a.Description},
		},
	}, nil
}

// --- transfer_between_wallets ---

type transferArgs struct {
	Amount       float64
	Fee          float64
	FromWalletID uuid.UUID
	ToWalletID   uuid.UUID
	Notes        string
}

func (a *transferArgs) execute(ctx context.Context, s *chatService, userID uuid.UUID) (*FinancialToolResult, error) {
	found, err := s.currentWallets(ctx, userID, a.FromWalletID, a.ToWalletID)
	if err != nil {
		return nil, err
	}
	from, to := found[0], found[1]
	if from == nil || to == nil {
		return proposalError("Chuyển tiền nội bộ", staleWalletMessage("chuyển tiền")), nil
	}
	var note *string
	if a.Notes != "" {
		notes := a.Notes
		note = &notes
	}
	res, err := s.walletService.TransferMoney(ctx, userID, dto.TransferMoneyRequest{
		FromWalletID: from.ID, ToWalletID: to.ID, Amount: a.Amount, Fee: a.Fee, Note: note,
	})
	if err != nil {
		if !isBusinessError(err) {
			s.logger.Error("failed to transfer money from AI tool", zap.Error(err))
		}
		return proposalError("Chuyển tiền nội bộ thất bại", toolErrorMessage(err)), nil
	}
	resultBytes, _ := json.Marshal(res)
	return &FinancialToolResult{
		ToolTitle:  "Đang thực hiện chuyển tiền...",
		ResultJSON: string(resultBytes),
		ActionCard: &dto.ActionCard{
			ActionType:  "WALLET_TRANSFER",
			Title:       "Chuyển tiền thành công",
			Description: fmt.Sprintf("Đã chuyển %s ₫ từ ví %s sang ví %s", formatVND(a.Amount), from.Name, to.Name),
			Data:        res,
		},
	}, nil
}

// --- create_debt ---

type createDebtArgs struct {
	Title       string
	TotalAmount float64
	Type        model.DebtType
	DueDate     *time.Time // calendar day (UTC midnight)
	Notes       string
}

func (a *createDebtArgs) execute(ctx context.Context, s *chatService, userID uuid.UUID) (*FinancialToolResult, error) {
	req := dto.CreateDebtRequest{Type: a.Type, Title: a.Title, TotalAmount: a.TotalAmount, Notes: a.Notes}
	if a.DueDate != nil {
		req.DueDate = &dto.CustomTime{Time: *a.DueDate}
	}
	created, err := s.debtService.CreateDebt(ctx, userID, req)
	if err != nil {
		return nil, err
	}
	typeVi := "Khoản nợ phải trả"
	if a.Type == model.DebtTypeReceivable {
		typeVi = "Khoản cho vay (cần thu)"
	}
	resultBytes, _ := json.Marshal(created)
	return &FinancialToolResult{
		ToolTitle:  "Đang ghi nhận khoản nợ...",
		ResultJSON: string(resultBytes),
		ActionCard: &dto.ActionCard{
			ActionType:  "DEBT_CREATED",
			Title:       fmt.Sprintf("Đã ghi nhận %s", typeVi),
			Description: fmt.Sprintf("%s ₫ - %s", formatVND(a.TotalAmount), a.Title),
			Data:        created,
		},
	}, nil
}

// --- record_debt_repayment ---

type debtRepaymentArgs struct {
	DebtID   uuid.UUID
	Amount   float64
	Notes    string
	WalletID *uuid.UUID // nil: the repayment moves no balance
}

func (a *debtRepaymentArgs) execute(ctx context.Context, s *chatService, userID uuid.UUID) (*FinancialToolResult, error) {
	walletName := ""
	if a.WalletID != nil {
		found, err := s.currentWallets(ctx, userID, *a.WalletID)
		if err != nil {
			return nil, err
		}
		if found[0] == nil {
			return proposalError("Cần xác nhận ví thanh toán", staleWalletMessage("trả nợ")), nil
		}
		walletName = found[0].Name
	}

	// DebtService.AddRepayment re-checks that the debt still exists, its
	// status and its remaining amount (business errors → 422 on confirm).
	updated, err := s.debtService.AddRepayment(ctx, userID, a.DebtID, dto.AddRepaymentRequest{Amount: a.Amount, Notes: a.Notes, WalletID: a.WalletID})
	if err != nil {
		return nil, err
	}

	description := fmt.Sprintf("Đã trả %s ₫ cho '%s' (Còn lại: %s ₫)", formatVND(a.Amount), updated.Title, formatVND(updated.Remaining))
	if walletName != "" {
		description += fmt.Sprintf(" - Ví %s", walletName)
	}
	resultBytes, _ := json.Marshal(updated)
	return &FinancialToolResult{
		ToolTitle:  "Đang cập nhật thanh toán nợ...",
		ResultJSON: string(resultBytes),
		ActionCard: &dto.ActionCard{
			ActionType:  "DEBT_REPAID",
			Title:       "Đã ghi nhận thanh toán nợ",
			Description: description,
			Data:        updated,
		},
	}, nil
}

// --- set_budget ---

type setBudgetArgs struct {
	CategoryID   uuid.UUID
	CategoryName string
	Amount       float64
	PeriodStart  time.Time // calendar day (UTC midnight)
}

func (a *setBudgetArgs) execute(ctx context.Context, s *chatService, userID uuid.UUID) (*FinancialToolResult, error) {
	budget, err := s.budgetService.CreateBudget(ctx, userID, dto.CreateBudgetRequest{
		CategoryID: a.CategoryID, Amount: a.Amount, PeriodType: model.BudgetPeriodMonthly, PeriodStart: a.PeriodStart,
	})
	if err != nil {
		// The category was deleted since the preview.
		if errors.Is(err, constant.ErrNotFound) {
			return proposalError("Đặt ngân sách", fmt.Sprintf("Không tìm thấy danh mục '%s' trong hệ thống của bạn", a.CategoryName)), nil
		}
		return nil, err
	}
	resultBytes, _ := json.Marshal(budget)
	return &FinancialToolResult{
		ToolTitle:  "Đang thiết lập ngân sách...",
		ResultJSON: string(resultBytes),
		ActionCard: &dto.ActionCard{
			ActionType:  "BUDGET_SET",
			Title:       "Đã thiết lập ngân sách",
			Description: fmt.Sprintf("Hạn mức danh mục %s: %s ₫/tháng", a.CategoryName, formatVND(a.Amount)),
			Data:        budget,
		},
	}, nil
}

// --- set_financial_target ---

type setTargetArgs struct {
	TargetType string // EXPENSE | INVESTMENT
	Amount     float64
	Month      int
	Year       int
}

// targetTypeVi labels a (normalized) target type.
func targetTypeVi(targetType string) string {
	if targetType == string(model.TargetTypeInvestment) {
		return "Mục tiêu đầu tư / tiết kiệm"
	}
	return "Hạn mức chi tiêu tối đa"
}

func (a *setTargetArgs) execute(ctx context.Context, s *chatService, userID uuid.UUID) (*FinancialToolResult, error) {
	if err := s.targetService.UpsertTarget(ctx, userID, &dto.UpsertTargetRequest{
		TargetType: a.TargetType, TargetAmount: a.Amount, Month: a.Month, Year: a.Year,
	}); err != nil {
		return nil, err
	}
	resultBytes, _ := json.Marshal(map[string]interface{}{
		"success": true, "targetType": a.TargetType, "targetAmount": a.Amount, "month": a.Month, "year": a.Year,
		"message": "Mục tiêu tài chính đã được thiết lập thành công.",
	})
	return &FinancialToolResult{
		ToolTitle:  "Đang lưu mục tiêu tài chính...",
		ResultJSON: string(resultBytes),
		ActionCard: &dto.ActionCard{
			ActionType:  "TARGET_SET",
			Title:       "Đã lưu mục tiêu tài chính",
			Description: fmt.Sprintf("%s Tháng %d/%d: %s ₫", targetTypeVi(a.TargetType), a.Month, a.Year, formatVND(a.Amount)),
			Data:        map[string]interface{}{"targetType": a.TargetType, "targetAmount": a.Amount, "month": a.Month, "year": a.Year},
		},
	}, nil
}
