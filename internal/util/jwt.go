package util

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

var jwtKey []byte

// InitJWT initializes JWT secret from config
func InitJWT(cfg *config.Config) {
	if cfg.JwtSecret == "replace_me" || cfg.JwtSecret == "" {
		panic("JWT_SECRET must be set to a secure value")
	}
	jwtKey = []byte(cfg.JwtSecret)
}

// Claims represents the JWT claims
type Claims struct {
	ID       uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	jwt.RegisteredClaims
}

// GenerateToken generates a new JWT token for the given user
func GenerateToken(user *model.User) (string, error) {
	if len(jwtKey) == 0 {
		return "", fmt.Errorf("JWT not initialized")
	}

	expirationTime := time.Now().Add(24 * time.Hour)

	claims := &Claims{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken validates the JWT token and returns the claims if valid
func ValidateToken(tokenString string) (*Claims, error) {
	if len(jwtKey) == 0 {
		return nil, fmt.Errorf("JWT not initialized")
	}

	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}
