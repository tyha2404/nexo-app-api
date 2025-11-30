package db

import (
	"fmt"

	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/migration"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func NewPostgres(cfg *config.Config, logger *zap.Logger) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.DBHost, cfg.DBUser, cfg.DBPass, cfg.DBName, cfg.DBPort, cfg.DBSSL,
	)

	// Configure GORM logger based on log level
	var logLevel gormlogger.LogLevel
	switch cfg.LogLevel {
	case "debug":
		logLevel = gormlogger.Info
	case "info":
		logLevel = gormlogger.Info // Changed to Info to show SQL queries
	case "warn":
		logLevel = gormlogger.Warn
	case "error":
		logLevel = gormlogger.Error
	default:
		logLevel = gormlogger.Info // Default to Info for development
	}

	// Create GORM configuration with custom logger
	gormConfig := &gorm.Config{
		Logger: NewGormLogger(logger, logLevel),
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, err
	}

	// Use migrator instead of direct auto-migration
	migrator := migration.NewMigrator(db)
	if err := migrator.AutoMigrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return db, nil
}
