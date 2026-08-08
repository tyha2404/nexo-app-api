package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TargetType string

const (
	TargetTypeExpense    TargetType = "EXPENSE"
	TargetTypeInvestment TargetType = "INVESTMENT"
)

type MonthlyTarget struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       uuid.UUID  `gorm:"type:uuid;not null;index:idx_user_target_month_type,unique" json:"userId"`
	TargetType   TargetType `gorm:"type:varchar(20);not null;index:idx_user_target_month_type,unique" json:"targetType"`
	TargetAmount float64    `gorm:"type:numeric(12,2);not null" json:"targetAmount"`
	Month        int        `gorm:"not null;index:idx_user_target_month_type,unique" json:"month"`
	Year         int        `gorm:"not null;index:idx_user_target_month_type,unique" json:"year"`
	CreatedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt    DeletedAt  `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`

	User *User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (t *MonthlyTarget) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID, err = uuid.NewV7()
	}
	return err
}
