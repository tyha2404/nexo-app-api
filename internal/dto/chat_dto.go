package dto

import (
	"time"

	"github.com/google/uuid"
)

type SendMessageRequest struct {
	SessionID *uuid.UUID `json:"sessionId,omitempty"`
	Message   string     `json:"message" validate:"required"`
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
	ToolResults *string   `json:"toolResults,omitempty"`
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
	ActionType  string      `json:"actionType"` // "TRANSACTION_CREATED", "BUDGET_ALERT", "FINANCIAL_SUMMARY", "KNOWLEDGE_SOURCE"
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Data        interface{} `json:"data,omitempty"`
}
