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
