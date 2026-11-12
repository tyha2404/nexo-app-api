package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/migration"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// BuildDSN builds the Postgres DSN (also used by cmd/migrate). The session TimeZone is set to the app
// timezone so date/timestamptz casts and date_trunc in SQL agree with the
// "today"/"this month" boundaries computed in Go (util.Now()).
func BuildDSN(cfg *config.Config) string {
	tz := cfg.AppTimezone
	if tz == "" {
		tz = config.DefaultAppTimezone
	}
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		cfg.DBHost, cfg.DBUser, cfg.DBPass, cfg.DBName, cfg.DBPort, cfg.DBSSL, tz,
	)
}

// gormLogLevel maps LOG_LEVEL to the GORM log level. GORM's Info level prints
// every SQL statement, so it is never used in production: there the level is
// capped at Warn (slow queries and errors only).
func gormLogLevel(cfg *config.Config) gormlogger.LogLevel {
	var level gormlogger.LogLevel
	switch strings.ToLower(strings.TrimSpace(cfg.LogLevel)) {
	case "debug", "info":
		level = gormlogger.Info // dev: show SQL queries
	case "warn":
		level = gormlogger.Warn
	case "error":
		level = gormlogger.Error
	default:
		level = gormlogger.Info // default for development
	}
	if cfg.IsProduction() && level > gormlogger.Warn {
		level = gormlogger.Warn
	}
	return level
}

// newGormLogger builds the GORM logger for cfg. In production the logged SQL
// keeps its $n placeholders instead of the bound values (emails, password and
// refresh-token hashes, chat text), also for slow-query and error logs.
func newGormLogger(cfg *config.Config, logger *zap.Logger) gormlogger.Interface {
	return &GormLogger{
		logger:        logger,
		level:         gormLogLevel(cfg),
		parameterized: cfg.IsProduction(),
	}
}

// shouldAutoMigrate reports whether NewPostgres runs GORM AutoMigrate: only
// when explicitly enabled (AUTO_MIGRATE=true) and never in production. Goose
// migrations are the schema source of truth.
func shouldAutoMigrate(cfg *config.Config) bool {
	return cfg.AutoMigrate && !cfg.IsProduction()
}

func NewPostgres(cfg *config.Config, logger *zap.Logger) (*gorm.DB, error) {
	dsn := BuildDSN(cfg)

	// Create GORM configuration with custom logger
	gormConfig := &gorm.Config{
		Logger: newGormLogger(cfg, logger),
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // Required for connection poolers like Supavisor / PgBouncer
	}), gormConfig)
	if err != nil {
		return nil, err
	}

	// Configure Connection Pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)                  // Maximum number of open connections to the database
	sqlDB.SetMaxIdleConns(10)                  // Maximum number of connections in the idle connection pool
	sqlDB.SetConnMaxLifetime(10 * time.Minute) // Maximum amount of time a connection may be reused

	// GORM AutoMigrate is opt-in (AUTO_MIGRATE=true) and refused in production;
	// the schema is managed with goose (cmd/migrate).
	switch {
	case shouldAutoMigrate(cfg):
		logger.Warn("AUTO_MIGRATE=true: running GORM AutoMigrate (goose migrations remain the schema source of truth)")
		migrator := migration.NewMigrator(db)
		if err := migrator.AutoMigrate(); err != nil {
			return nil, fmt.Errorf("migration failed: %w", err)
		}
	case cfg.AutoMigrate:
		logger.Warn("AUTO_MIGRATE is ignored in production")
	}

	return db, nil
}
