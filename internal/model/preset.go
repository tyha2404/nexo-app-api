package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Preset struct {
	ID          uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID       `gorm:"type:uuid;not null;index" json:"userId"`
	CategoryID  uuid.UUID       `gorm:"type:uuid;not null;index" json:"categoryId"`
	Name        string          `gorm:"type:varchar(100);not null" json:"name"`
	Amount      float64         `gorm:"type:numeric(12,2);not null" json:"amount"`
	Type        TransactionType `gorm:"type:varchar(20);not null" json:"type"`
	Description string          `gorm:"type:varchar(255)" json:"description"`
	Icon        string          `gorm:"type:varchar(50)" json:"icon"`
	SortOrder   int             `gorm:"default:0" json:"sortOrder"`
	CreatedAt   time.Time       `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time       `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	User     User     `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category Category `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func (p *Preset) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID, err = uuid.NewV7()
	}
	return err
}
