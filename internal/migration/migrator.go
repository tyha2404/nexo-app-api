package migration

import (
	"fmt"
	"os"

	"github.com/tyha2404/nexo-app-api/internal/config"
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
	if config.IsProductionEnv(os.Getenv("APP_ENV")) {
		return fmt.Errorf("auto-migration is disabled in production")
	}

	// Enable uuid-ossp extension to allow uuid_generate_v4() function
	if err := m.db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`).Error; err != nil {
		return fmt.Errorf("failed to enable uuid-ossp extension: %w", err)
	}

	return m.db.AutoMigrate(
		&model.User{},
		&model.Category{},
		&model.Cost{},
		&model.Alert{},
		&model.Budget{},
		&model.Transaction{},
		&model.MonthlyTarget{},
		&model.Debt{},
		&model.Repayment{},
		&model.Wallet{},
		&model.WalletTransfer{},
		&model.CreditCardStatement{},
		&model.Preset{},
		&model.ChatSession{},
		&model.ChatMessage{},
		&model.FinancialKnowledge{},
		&model.RefreshToken{},
	)
}
