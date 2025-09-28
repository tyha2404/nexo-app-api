package model

import (
	"time"

	"github.com/google/uuid"
)

type Budget struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	CategoryID  uuid.UUID `gorm:"type:uuid;not null;index" json:"category_id"`
	Amount      float64   `gorm:"type:numeric(10,2);not null" json:"amount"`
	PeriodType  string    `gorm:"type:varchar(10);not null;check:period_type_check,period_type IN ('monthly','yearly')" json:"period_type"`
	PeriodStart time.Time `gorm:"type:date;not null" json:"period_start"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`

	User     User     `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category Category `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}
