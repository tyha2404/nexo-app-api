package model

import (
	"gorm.io/gorm"
)

// DeletedAt enables GORM soft deletes: Delete sets deleted_at instead of removing
// the row, and normal queries automatically add `deleted_at IS NULL`.
// It is an alias (not a new type) so GORM recognises the soft-delete hooks.
// Raw SQL, Joins strings and Unscoped queries are NOT filtered automatically.
// swagger:model DeletedAt
type DeletedAt = gorm.DeletedAt
