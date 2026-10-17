package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PushSubscription is one browser/device Web Push subscription
// (migration 20260905140000_create_push_subscriptions_table.sql).
type PushSubscription struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index:idx_push_subs_user_id" json:"userId"`
	Endpoint   string    `gorm:"type:text;not null;uniqueIndex" json:"endpoint"`
	P256dh     string    `gorm:"column:p256dh;type:text;not null" json:"p256dh"`
	Auth       string    `gorm:"type:text;not null" json:"auth"`
	UserAgent  *string   `gorm:"type:text" json:"userAgent,omitempty"`
	DeviceType string    `gorm:"type:varchar(20);default:'ios'" json:"deviceType"`
	CreatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (PushSubscription) TableName() string { return "push_subscriptions" }

// BeforeCreate GORM Hook to generate UUID v7
func (p *PushSubscription) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID, err = uuid.NewV7()
	}
	return err
}
