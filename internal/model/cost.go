package model

import (
	"time"

	"github.com/google/uuid"
)

type Cost struct {
	ID         uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Title      string    `gorm:"size:255;not null" json:"title" validate:"required"`
	Amount     float64   `gorm:"type:numeric;not null" json:"amount" validate:"required,gt=0"`
	Currency   string    `gorm:"size:3;not null" json:"currency" validate:"required,len=3"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	CategoryID uuid.UUID `gorm:"type:uuid;not null;index" json:"category_id"`
	IncurredAt time.Time `json:"incurred_at" validate:"required"`
	CreatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt  DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`

	User     *User     `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category *Category `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}
