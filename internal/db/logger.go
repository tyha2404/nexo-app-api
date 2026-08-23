package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/logger"
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

// Trace logs SQL queries with fine-grained syntax highlighting and execution metrics
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()
	nowStr := fmt.Sprintf("%s%s%s", logger.HiBlack, time.Now().Format("15:04:05.000"), logger.Reset)
	highlightedSQL := logger.HighlightSQL(sql)

	rowsStr := fmt.Sprintf("%s[%d %s]%s", logger.Magenta, rows, func() string {
		if rows == 1 {
			return "row"
		}
		return "rows"
	}(), logger.Reset)

	switch {
	case err != nil && l.level >= gormlogger.Error && (!errors.Is(err, gorm.ErrRecordNotFound)):
		fmt.Printf("%s%s[SQL ✖ ERR]%s %s | %s | %s\n      ↳ %sError:%s %v\n",
			logger.BgRed, logger.White, logger.Reset,
			nowStr,
			logger.ColorDuration(elapsed),
			highlightedSQL,
			logger.Red+logger.Bold, logger.Reset, err,
		)
		l.logger.Error("SQL Error",
			zap.String("sql", sql),
			zap.Duration("duration", elapsed),
			zap.Error(err),
		)
	case elapsed > 200*time.Millisecond && l.level >= gormlogger.Warn:
		fmt.Printf("%s%s[SQL ⚠️ SLOW]%s %s | %s | %s %s\n",
			logger.BgYellow, logger.White, logger.Reset,
			nowStr,
			logger.ColorDuration(elapsed),
			rowsStr,
			highlightedSQL,
		)
		l.logger.Warn("Slow SQL",
			zap.String("sql", sql),
			zap.Duration("duration", elapsed),
			zap.Int64("rows_affected", rows),
		)
	case l.level >= gormlogger.Info:
		fmt.Printf("%s%s[SQL 🗄️]%s %s | %s | %s %s\n",
			logger.BgBlue, logger.White, logger.Reset,
			nowStr,
			logger.ColorDuration(elapsed),
			rowsStr,
			highlightedSQL,
		)
	default:
		l.logger.Debug("SQL Query",
			zap.String("sql", sql),
			zap.Duration("duration", elapsed),
			zap.Int64("rows_affected", rows),
		)
	}
}
