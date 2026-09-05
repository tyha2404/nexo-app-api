package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PushSubscription stores browser/device Web Push subscription details (RFC 8291/8292)
type PushSubscription struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	Endpoint   string    `gorm:"type:text;not null;unique" json:"endpoint"`
	P256dh     string    `gorm:"type:text;not null" json:"p256dh"`
	Auth       string    `gorm:"type:text;not null" json:"auth"`
	UserAgent  string    `gorm:"type:text" json:"userAgent,omitempty"`
	DeviceType string    `gorm:"type:varchar(20);default:'ios'" json:"deviceType"` // 'ios', 'android', 'desktop'
	CreatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	User User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

// TableName overrides the table name
func (PushSubscription) TableName() string {
	return "push_subscriptions"
}

// BeforeCreate GORM Hook to generate UUID v7
func (p *PushSubscription) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID, err = uuid.NewV7()
	}
	return err
}
