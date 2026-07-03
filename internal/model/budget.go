package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Budget struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	CategoryID  uuid.UUID `gorm:"type:uuid;not null;index" json:"categoryId"`
	Amount      float64   `gorm:"type:numeric(10,2);not null" json:"amount"`
	PeriodType  string    `gorm:"type:varchar(10);not null;check:period_type_check,period_type IN ('monthly','yearly')" json:"periodType"`
	PeriodStart time.Time `gorm:"type:date;not null" json:"periodStart"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt   DeletedAt `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`

	User     User     `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category Category `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

// BeforeCreate GORM Hook to generate UUID v7
func (b *Budget) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == uuid.Nil {
		b.ID, err = uuid.NewV7()
	}
	return err
}
