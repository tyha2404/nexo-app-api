package config

import (
	"reflect"
	"testing"
)

func TestParseOrigins(t *testing.T) {
	got := ParseOrigins(" http://localhost:5173/ , ,https://nexo.example.com")
	want := []string{"http://localhost:5173", "https://nexo.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLoadConfig_TimezoneAndCORSDefaults(t *testing.T) {
	t.Setenv("JWT_SECRET", "a_test_secret_that_is_at_least_32_characters")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("APP_TIMEZONE", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppTimezone != DefaultAppTimezone {
		t.Fatalf("expected default timezone, got %q", cfg.AppTimezone)
	}
	if !reflect.DeepEqual(cfg.CORSAllowedOrigins, ParseOrigins(DefaultCORSAllowedOrigins)) {
		t.Fatalf("unexpected default origins %v", cfg.CORSAllowedOrigins)
	}

	t.Setenv("APP_TIMEZONE", "Mars/Olympus")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error for invalid APP_TIMEZONE")
	}
}

func TestLoadConfig_TokenTTLs(t *testing.T) {
	t.Setenv("JWT_SECRET", "a_test_secret_that_is_at_least_32_characters")
	t.Setenv("JWT_ACCESS_TTL", "")
	t.Setenv("JWT_REFRESH_TTL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JwtAccessTTL != DefaultJwtAccessTTL || cfg.JwtRefreshTTL != DefaultJwtRefreshTTL {
		t.Fatalf("unexpected default TTLs %s / %s", cfg.JwtAccessTTL, cfg.JwtRefreshTTL)
	}

	t.Setenv("JWT_ACCESS_TTL", "5m")
	t.Setenv("JWT_REFRESH_TTL", "168h")
	if cfg, err = LoadConfig(); err != nil || cfg.JwtAccessTTL.Minutes() != 5 || cfg.JwtRefreshTTL.Hours() != 168 {
		t.Fatalf("expected 5m / 168h, got %v (%v)", cfg, err)
	}

	for _, tc := range [][2]string{{"soon", "720h"}, {"-1m", "720h"}, {"2h", "1h"}} {
		t.Setenv("JWT_ACCESS_TTL", tc[0])
		t.Setenv("JWT_REFRESH_TTL", tc[1])
		if _, err := LoadConfig(); err == nil {
			t.Errorf("expected error for access %q / refresh %q", tc[0], tc[1])
		}
	}
}

func TestIsProductionEnv(t *testing.T) {
	for env, want := range map[string]bool{
		"production": true, "prod": true, "Production": true, " PROD ": true,
		"dev": false, "": false, "staging": false, "preprod": false,
	} {
		if got := IsProductionEnv(env); got != want {
			t.Errorf("IsProductionEnv(%q) = %v, want %v", env, got, want)
		}
		if got := (&Config{AppEnv: env}).IsProduction(); got != want {
			t.Errorf("Config{AppEnv: %q}.IsProduction() = %v, want %v", env, got, want)
		}
	}
}

func TestLoadConfig_AutoMigrateIsOptIn(t *testing.T) {
	t.Setenv("JWT_SECRET", "a_test_secret_that_is_at_least_32_characters")
	for raw, want := range map[string]bool{"": false, "false": false, "1": false, "true": true, "TRUE": true} {
		t.Setenv("AUTO_MIGRATE", raw)
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AutoMigrate != want {
			t.Errorf("AUTO_MIGRATE=%q: got %v, want %v", raw, cfg.AutoMigrate, want)
		}
	}
}

func TestLoadConfig_ProdDisablesLocalRegisterByDefault(t *testing.T) {
	t.Setenv("JWT_SECRET", "a_test_secret_that_is_at_least_32_characters")
	t.Setenv("LOCAL_REGISTER_ENABLED", "")
	for env, want := range map[string]bool{"prod": false, "production": false, "dev": true} {
		t.Setenv("APP_ENV", env)
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.LocalRegisterEnabled != want {
			t.Errorf("APP_ENV=%q: LocalRegisterEnabled=%v, want %v", env, cfg.LocalRegisterEnabled, want)
		}
	}
}
