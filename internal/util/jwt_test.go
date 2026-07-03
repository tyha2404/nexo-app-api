package util

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

func TestJWT(t *testing.T) {
	// Initialize JWT with a secure secret
	cfg := &config.Config{
		JwtSecret: "a_very_secure_secret_with_at_least_32_characters_long",
	}
	InitJWT(cfg)

	user := &model.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}

	// 1. Test token generation
	token, err := GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	if token == "" {
		t.Fatal("generated token is empty")
	}

	// 2. Test token validation
	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.ID != user.ID {
		t.Errorf("expected user ID %v, got %v", user.ID, claims.ID)
	}
	if claims.Username != user.Username {
		t.Errorf("expected username %s, got %s", user.Username, claims.Username)
	}
	if claims.Email != user.Email {
		t.Errorf("expected email %s, got %s", user.Email, claims.Email)
	}
	if claims.Role != user.Role {
		t.Errorf("expected role %s, got %s", user.Role, claims.Role)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	// Initialize JWT
	cfg := &config.Config{
		JwtSecret: "another_secure_secret_at_least_32_chars",
	}
	InitJWT(cfg)

	// Validate invalid token
	_, err := ValidateToken("invalid.token.string")
	if err == nil {
		t.Fatal("expected validation error for invalid token, got nil")
	}
}
