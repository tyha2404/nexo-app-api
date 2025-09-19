package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Cost struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Title      string    `gorm:"size:255;not null" json:"title" validate:"required"`
	Amount     float64   `gorm:"type:numeric;not null" json:"amount" validate:"required,gt=0"`
	Currency   string    `gorm:"size:3;not null" json:"currency" validate:"required,len=3"`
	Category   string    `gorm:"size:100" json:"category"`
	IncurredAt time.Time `json:"incurred_at" validate:"required"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (c *Cost) BeforeCreate(tx *gorm.DB) error {
	// Set UUID if not already set
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	// Set timestamps
	now := time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	return nil
}
