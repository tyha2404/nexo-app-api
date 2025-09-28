package model

import (
	"time"

	"gorm.io/gorm"
)

// DeletedAt is a pointer to time.Time to handle soft deletes in GORM
// swagger:model DeletedAt
type DeletedAt *time.Time

// GormDeletedAt is a helper function to convert DeletedAt to gorm.DeletedAt
func GormDeletedAt(d DeletedAt) gorm.DeletedAt {
	if d == nil {
		return gorm.DeletedAt{}
	}
	return gorm.DeletedAt{Time: *d, Valid: true}
}
