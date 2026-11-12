package handler

import (
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

// Concrete (generic-free) response envelopes for the Swagger docs: swag
// cannot parse response.BaseResponse[T] / response.PaginationResponse[T].
// Each one mirrors the JSON written by ErrorHandler.HandleSuccess /
// HandleSuccessWithMessage ({status, success, message, data}) or
// HandlePaginated ({status, success, message, items, total, page, limit[,
// summary]}). message is "" unless the endpoint sets one.

// MessageResponse is the envelope of an endpoint that only reports success:
// every DELETE (except the legacy /costs) and PUT /targets/monthly.
// {status: 200, success: true, message, data: null}.
type MessageResponse struct {
	Status  int         `json:"status" example:"200"`
	Success bool        `json:"success" example:"true"`
	Message string      `json:"message" example:"Transaction deleted successfully"`
	Data    interface{} `json:"data" swaggertype:"object"`
}

// --- auth / users ---

type LoginEnvelope struct {
	Status  int               `json:"status"`
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Data    dto.LoginResponse `json:"data"`
}

type TokenEnvelope struct {
	Status  int               `json:"status"`
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Data    dto.TokenResponse `json:"data"`
}

// LogoutEnvelope: data is {"message": "Logged out"}.
type LogoutEnvelope struct {
	Status  int               `json:"status"`
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Data    map[string]string `json:"data"`
}

type ChangePasswordEnvelope struct {
	Status  int                        `json:"status"`
	Success bool                       `json:"success"`
	Message string                     `json:"message"`
	Data    dto.ChangePasswordResponse `json:"data"`
}

type UserEnvelope struct {
	Status  int        `json:"status"`
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Data    model.User `json:"data"`
}

type UserPage struct {
	Status  int          `json:"status"`
	Success bool         `json:"success"`
	Message string       `json:"message"`
	Items   []model.User `json:"items"`
	Total   int          `json:"total"`
	Page    int          `json:"page"`
	Limit   int          `json:"limit"`
}

// --- categories ---

type CategoryEnvelope struct {
	Status  int            `json:"status"`
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    model.Category `json:"data"`
}

type CategoryPage struct {
	Status  int              `json:"status"`
	Success bool             `json:"success"`
	Message string           `json:"message"`
	Items   []model.Category `json:"items"`
	Total   int              `json:"total"`
	Page    int              `json:"page"`
	Limit   int              `json:"limit"`
}

// --- transactions ---

type TransactionEnvelope struct {
	Status  int                     `json:"status"`
	Success bool                    `json:"success"`
	Message string                  `json:"message"`
	Data    dto.TransactionResponse `json:"data"`
}

type TransactionPage struct {
	Status  int                        `json:"status"`
	Success bool                       `json:"success"`
	Message string                     `json:"message"`
	Items   []dto.TransactionResponse  `json:"items"`
	Total   int                        `json:"total"`
	Page    int                        `json:"page"`
	Limit   int                        `json:"limit"`
	Summary *dto.TransactionSummaryDTO `json:"summary"`
}

// --- wallets ---

type WalletEnvelope struct {
	Status  int                `json:"status"`
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Data    dto.WalletResponse `json:"data"`
}

type WalletSummaryEnvelope struct {
	Status  int                       `json:"status"`
	Success bool                      `json:"success"`
	Message string                    `json:"message"`
	Data    dto.WalletSummaryResponse `json:"data"`
}

type WalletTransferEnvelope struct {
	Status  int                        `json:"status"`
	Success bool                       `json:"success"`
	Message string                     `json:"message"`
	Data    dto.WalletTransferResponse `json:"data"`
}

type AutoAllocateEnvelope struct {
	Status  int                      `json:"status"`
	Success bool                     `json:"success"`
	Message string                   `json:"message"`
	Data    dto.AutoAllocateResponse `json:"data"`
}

// --- budgets / alerts / presets / targets ---

type BudgetEnvelope struct {
	Status  int                `json:"status"`
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Data    dto.BudgetResponse `json:"data"`
}

type BudgetPage struct {
	Status  int                  `json:"status"`
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Items   []dto.BudgetResponse `json:"items"`
	Total   int                  `json:"total"`
	Page    int                  `json:"page"`
	Limit   int                  `json:"limit"`
}

type AlertPage struct {
	Status  int                 `json:"status"`
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Items   []dto.AlertResponse `json:"items"`
	Total   int                 `json:"total"`
	Page    int                 `json:"page"`
	Limit   int                 `json:"limit"`
}

type PresetEnvelope struct {
	Status  int                `json:"status"`
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Data    dto.PresetResponse `json:"data"`
}

type PresetListEnvelope struct {
	Status  int                  `json:"status"`
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Data    []dto.PresetResponse `json:"data"`
}

type TargetSummaryEnvelope struct {
	Status  int                       `json:"status"`
	Success bool                      `json:"success"`
	Message string                    `json:"message"`
	Data    dto.TargetSummaryResponse `json:"data"`
}

// --- debts ---

type DebtListEnvelope struct {
	Status  int                `json:"status"`
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Data    []dto.DebtResponse `json:"data"`
}

type DebtSummaryEnvelope struct {
	Status  int                     `json:"status"`
	Success bool                    `json:"success"`
	Message string                  `json:"message"`
	Data    dto.DebtSummaryResponse `json:"data"`
}

// --- credit card statements ---

type StatementEnvelope struct {
	Status  int                   `json:"status"`
	Success bool                  `json:"success"`
	Message string                `json:"message"`
	Data    dto.StatementResponse `json:"data"`
}

type StatementListEnvelope struct {
	Status  int                       `json:"status"`
	Success bool                      `json:"success"`
	Message string                    `json:"message"`
	Data    dto.StatementListResponse `json:"data"`
}

// --- reports ---

type SummaryReportEnvelope struct {
	Status  int               `json:"status"`
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Data    dto.SummaryReport `json:"data"`
}

type CategoryBreakdownEnvelope struct {
	Status  int                         `json:"status"`
	Success bool                        `json:"success"`
	Message string                      `json:"message"`
	Data    dto.CategoryBreakdownReport `json:"data"`
}

// --- chat ---

type ChatSessionEnvelope struct {
	Status  int                     `json:"status"`
	Success bool                    `json:"success"`
	Message string                  `json:"message"`
	Data    dto.ChatSessionResponse `json:"data"`
}

type ChatSessionListEnvelope struct {
	Status  int                       `json:"status"`
	Success bool                      `json:"success"`
	Message string                    `json:"message"`
	Data    []dto.ChatSessionResponse `json:"data"`
}

type ChatModelsEnvelope struct {
	Status  int                    `json:"status"`
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Data    dto.ChatModelsResponse `json:"data"`
}

type ChatActionResultEnvelope struct {
	Status  int                  `json:"status"`
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Data    dto.ChatActionResult `json:"data"`
}

// StringEnvelope: data is a confirmation message (POST /chat/clear).
type StringEnvelope struct {
	Status  int    `json:"status"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data" example:"Đã xóa toàn bộ lịch sử trò chuyện"`
}

// --- legacy /costs (nexo-app only) ---

type CostEnvelope struct {
	Status  int        `json:"status"`
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Data    model.Cost `json:"data"`
}

// CostPage is the legacy /costs list: no message field.
type CostPage struct {
	Status  int          `json:"status"`
	Success bool         `json:"success"`
	Items   []model.Cost `json:"items"`
	Total   int          `json:"total"`
	Page    int          `json:"page"`
	Limit   int          `json:"limit"`
}
