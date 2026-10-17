package db

import (
	"strings"
	"testing"

	"github.com/tyha2404/nexo-app-api/internal/config"
)

func TestBuildDSNSetsTimeZone(t *testing.T) {
	cfg := &config.Config{DBHost: "h", DBUser: "u", DBPass: "p", DBName: "n", DBPort: "5432", DBSSL: "disable"}
	if dsn := buildDSN(cfg); !strings.HasSuffix(dsn, "TimeZone=Asia/Ho_Chi_Minh") {
		t.Fatalf("expected default TimeZone in DSN, got %q", dsn)
	}
	cfg.AppTimezone = "UTC"
	if dsn := buildDSN(cfg); !strings.HasSuffix(dsn, "TimeZone=UTC") {
		t.Fatalf("expected configured TimeZone in DSN, got %q", dsn)
	}
}
