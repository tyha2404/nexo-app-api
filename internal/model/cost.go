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
	UserID     uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	CategoryID uuid.UUID `gorm:"type:uuid;not null;index" json:"categoryId"`
	IncurredAt time.Time `json:"incurredAt" validate:"required"`
	CreatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt  DeletedAt `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`

	User     *User     `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category *Category `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}
