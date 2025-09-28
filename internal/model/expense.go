package model

import (
	"time"

	"github.com/google/uuid"
)

type Expense struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	CategoryID  uuid.UUID `gorm:"type:uuid;not null;index" json:"category_id"`
	Amount      float64   `gorm:"type:numeric(10,2);not null" json:"amount"`
	Description *string   `gorm:"type:text" json:"description,omitempty"`
	ExpenseDate time.Time `gorm:"type:date;not null;index:idx_user_expense_date" json:"expense_date"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`

	User     User     `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category Category `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}
