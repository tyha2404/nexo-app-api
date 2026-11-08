package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RefreshToken is one rotation step of a login session
// (migration 20261108100000_create_refresh_tokens_table.sql). Only the
// SHA-256 hash of the token is stored.
type RefreshToken struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index:idx_refresh_tokens_user_id"`
	FamilyID   uuid.UUID `gorm:"type:uuid;not null;index:idx_refresh_tokens_family_id"`
	TokenHash  string    `gorm:"type:varchar(64);not null;uniqueIndex"`
	ExpiresAt  time.Time `gorm:"not null"`
	RevokedAt  *time.Time
	ReplacedBy *uuid.UUID `gorm:"type:uuid"`
	UserAgent  *string    `gorm:"type:text"`
	CreatedAt  time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

// BeforeCreate GORM Hook to generate UUID v7
func (t *RefreshToken) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID, err = uuid.NewV7()
	}
	return err
}
