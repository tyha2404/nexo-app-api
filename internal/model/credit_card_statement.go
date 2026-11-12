package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type StatementStatus string

const (
	StatementStatusUnpaid        StatementStatus = "UNPAID"
	StatementStatusPartiallyPaid StatementStatus = "PARTIALLY_PAID"
	StatementStatusPaid          StatementStatus = "PAID"
	StatementStatusOverdue       StatementStatus = "OVERDUE"
)

// StatementStatusFor is the stored status that follows a statement's amounts:
// PAID once a positive balance is fully paid, PARTIALLY_PAID after any
// payment, UNPAID otherwise. OVERDUE is derived at read time from the due date
// (see dto.ToStatementResponse).
func StatementStatusFor(statementBalance, paidAmount float64) StatementStatus {
	switch {
	case paidAmount >= statementBalance && statementBalance > 0:
		return StatementStatusPaid
	case paidAmount > 0:
		return StatementStatusPartiallyPaid
	default:
		return StatementStatusUnpaid
	}
}

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

// BeforeCreate GORM Hook to generate UUID v7
func (s *CreditCardStatement) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID, err = uuid.NewV7()
	}
	return err
}
