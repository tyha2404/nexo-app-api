package migration

import (
	"fmt"
	"os"

	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type Migrator struct {
	db *gorm.DB
}

func NewMigrator(db *gorm.DB) *Migrator {
	return &Migrator{db: db}
}

func (m *Migrator) AutoMigrate() error {
	if os.Getenv("APP_ENV") == "production" {
		return fmt.Errorf("auto-migration is disabled in production")
	}

	return m.db.AutoMigrate(
		&model.User{},
		&model.Category{},
		&model.Cost{},
		&model.Alert{},
		&model.Expense{},
		&model.Budget{},
	)
}

func (m *Migrator) CreateMigrationsTable() error {
	return m.db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
}
