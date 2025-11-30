package model

import (
	"time"

	"github.com/google/uuid"
)

type Expense struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	CategoryID  uuid.UUID `gorm:"type:uuid;not null;index" json:"categoryId"`
	Amount      float64   `gorm:"type:numeric(10,2);not null" json:"amount"`
	Description *string   `gorm:"type:text" json:"description,omitempty"`
	ExpenseDate time.Time `gorm:"type:date;not null;index:idx_user_expense_date" json:"expenseDate"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt   DeletedAt `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`

	User     User     `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category Category `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}
