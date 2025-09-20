package model

import (
	"time"

	"github.com/google/uuid"
)

type Alert struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index"`
	BudgetID    uuid.UUID `gorm:"type:uuid;not null;index"`
	AlertType   string    `gorm:"type:varchar(20);not null;check:alert_type_check,alert_type IN ('approaching_limit','over_limit')"`
	TriggeredAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	Message     string    `gorm:"type:text;not null"`

	User   User   `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Budget Budget `gorm:"foreignKey:BudgetID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
