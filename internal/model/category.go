package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CategoryType string

const (
	CategoryTypeIncome     CategoryType = "INCOME"
	CategoryTypeExpense    CategoryType = "EXPENSE"
	CategoryTypeInvestment CategoryType = "INVESTMENT"
)

type Category struct {
	ID          uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID    `gorm:"type:uuid;not null;index" json:"userId"`
	Name        string       `gorm:"type:varchar(50);not null;index:idx_user_category_name,unique" json:"name"`
	Type                    CategoryType `gorm:"type:varchar(10);not null;default:'EXPENSE';check:type IN ('INCOME', 'EXPENSE', 'INVESTMENT')" json:"type"`
	Description             *string      `gorm:"type:text" json:"description,omitempty"`
	ExcludeFromAverageDaily bool         `gorm:"type:boolean;not null;default:false" json:"excludeFromAverageDaily"`
	CreatedAt               time.Time    `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time    `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt   DeletedAt    `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`

	User *User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// BeforeCreate GORM Hook to generate UUID v7
func (c *Category) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID, err = uuid.NewV7()
	}
	return err
}
