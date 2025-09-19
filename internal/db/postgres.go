package db

import (
	"fmt"

	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewPostgres(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.DBHost, cfg.DBUser, cfg.DBPass, cfg.DBName, cfg.DBPort, cfg.DBSSL,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate (for small projects; for production use migrations)
	if err := db.AutoMigrate(&model.Cost{}, &model.Category{}); err != nil {
		return nil, err
	}

	return db, nil
}
