package db

import (
	"context"
	"strings"
	"testing"

	"github.com/tyha2404/nexo-app-api/internal/config"
	"go.uber.org/zap"
	gormlogger "gorm.io/gorm/logger"
)

func TestBuildDSNSetsTimeZone(t *testing.T) {
	cfg := &config.Config{DBHost: "h", DBUser: "u", DBPass: "p", DBName: "n", DBPort: "5432", DBSSL: "disable"}
	if dsn := BuildDSN(cfg); !strings.HasSuffix(dsn, "TimeZone=Asia/Ho_Chi_Minh") {
		t.Fatalf("expected default TimeZone in DSN, got %q", dsn)
	}
	cfg.AppTimezone = "UTC"
	if dsn := BuildDSN(cfg); !strings.HasSuffix(dsn, "TimeZone=UTC") {
		t.Fatalf("expected configured TimeZone in DSN, got %q", dsn)
	}
}

func TestGormLogLevel_NeverLogsEveryStatementInProduction(t *testing.T) {
	cases := []struct {
		logLevel, appEnv string
		want             gormlogger.LogLevel
	}{
		{"info", "dev", gormlogger.Info},
		{"debug", "dev", gormlogger.Info},
		{"", "dev", gormlogger.Info},
		{"warn", "dev", gormlogger.Warn},
		{"error", "dev", gormlogger.Error},
		{"info", "production", gormlogger.Warn},
		{"debug", "prod", gormlogger.Warn},
		{"", "production", gormlogger.Warn},
		{"warn", "production", gormlogger.Warn},
		{"error", "production", gormlogger.Error},
	}
	for _, tc := range cases {
		cfg := &config.Config{LogLevel: tc.logLevel, AppEnv: tc.appEnv}
		if got := gormLogLevel(cfg); got != tc.want {
			t.Errorf("LOG_LEVEL=%q APP_ENV=%q: got %v, want %v", tc.logLevel, tc.appEnv, got, tc.want)
		}
	}
}

func TestGormLogger_ParameterizedInProduction(t *testing.T) {
	ctx := context.Background()
	params := []interface{}{"a@b.c", "hash"}

	prod := newGormLogger(&config.Config{AppEnv: "production", LogLevel: "info"}, zap.NewNop()).(*GormLogger)
	sql, got := prod.ParamsFilter(ctx, "SELECT * FROM users WHERE email = $1 AND password = $2", params...)
	if got != nil || !strings.Contains(sql, "$1") {
		t.Fatalf("production must log placeholders only, got %q %v", sql, got)
	}

	dev := newGormLogger(&config.Config{AppEnv: "dev", LogLevel: "info"}, zap.NewNop()).(*GormLogger)
	if _, got := dev.ParamsFilter(ctx, "SELECT 1", params...); len(got) != 2 {
		t.Fatalf("dev keeps the values, got %v", got)
	}
}

func TestShouldAutoMigrate_OptInAndNeverInProduction(t *testing.T) {
	cases := []struct {
		auto   bool
		appEnv string
		want   bool
	}{
		{false, "dev", false},
		{true, "dev", true},
		{true, "", true},
		{true, "production", false},
		{true, "prod", false},
		{false, "production", false},
	}
	for _, tc := range cases {
		if got := shouldAutoMigrate(&config.Config{AutoMigrate: tc.auto, AppEnv: tc.appEnv}); got != tc.want {
			t.Errorf("AUTO_MIGRATE=%v APP_ENV=%q: got %v, want %v", tc.auto, tc.appEnv, got, tc.want)
		}
	}
}
