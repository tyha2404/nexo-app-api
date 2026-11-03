package util

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"

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

func TestValidateToken_RejectsTokenWithoutUserID(t *testing.T) {
	InitJWT(&config.Config{JwtSecret: "a_very_secure_secret_with_at_least_32_characters_long"})

	// Same secret, valid signature and expiry, but no user id: e.g. a
	// Supabase anon key if JWT_SECRET were the Supabase legacy secret.
	token, err := GenerateToken(&model.User{Role: "anon"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateToken(token); err == nil {
		t.Fatal("expected a token without a user id to be rejected")
	}
}

func TestValidateToken_RequiresExpiry(t *testing.T) {
	InitJWT(&config.Config{JwtSecret: "a_very_secure_secret_with_at_least_32_characters_long"})

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{ID: uuid.New(), Role: "user"}).SignedString(jwtKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateToken(token); err == nil {
		t.Fatal("expected a token without exp to be rejected")
	}
}
