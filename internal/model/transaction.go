package model

import (
	"time"

	"github.com/google/uuid"
)

type TransactionType string

const (
	TransactionTypeIncome  TransactionType = "INCOME"
	TransactionTypeExpense TransactionType = "EXPENSE"
)

type Transaction struct {
	ID              uuid.UUID       `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID          uuid.UUID       `gorm:"type:uuid;not null;index" json:"userId"`
	CategoryID      uuid.UUID       `gorm:"type:uuid;not null;index" json:"categoryId"`
	Amount          float64         `gorm:"type:numeric(15,2);not null" json:"amount"`
	Type            TransactionType `gorm:"type:varchar(10);not null;check:type IN ('INCOME', 'EXPENSE')" json:"type"`
	Description     *string         `gorm:"type:text" json:"description,omitempty"`
	TransactionDate time.Time       `gorm:"type:date;not null;index:idx_user_transaction_date" json:"transactionDate"`
	CreatedAt       time.Time       `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt       time.Time       `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt       DeletedAt       `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`

	User     *User     `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category *Category `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}
