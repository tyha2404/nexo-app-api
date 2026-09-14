package util

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwk"
	jwxjwt "github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

var (
	jwtKey       []byte
	jwksCache    *jwk.Cache
	jwksURL      string
	jwksInitOnce sync.Once
)

// InitJWT initializes JWT secret from config
func InitJWT(cfg *config.Config) {
	if cfg.JwtSecret == "replace_me" || cfg.JwtSecret == "" {
		panic("JWT_SECRET must be set to a secure value")
	}
	jwtKey = []byte(cfg.JwtSecret)

	if cfg.SupabaseJWKSURL != "" {
		jwksURL = cfg.SupabaseJWKSURL
		jwksInitOnce.Do(func() {
			ctx := context.Background()
			c := jwk.NewCache(ctx)
			// Refresh cache every 15 minutes
			err := c.Register(jwksURL, jwk.WithMinRefreshInterval(15*time.Minute))
			if err == nil {
				jwksCache = c
			}
		})
	}
}

// Claims represents the JWT claims
type Claims struct {
	ID       uuid.UUID `json:"userId"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
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
		Role:     user.Role,
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

// ValidateToken validates the JWT token (supports both local HS256 and Supabase JWKS)
func ValidateToken(tokenString string) (*Claims, error) {
	// 1. Try local HS256 verification first
	if len(jwtKey) > 0 {
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtKey, nil
		})

		if err == nil && token.Valid {
			return claims, nil
		}
	}

	// 2. Try Supabase JWKS verification if available
	if jwksCache != nil && jwksURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		set, err := jwksCache.Get(ctx, jwksURL)
		if err == nil {
			parsedToken, err := jwxjwt.Parse([]byte(tokenString), jwxjwt.WithKeySet(set), jwxjwt.WithValidate(true))
			if err == nil {
				sub := parsedToken.Subject()
				userUUID, err := uuid.Parse(sub)
				if err == nil {
					emailClaim, _ := parsedToken.Get("email")
					emailStr, _ := emailClaim.(string)

					roleClaim, _ := parsedToken.Get("role")
					roleStr, _ := roleClaim.(string)
					if roleStr == "" || roleStr == "authenticated" {
						roleStr = "user"
					}

					username := emailStr
					if strings.Contains(emailStr, "@") {
						username = strings.Split(emailStr, "@")[0]
					}

					return &Claims{
						ID:       userUUID,
						Username: username,
						Email:    emailStr,
						Role:     roleStr,
					}, nil
				}
			}
		}
	}

	return nil, jwt.ErrSignatureInvalid
}
