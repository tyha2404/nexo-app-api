package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Category struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Title     string    `gorm:"size:255;not null" json:"title" validate:"required"`
	ParentID  uuid.UUID `gorm:"type:uuid" json:"parent_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *Category) BeforeCreate(tx *gorm.DB) error {
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
