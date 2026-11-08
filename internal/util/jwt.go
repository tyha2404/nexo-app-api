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
	accessTTL    = config.DefaultJwtAccessTTL
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
	accessTTL = config.DefaultJwtAccessTTL
	if cfg.JwtAccessTTL > 0 {
		accessTTL = cfg.JwtAccessTTL
	}

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

// AccessTokenTTL is the lifetime of tokens made by GenerateToken.
func AccessTokenTTL() time.Duration {
	return accessTTL
}

// GenerateToken generates a new short-lived access token for the given user.
// Clients renew it with a refresh token (POST /auth/refresh).
func GenerateToken(user *model.User) (string, error) {
	if len(jwtKey) == 0 {
		return "", fmt.Errorf("JWT not initialized")
	}

	now := time.Now()
	expirationTime := now.Add(accessTTL)

	claims := &Claims{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(now),
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
		}, jwt.WithExpirationRequired())

		// Every token this API issues carries a user id. A valid signature
		// without one (e.g. a Supabase anon/service key, should JWT_SECRET
		// ever equal the Supabase legacy JWT secret) is not a user.
		if err == nil && token.Valid && claims.ID != uuid.Nil {
			return claims, nil
		}
	}

	// 2. Try Supabase JWKS verification if available
	if jwksCache != nil && jwksURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		set, err := jwksCache.Get(ctx, jwksURL)
		if err == nil {
			// Supabase user access tokens have aud "authenticated"; this
			// rejects other token kinds signed by the same project keys.
			parsedToken, err := jwxjwt.Parse([]byte(tokenString), jwxjwt.WithKeySet(set), jwxjwt.WithValidate(true),
				jwxjwt.WithAudience("authenticated"))
			if err == nil {
				sub := parsedToken.Subject()
				userUUID, err := uuid.Parse(sub)
				if err == nil {
					emailClaim, _ := parsedToken.Get("email")
					emailStr, _ := emailClaim.(string)

					// Only signed-in end users are accepted ("anon" and
					// "service_role" are not users of this API), and they
					// are always plain users here: admin is a local role.
					roleClaim, _ := parsedToken.Get("role")
					if roleStr, _ := roleClaim.(string); roleStr != "authenticated" {
						return nil, jwt.ErrTokenInvalidClaims
					}
					roleStr := "user"

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
