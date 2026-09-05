package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost                 string
	DBPort                 string
	DBUser                 string
	DBPass                 string
	DBName                 string
	DBSSL                  string
	Port                   string
	LogLevel               string
	JwtSecret              string
	AppEnv                 string
	NineRouterURL          string
	NineRouterAPIKey       string
	NineRouterModel        string
	NineRouterEmbeddingModel string
	VapidPublicKey         string
	VapidPrivateKey        string
	VapidSubject           string
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
		NineRouterURL:          nineRouterURL,
		NineRouterAPIKey:       nineRouterAPIKey,
		NineRouterModel:        nineRouterModel,
		NineRouterEmbeddingModel: nineRouterEmbeddingModel,
		VapidPublicKey:         getEnv("VAPID_PUBLIC_KEY", "BAyz5fFinQHdqEWjHznwDfqpRMIrJshJd31quXzgE-aRMBUd9F_a2iIhnxOocrbDe_mt_zFXOI_3BJVykFDMPBU"),
		VapidPrivateKey:        getEnv("VAPID_PRIVATE_KEY", "sfbZBDeRsCQYRhU56XEBl-8KL6LuNNbFGuGD0JSzXPg"),
		VapidSubject:           getEnv("VAPID_SUBJECT", "mailto:support@nexo.local"),
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
