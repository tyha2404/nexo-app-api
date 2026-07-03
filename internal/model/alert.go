package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Alert struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
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

// BeforeCreate GORM Hook to generate UUID v7
func (a *Alert) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == uuid.Nil {
		a.ID, err = uuid.NewV7()
	}
	return err
}
