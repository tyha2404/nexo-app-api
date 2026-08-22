package dto

import "github.com/google/uuid"

type ParseNLPRequest struct {
	Text string `json:"text" binding:"required"`
}

type ParseNLPResponse struct {
	Amount          float64    `json:"amount"`
	CategoryID      *uuid.UUID `json:"categoryId,omitempty"`
	CategoryName    string     `json:"categoryName,omitempty"`
	Type            string     `json:"type"` // INCOME or EXPENSE
	Description     string     `json:"description"`
	TransactionDate string     `json:"transactionDate,omitempty"`
	ConfidenceScore float64    `json:"confidenceScore"`
}
