package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // embed the tz database so APP_TIMEZONE works on hosts without zoneinfo

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost    string
	DBPort    string
	DBUser    string
	DBPass    string
	DBName    string
	DBSSL     string
	Port      string
	LogLevel  string
	JwtSecret string
	// JwtAccessTTL is how long an API-issued access token lives
	// (JWT_ACCESS_TTL, Go duration, default 15m). Clients renew it with the
	// refresh token.
	JwtAccessTTL time.Duration
	// JwtRefreshTTL is how long a refresh token lives without being used
	// (JWT_REFRESH_TTL, Go duration, default 720h = 30 days). Each refresh
	// rotates it and restarts this window.
	JwtRefreshTTL            time.Duration
	AppEnv                   string
	NineRouterURL            string
	NineRouterAPIKey         string
	NineRouterModel          string
	NineRouterEmbeddingModel string
	// NineRouterAllowedModels limits the models users may pick for chat
	// (NINE_ROUTER_ALLOWED_MODELS, comma-separated). Empty offers every model
	// the gateway lists.
	NineRouterAllowedModels []string
	SupabaseURL             string
	SupabaseJWKSURL         string
	// CORSAllowedOrigins is the browser-origin allowlist (CORS_ALLOWED_ORIGINS,
	// comma-separated). Production calls are same-origin through the nexo-web
	// proxy, so this mostly matters for local development.
	CORSAllowedOrigins []string
	// LocalRegisterEnabled allows POST /auth/register (LOCAL_REGISTER_ENABLED).
	// Defaults to false in production, where sign-up goes through Supabase.
	LocalRegisterEnabled bool
	// AppTimezone is the IANA zone used for "today"/"this month" (APP_TIMEZONE).
	AppTimezone string
}

// DefaultCORSAllowedOrigins are used when CORS_ALLOWED_ORIGINS is unset: the
// Vite dev server. In production nexo-web's server.cjs serves the app and
// proxies /api on the same origin, so no cross-origin entry is needed.
const DefaultCORSAllowedOrigins = "http://localhost:5173,http://127.0.0.1:5173"

// Default token lifetimes when JWT_ACCESS_TTL / JWT_REFRESH_TTL are unset.
const (
	DefaultJwtAccessTTL  = 15 * time.Minute
	DefaultJwtRefreshTTL = 30 * 24 * time.Hour
)

// DefaultAppTimezone is the timezone of the app's users (Vietnam).
const DefaultAppTimezone = "Asia/Ho_Chi_Minh"

// ParseOrigins splits a comma-separated origin list, trimming blanks and
// trailing slashes.
func ParseOrigins(raw string) []string {
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	nineRouterURL := getEnv("NINE_ROUTER_URL", "")
	if nineRouterURL == "" {
		nineRouterURL = getEnv("AI_BASE_URL", "http://localhost:20128/v1/")
	}

	nineRouterAPIKey := getEnv("NINE_ROUTER_API_KEY", "")
	if nineRouterAPIKey == "" {
		nineRouterAPIKey = getEnv("AI_API_KEY", "9router-local")
	}

	nineRouterModel := getEnv("NINE_ROUTER_MODEL", "")
	if nineRouterModel == "" {
		nineRouterModel = getEnv("AI_MODEL", "gemini-2.5-flash")
	}

	nineRouterEmbeddingModel := getEnv("NINE_ROUTER_EMBEDDING_MODEL", "")
	if nineRouterEmbeddingModel == "" {
		nineRouterEmbeddingModel = getEnv("AI_EMBEDDING_MODEL", "embedding-3")
	}

	c := &Config{
		DBHost:                   getEnv("DB_HOST", "localhost"),
		DBPort:                   getEnv("DB_PORT", "5432"),
		DBUser:                   getEnv("DB_USER", "postgres"),
		DBPass:                   getEnv("DB_PASS", "postgres"),
		DBName:                   getEnv("DB_NAME", "costdb"),
		DBSSL:                    getEnv("DB_SSLMODE", "disable"),
		Port:                     getEnv("APP_PORT", "3001"),
		LogLevel:                 getEnv("LOG_LEVEL", "info"),
		JwtSecret:                getEnv("JWT_SECRET", "secret"),
		AppEnv:                   getEnv("APP_ENV", "dev"),
		NineRouterURL:            nineRouterURL,
		NineRouterAPIKey:         nineRouterAPIKey,
		NineRouterModel:          nineRouterModel,
		NineRouterEmbeddingModel: nineRouterEmbeddingModel,
		NineRouterAllowedModels:  splitList(getEnv("NINE_ROUTER_ALLOWED_MODELS", "")),
		SupabaseURL:              getEnv("SUPABASE_URL", ""),
		SupabaseJWKSURL:          getEnv("SUPABASE_JWKS_URL", ""),
		CORSAllowedOrigins:       ParseOrigins(getEnv("CORS_ALLOWED_ORIGINS", DefaultCORSAllowedOrigins)),
		AppTimezone:              getEnv("APP_TIMEZONE", DefaultAppTimezone),
	}

	var err error
	if c.JwtAccessTTL, err = getDuration("JWT_ACCESS_TTL", DefaultJwtAccessTTL); err != nil {
		return nil, err
	}
	if c.JwtRefreshTTL, err = getDuration("JWT_REFRESH_TTL", DefaultJwtRefreshTTL); err != nil {
		return nil, err
	}
	if c.JwtRefreshTTL <= c.JwtAccessTTL {
		return nil, fmt.Errorf("JWT_REFRESH_TTL (%s) must be longer than JWT_ACCESS_TTL (%s)", c.JwtRefreshTTL, c.JwtAccessTTL)
	}

	c.LocalRegisterEnabled = getEnv("LOCAL_REGISTER_ENABLED", strconv.FormatBool(!c.IsProduction())) == "true"

	if _, err := time.LoadLocation(c.AppTimezone); err != nil {
		return nil, fmt.Errorf("APP_TIMEZONE %q is not a valid IANA timezone: %w", c.AppTimezone, err)
	}

	if c.SupabaseURL != "" && c.SupabaseJWKSURL == "" {
		c.SupabaseJWKSURL = fmt.Sprintf("%s/auth/v1/.well-known/jwks.json", c.SupabaseURL)
	}

	// Security validations
	if c.DBHost == "" {
		return nil, fmt.Errorf("DB_HOST is required")
	}

	if c.JwtSecret == "secret" || c.JwtSecret == "replace_me" || len(c.JwtSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be set to a secure value (minimum 32 characters)")
	}

	return c, nil
}

// splitList splits a comma-separated list, dropping blank entries.
func splitList(raw string) []string {
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// IsProduction reports whether APP_ENV is "production".
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getDuration reads a positive Go duration (e.g. "15m", "720h").
func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s %q must be a positive duration such as 15m or 720h", key, raw)
	}
	return d, nil
}
