package model

import (
	"time"

	"github.com/google/uuid"
)

type DebtType string

const (
	DebtTypePayable    DebtType = "PAYABLE"    // Tôi nợ
	DebtTypeReceivable DebtType = "RECEIVABLE" // Người khác nợ tôi
)

type DebtStatus string

const (
	DebtStatusPending   DebtStatus = "PENDING"
	DebtStatusCompleted DebtStatus = "COMPLETED"
	DebtStatusOverdue   DebtStatus = "OVERDUE"
)

type Debt struct {
	ID          uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID   `gorm:"type:uuid;not null;index" json:"userId"`
	Type        DebtType    `gorm:"type:varchar(20);not null" json:"type"`
	Title       string      `gorm:"type:varchar(255);not null" json:"title"`
	TotalAmount float64     `gorm:"type:numeric(14,2);not null" json:"totalAmount"`
	PaidAmount  float64     `gorm:"type:numeric(14,2);default:0" json:"paidAmount"`
	StartDate   *time.Time  `gorm:"type:timestamp" json:"startDate"`
	DueDate     *time.Time  `gorm:"type:timestamp" json:"dueDate"`
	Status      DebtStatus  `gorm:"type:varchar(20);default:'PENDING'" json:"status"`
	Notes       string      `gorm:"type:text" json:"notes"`
	Repayments  []Repayment `gorm:"foreignKey:DebtID;constraint:OnDelete:CASCADE" json:"repayments,omitempty"`
	CreatedAt   time.Time   `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time   `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

type Repayment struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	DebtID    uuid.UUID `gorm:"type:uuid;not null;index" json:"debtId"`
	Amount    float64   `gorm:"type:numeric(14,2);not null" json:"amount"`
	PaidAt    time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"paidAt"`
	Notes     string    `gorm:"type:text" json:"notes"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
}
