package model

import (
	"time"

	"github.com/google/uuid"
)

type Budget struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index"`
	CategoryID  uuid.UUID `gorm:"type:uuid;not null;index"`
	Amount      float64   `gorm:"type:numeric(10,2);not null"`
	PeriodType  string    `gorm:"type:varchar(10);not null;check:period_type_check,period_type IN ('monthly','yearly')"`
	PeriodStart time.Time `gorm:"type:date;not null"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP"`

	User     User     `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category Category `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}
