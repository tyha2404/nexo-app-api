package config

import (
	"fmt"
	"os"

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
	JwtSecret         string
	AppEnv            string
	RequestyApiKey         string
	RequestyBaseURL        string
	RequestyModel          string
	RequestyEmbeddingModel string
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	apiKey := getEnv("REQUESTY_API_KEY", "")
	if apiKey == "" {
		apiKey = getEnv("AI_API_KEY", "")
	}

	baseURL := getEnv("REQUESTY_BASE_URL", "https://router.requesty.ai/v1/")
	model := getEnv("REQUESTY_MODEL", "google/gemma-4-31b-it")
	embeddingModel := getEnv("REQUESTY_EMBEDDING_MODEL", "embedding-3")

	c := &Config{
		DBHost:                 getEnv("DB_HOST", "localhost"),
		DBPort:                 getEnv("DB_PORT", "5432"),
		DBUser:                 getEnv("DB_USER", "postgres"),
		DBPass:                 getEnv("DB_PASS", "postgres"),
		DBName:                 getEnv("DB_NAME", "costdb"),
		DBSSL:                  getEnv("DB_SSLMODE", "disable"),
		Port:                   getEnv("APP_PORT", "3001"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		JwtSecret:              getEnv("JWT_SECRET", "secret"),
		AppEnv:                 getEnv("APP_ENV", "dev"),
		RequestyApiKey:         apiKey,
		RequestyBaseURL:        baseURL,
		RequestyModel:          model,
		RequestyEmbeddingModel: embeddingModel,
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

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
