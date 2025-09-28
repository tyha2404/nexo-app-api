package model

import (
	"time"

	"github.com/google/uuid"
)

type Alert struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	BudgetID    uuid.UUID `gorm:"type:uuid;not null;index" json:"budget_id"`
	AlertType   string    `gorm:"type:varchar(20);not null;check:alert_type_check,alert_type IN ('approaching_limit','over_limit')" json:"alert_type"`
	TriggeredAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"triggered_at"`
	Message     string    `gorm:"type:text;not null" json:"message"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`

	User   User   `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Budget Budget `gorm:"foreignKey:BudgetID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
