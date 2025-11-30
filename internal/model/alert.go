package model

import (
	"time"

	"github.com/google/uuid"
)

type Alert struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	BudgetID    uuid.UUID `gorm:"type:uuid;not null;index" json:"budgetId"`
	AlertType   string    `gorm:"type:varchar(20);not null;check:alert_type_check,alert_type IN ('approaching_limit','over_limit')" json:"alertType"`
	TriggeredAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"triggeredAt"`
	Message     string    `gorm:"type:text;not null" json:"message"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt   DeletedAt `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`

	User   User   `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Budget Budget `gorm:"foreignKey:BudgetID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
