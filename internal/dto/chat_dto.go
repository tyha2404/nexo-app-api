package dto

import (
	"time"

	"github.com/google/uuid"
)

// MaxChatMessageLength is the maximum length of one user chat message, in
// characters (runes). Longer messages are rejected with 400.
const MaxChatMessageLength = 4000

type SendMessageRequest struct {
	SessionID *uuid.UUID `json:"sessionId,omitempty"`
	Message   string     `json:"message" validate:"required" maxLength:"4000"`
	// Model must be one of GET /chat/models; any other value is ignored and
	// the server default is used.
	Model *string `json:"model,omitempty"`
}

type ChatModelsResponse struct {
	Models []string `json:"models"`
}

type ChatSessionResponse struct {
	ID        uuid.UUID             `json:"id"`
	Title     string                `json:"title"`
	CreatedAt time.Time             `json:"createdAt"`
	UpdatedAt time.Time             `json:"updatedAt"`
	Messages  []ChatMessageResponse `json:"messages,omitempty"`
}

type ChatMessageResponse struct {
	ID          uuid.UUID `json:"id"`
	SessionID   uuid.UUID `json:"sessionId"`
	Role        string    `json:"role"`
	Content     string    `json:"content"`
	Status      string    `json:"status"`
	ToolCalls   *string   `json:"toolCalls,omitempty"`
	// ToolResults is a JSON array of the ActionCards shown with this message.
	ToolResults *string `json:"toolResults,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ChatStreamEvent struct {
	Type         string      `json:"type"` // "session_info", "tool_start", "tool_done", "text_delta", "action_card", "error", "done"
	SessionID    *uuid.UUID  `json:"sessionId,omitempty"`
	MessageID    *uuid.UUID  `json:"messageId,omitempty"`
	Status       string      `json:"status,omitempty"` // "PENDING", "STREAMING", "SUCCESS", "ERROR"
	Delta        string      `json:"delta,omitempty"`
	ToolName     string      `json:"toolName,omitempty"`
	ToolTitle    string      `json:"toolTitle,omitempty"`
	ActionCard   *ActionCard `json:"actionCard,omitempty"`
	ErrorMessage string      `json:"errorMessage,omitempty"`
}

type ActionCard struct {
	// ActionType: "TRANSACTION_CREATED", "BUDGET_ALERT", "FINANCIAL_SUMMARY", "KNOWLEDGE_SOURCE",
	// "PENDING_CONFIRMATION" (AI write proposal; data has actionId, toolName,
	// kind, status PENDING|CONFIRMED|CANCELLED|EXPIRED, expiresAt, amount,
	// type, categoryName, walletName, date, description, resultCard).
	ActionType  string      `json:"actionType"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Data        interface{} `json:"data,omitempty"`
}

// ChatActionResult is returned by POST /chat/actions/{id}/confirm and /cancel.
type ChatActionResult struct {
	ActionID uuid.UUID `json:"actionId"`
	// Status is "CONFIRMED" or "CANCELLED".
	Status   string `json:"status"`
	ToolName string `json:"toolName"`
	// ActionCard is the card of the executed action (e.g. TRANSACTION_CREATED
	// with the new transaction id), only on confirm.
	ActionCard *ActionCard `json:"actionCard,omitempty"`
	// Result is the created/updated entity returned by the tool, only on confirm.
	Result interface{} `json:"result,omitempty"`
}
