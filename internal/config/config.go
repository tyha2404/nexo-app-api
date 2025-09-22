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
	JwtSecret string
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	c := &Config{
		DBHost:    getEnv("DB_HOST", "localhost"),
		DBPort:    getEnv("DB_PORT", "5432"),
		DBUser:    getEnv("DB_USER", "postgres"),
		DBPass:    getEnv("DB_PASS", "postgres"),
		DBName:    getEnv("DB_NAME", "costdb"),
		DBSSL:     getEnv("DB_SSLMODE", "disable"),
		Port:      getEnv("APP_PORT", "3000"),
		LogLevel:  getEnv("LOG_LEVEL", "info"),
		JwtSecret: getEnv("JWT_SECRET", "secret"),
	}

	if c.DBHost == "" {
		return nil, fmt.Errorf("DB_HOST is required")
	}
	return c, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
