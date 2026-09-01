package model

import (
	"time"

	"github.com/google/uuid"
)

type StatementStatus string

const (
	StatementStatusUnpaid        StatementStatus = "UNPAID"
	StatementStatusPartiallyPaid StatementStatus = "PARTIALLY_PAID"
	StatementStatusPaid          StatementStatus = "PAID"
	StatementStatusOverdue       StatementStatus = "OVERDUE"
)

type CreditCardStatement struct {
	ID               uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	UserID           uuid.UUID       `gorm:"type:uuid;not null;index" json:"userId"`
	WalletID         uuid.UUID       `gorm:"type:uuid;not null;index" json:"walletId"`
	StatementMonth   int             `gorm:"type:int;not null" json:"statementMonth"`
	StatementYear    int             `gorm:"type:int;not null" json:"statementYear"`
	StatementDate    time.Time       `gorm:"type:date;not null" json:"statementDate"`
	DueDate          time.Time       `gorm:"type:date;not null" json:"dueDate"`
	StatementBalance float64         `gorm:"type:numeric(15,2);not null;default:0.00" json:"statementBalance"`
	MinimumPayment   float64         `gorm:"type:numeric(15,2);not null;default:0.00" json:"minimumPayment"`
	PreviousBalance  float64         `gorm:"type:numeric(15,2);not null;default:0.00" json:"previousBalance"`
	PaidAmount       float64         `gorm:"type:numeric(15,2);not null;default:0.00" json:"paidAmount"`
	Status           StatementStatus `gorm:"type:varchar(20);not null;default:'UNPAID'" json:"status"`
	Note             string          `gorm:"type:text" json:"note"`
	CreatedAt        time.Time       `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt        time.Time       `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt        DeletedAt       `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`

	Wallet *Wallet `gorm:"foreignKey:WalletID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"wallet,omitempty"`
	User   *User   `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user,omitempty"`
}
