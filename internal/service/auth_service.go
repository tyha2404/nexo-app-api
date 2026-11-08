package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Login(ctx context.Context, identifier string, password string) (*model.User, error)
	Register(ctx context.Context, user *model.User) (*model.User, error)
	// ChangePassword also revokes every refresh token of the user, signing
	// out all sessions; the caller starts a new one for the current client.
	ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error

	// StartSession issues the first refresh token of a new login session.
	StartSession(ctx context.Context, userID uuid.UUID, userAgent string) (string, error)
	// RefreshSession rotates a refresh token: it returns the token's user and
	// its replacement. Any invalid, expired, revoked or reused token gives
	// constant.ErrInvalidRefreshToken (reuse also revokes the session).
	RefreshSession(ctx context.Context, refreshToken, userAgent string) (*model.User, string, error)
	// EndSession revokes the session of a refresh token (logout). Unknown
	// tokens are not an error.
	EndSession(ctx context.Context, refreshToken string) error
}

type authService struct {
	repo        repository.UserRepo
	refreshRepo repository.RefreshTokenRepo
	refreshTTL  time.Duration
}

// refreshTokenBytes is the entropy of a refresh token (base64url: 43 chars).
const refreshTokenBytes = 32

// maxUserAgentLen caps the user agent stored with a refresh token.
const maxUserAgentLen = 255

// dummyPasswordHash is compared against when there is no hash to check, to
// keep login timing uniform.
var dummyPasswordHash = sync.OnceValue(func() []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte("nexo-timing-equaliser"), bcrypt.DefaultCost)
	return hash
})

// NewAuthService builds the auth service. refreshTTL <= 0 uses
// config.DefaultJwtRefreshTTL.
func NewAuthService(repo repository.UserRepo, refreshRepo repository.RefreshTokenRepo, refreshTTL time.Duration) AuthService {
	if refreshTTL <= 0 {
		refreshTTL = config.DefaultJwtRefreshTTL
	}
	return &authService{
		repo:        repo,
		refreshRepo: refreshRepo,
		refreshTTL:  refreshTTL,
	}
}

func (s *authService) Login(ctx context.Context, identifier string, password string) (*model.User, error) {
	// 1. Find user by email or username
	var user *model.User
	var err error

	user, err = s.repo.FindByEmail(ctx, identifier)
	if err != nil && err != constant.ErrNotFound {
		return nil, err
	}

	if user == nil {
		user, err = s.repo.FindByUsername(ctx, identifier)
		if err != nil {
			if err == constant.ErrNotFound {
				// Spend the same bcrypt time as a wrong password, so the
				// response time doesn't reveal whether the account exists.
				_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(password))
				return nil, constant.ErrInvalidCredentials
			}
			return nil, err
		}
	}

	// 2. Verify password hash. Supabase accounts have no local hash.
	if user.Password == "" {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(password))
		return nil, constant.ErrInvalidCredentials
	}
	if err := user.CheckPassword(password); err != nil {
		return nil, constant.ErrInvalidCredentials
	}

	// 3. Clear the password hash from the returned user for security
	user.Password = ""

	// Note: JWT token generation would typically happen here in the handler
	// and not in the service layer, following separation of concerns

	// 4. Return user data
	return user, nil
}

func (s *authService) Register(ctx context.Context, user *model.User) (*model.User, error) {
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))

	// Validate user data
	if err := user.Validate(); err != nil {
		return nil, err
	}

	// Check if user with email already exists
	existingUser, err := s.repo.FindByEmail(ctx, user.Email)
	if err != nil && err != constant.ErrNotFound {
		return nil, err
	}
	if existingUser != nil {
		return nil, constant.ErrEmailAlreadyExists
	}

	// Check if username is taken
	existingUser, err = s.repo.FindByUsername(ctx, user.Username)
	if err != nil && err != constant.ErrNotFound {
		return nil, err
	}
	if existingUser != nil {
		return nil, constant.ErrUsernameTaken
	}

	// Generate password hash
	if err := user.HashPassword(); err != nil {
		return nil, err
	}

	// Save user to database
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Get the created user to return
	createdUser, err := s.repo.GetByID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// Clear the password hash from the returned user for security
	createdUser.Password = ""

	return createdUser, nil
}

// ChangePassword replaces the password of a locally registered user after
// verifying the current one. Supabase-synced users have an empty local hash
// and must change their password through Supabase Auth instead.
func (s *authService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.Password == "" {
		return constant.ErrPasswordManagedExternally
	}
	if err := user.CheckPassword(currentPassword); err != nil {
		return constant.ErrInvalidCredentials
	}
	if currentPassword == newPassword {
		return constant.ErrSamePassword
	}

	user.Password = newPassword
	if err := user.HashPassword(); err != nil {
		return err
	}

	if err := s.repo.UpdateFields(ctx, userID, map[string]interface{}{"password": user.Password}); err != nil {
		return err
	}

	// A changed password must end sessions opened with the old one (e.g. on
	// a lost phone). Access tokens already issued expire on their own.
	return s.refreshRepo.RevokeAllForUser(ctx, userID, time.Now())
}

func (s *authService) StartSession(ctx context.Context, userID uuid.UUID, userAgent string) (string, error) {
	now := time.Now()
	// Housekeeping: drop this user's expired tokens while we are here.
	if err := s.refreshRepo.DeleteExpiredForUser(ctx, userID, now); err != nil {
		return "", err
	}

	raw, hash, err := newRefreshToken()
	if err != nil {
		return "", err
	}
	familyID, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	token := &model.RefreshToken{
		UserID:    userID,
		FamilyID:  familyID,
		TokenHash: hash,
		ExpiresAt: now.Add(s.refreshTTL),
		UserAgent: userAgentPtr(userAgent),
	}
	if err := s.refreshRepo.Create(ctx, token); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *authService) RefreshSession(ctx context.Context, refreshToken, userAgent string) (*model.User, string, error) {
	if refreshToken == "" {
		return nil, "", constant.ErrInvalidRefreshToken
	}

	raw, hash, err := newRefreshToken()
	if err != nil {
		return nil, "", err
	}
	now := time.Now()
	next := &model.RefreshToken{
		TokenHash: hash,
		ExpiresAt: now.Add(s.refreshTTL),
		UserAgent: userAgentPtr(userAgent),
	}

	old, err := s.refreshRepo.Rotate(ctx, hashRefreshToken(refreshToken), next, now)
	if errors.Is(err, repository.ErrRefreshTokenReused) {
		return nil, "", constant.ErrInvalidRefreshToken
	}
	if err != nil {
		return nil, "", err
	}

	// Re-read the user so the new access token carries the current role,
	// username and email, and a deleted user can't keep refreshing.
	user, err := s.repo.GetByID(ctx, old.UserID)
	if errors.Is(err, constant.ErrNotFound) {
		return nil, "", constant.ErrInvalidRefreshToken
	}
	if err != nil {
		return nil, "", err
	}
	user.Password = ""
	return user, raw, nil
}

func (s *authService) EndSession(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.refreshRepo.RevokeFamilyByHash(ctx, hashRefreshToken(refreshToken), time.Now())
}

// newRefreshToken returns a random token and the hash stored for it.
func newRefreshToken() (raw, hash string, err error) {
	b := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashRefreshToken(raw), nil
}

// hashRefreshToken is the lookup key of a token. A plain SHA-256 is enough:
// the token is 256 random bits, so it can't be brute-forced from the hash.
func hashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func userAgentPtr(ua string) *string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return nil
	}
	if len(ua) > maxUserAgentLen {
		ua = strings.ToValidUTF8(ua[:maxUserAgentLen], "")
	}
	return &ua
}
