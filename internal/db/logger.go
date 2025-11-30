package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GormLogger wraps zap logger to implement GORM's logger interface
type GormLogger struct {
	logger *zap.Logger
	level  gormlogger.LogLevel
}

// NewGormLogger creates a new GORM logger with zap
func NewGormLogger(logger *zap.Logger, level gormlogger.LogLevel) gormlogger.Interface {
	return &GormLogger{
		logger: logger,
		level:  level,
	}
}

// LogMode sets the log level for GORM logger
func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.level = level
	return &newLogger
}

// Info logs info messages
func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= gormlogger.Info {
		l.logger.Sugar().Infof(msg, data...)
	}
}

// Warn logs warning messages
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= gormlogger.Warn {
		l.logger.Sugar().Warnf(msg, data...)
	}
}

// Error logs error messages
func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= gormlogger.Error {
		l.logger.Sugar().Errorf(msg, data...)
	}
}

// Trace logs SQL queries with execution time
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	// Always log SQL queries to console for debugging
	fmt.Printf("\n[SQL] %s\n", sql)
	fmt.Printf("[Duration] %v\n", elapsed)
	fmt.Printf("[Rows Affected] %d\n", rows)
	if err != nil {
		fmt.Printf("[Error] %v\n", err)
	}
	fmt.Println("----------------------------------------")

	fields := []zap.Field{
		zap.String("sql", sql),
		zap.Duration("duration", elapsed),
		zap.Int64("rows_affected", rows),
	}

	switch {
	case err != nil && l.level >= gormlogger.Error && (!errors.Is(err, gorm.ErrRecordNotFound)):
		l.logger.Error("SQL Error", append(fields, zap.Error(err))...)
	case elapsed > 200*time.Millisecond && l.level >= gormlogger.Warn:
		l.logger.Warn("Slow SQL", append(fields, zap.String("threshold", "200ms"))...)
	case l.level >= gormlogger.Info:
		l.logger.Info("SQL Trace", fields...)
	default:
		// Always log SQL queries regardless of log level
		l.logger.Debug("SQL Query", fields...)
	}
}
