package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

// mockRefreshTokenRepo is an in-memory repository.RefreshTokenRepo with the
// same semantics as the GORM one.
type mockRefreshTokenRepo struct {
	byHash map[string]*model.RefreshToken
}

func newMockRefreshTokenRepo() *mockRefreshTokenRepo {
	return &mockRefreshTokenRepo{byHash: make(map[string]*model.RefreshToken)}
}

func (m *mockRefreshTokenRepo) Create(_ context.Context, token *model.RefreshToken) error {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	m.byHash[token.TokenHash] = token
	return nil
}

func (m *mockRefreshTokenRepo) Rotate(ctx context.Context, tokenHash string, next *model.RefreshToken, now time.Time) (*model.RefreshToken, error) {
	old, ok := m.byHash[tokenHash]
	if !ok {
		return nil, constant.ErrInvalidRefreshToken
	}
	if old.RevokedAt != nil {
		m.revokeWhere(func(t *model.RefreshToken) bool { return t.FamilyID == old.FamilyID }, now)
		return nil, repository.ErrRefreshTokenReused
	}
	if !old.ExpiresAt.After(now) {
		return nil, constant.ErrInvalidRefreshToken
	}
	next.UserID = old.UserID
	next.FamilyID = old.FamilyID
	_ = m.Create(ctx, next)
	old.RevokedAt = &now
	old.ReplacedBy = &next.ID
	copied := *old
	return &copied, nil
}

func (m *mockRefreshTokenRepo) RevokeFamilyByHash(_ context.Context, tokenHash string, now time.Time) error {
	if t, ok := m.byHash[tokenHash]; ok {
		m.revokeWhere(func(o *model.RefreshToken) bool { return o.FamilyID == t.FamilyID }, now)
	}
	return nil
}

func (m *mockRefreshTokenRepo) RevokeAllForUser(_ context.Context, userID uuid.UUID, now time.Time) error {
	m.revokeWhere(func(t *model.RefreshToken) bool { return t.UserID == userID }, now)
	return nil
}

func (m *mockRefreshTokenRepo) DeleteExpiredForUser(_ context.Context, userID uuid.UUID, now time.Time) error {
	for h, t := range m.byHash {
		if t.UserID == userID && !t.ExpiresAt.After(now) {
			delete(m.byHash, h)
		}
	}
	return nil
}

func (m *mockRefreshTokenRepo) revokeWhere(match func(*model.RefreshToken) bool, now time.Time) {
	for _, t := range m.byHash {
		if match(t) && t.RevokedAt == nil {
			at := now
			t.RevokedAt = &at
		}
	}
}

func newSessionFixture(t *testing.T) (AuthService, *mockUserRepo, *mockRefreshTokenRepo, *model.User) {
	t.Helper()
	users := newMockUserRepo()
	tokens := newMockRefreshTokenRepo()
	user := &model.User{Username: "sessionuser", Email: "session@example.com", Password: "password123", Role: "user"}
	_ = user.HashPassword()
	_ = users.Create(context.Background(), user)
	return NewAuthService(users, tokens, time.Hour), users, tokens, user
}

func TestAuthService_StartSession_StoresOnlyHash(t *testing.T) {
	svc, _, tokens, user := newSessionFixture(t)

	raw, err := svc.StartSession(context.Background(), user.ID, "Mozilla/5.0 "+strings.Repeat("x", 400))
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if len(raw) != 43 {
		t.Errorf("expected a 43-char base64url token, got %d chars", len(raw))
	}
	if _, ok := tokens.byHash[raw]; ok {
		t.Fatal("raw token must not be stored")
	}
	stored, ok := tokens.byHash[hashRefreshToken(raw)]
	if !ok {
		t.Fatal("expected the token hash to be stored")
	}
	if stored.UserID != user.ID || stored.FamilyID == uuid.Nil {
		t.Errorf("unexpected stored token: %+v", stored)
	}
	if d := time.Until(stored.ExpiresAt); d < 59*time.Minute || d > time.Hour {
		t.Errorf("expected expiry ~1h from now, got %s", d)
	}
	if stored.UserAgent == nil || len(*stored.UserAgent) != maxUserAgentLen {
		t.Errorf("expected user agent truncated to %d chars", maxUserAgentLen)
	}
}

func TestAuthService_RefreshSession_Rotates(t *testing.T) {
	svc, _, tokens, user := newSessionFixture(t)
	ctx := context.Background()

	first, _ := svc.StartSession(ctx, user.ID, "")
	gotUser, second, err := svc.RefreshSession(ctx, first, "")
	if err != nil {
		t.Fatalf("RefreshSession: %v", err)
	}
	if gotUser.ID != user.ID || gotUser.Password != "" {
		t.Errorf("expected the session's user without password, got %+v", gotUser)
	}
	if second == "" || second == first {
		t.Fatal("expected a new refresh token")
	}
	if tokens.byHash[hashRefreshToken(first)].FamilyID != tokens.byHash[hashRefreshToken(second)].FamilyID {
		t.Error("rotated token must stay in the same family")
	}

	// The new token works in turn.
	if _, _, err := svc.RefreshSession(ctx, second, ""); err != nil {
		t.Errorf("expected rotated token to refresh, got %v", err)
	}
}

func TestAuthService_RefreshSession_ReuseRevokesFamily(t *testing.T) {
	svc, _, _, user := newSessionFixture(t)
	ctx := context.Background()

	first, _ := svc.StartSession(ctx, user.ID, "")
	other, _ := svc.StartSession(ctx, user.ID, "") // another device
	_, second, _ := svc.RefreshSession(ctx, first, "")

	// An attacker replays the old token: rejected, and the session is dead.
	if _, _, err := svc.RefreshSession(ctx, first, ""); !errors.Is(err, constant.ErrInvalidRefreshToken) {
		t.Fatalf("expected reuse to be rejected, got %v", err)
	}
	if _, _, err := svc.RefreshSession(ctx, second, ""); !errors.Is(err, constant.ErrInvalidRefreshToken) {
		t.Errorf("expected the whole family to be revoked after reuse, got %v", err)
	}
	// Other sessions are untouched.
	if _, _, err := svc.RefreshSession(ctx, other, ""); err != nil {
		t.Errorf("expected the other device's session to survive, got %v", err)
	}
}

func TestAuthService_RefreshSession_Invalid(t *testing.T) {
	svc, users, tokens, user := newSessionFixture(t)
	ctx := context.Background()

	if _, _, err := svc.RefreshSession(ctx, "", ""); !errors.Is(err, constant.ErrInvalidRefreshToken) {
		t.Errorf("empty token: expected ErrInvalidRefreshToken, got %v", err)
	}
	if _, _, err := svc.RefreshSession(ctx, "unknown-token", ""); !errors.Is(err, constant.ErrInvalidRefreshToken) {
		t.Errorf("unknown token: expected ErrInvalidRefreshToken, got %v", err)
	}

	expired, _ := svc.StartSession(ctx, user.ID, "")
	tokens.byHash[hashRefreshToken(expired)].ExpiresAt = time.Now().Add(-time.Second)
	if _, _, err := svc.RefreshSession(ctx, expired, ""); !errors.Is(err, constant.ErrInvalidRefreshToken) {
		t.Errorf("expired token: expected ErrInvalidRefreshToken, got %v", err)
	}

	orphan, _ := svc.StartSession(ctx, user.ID, "")
	_ = users.Delete(ctx, user.ID)
	if _, _, err := svc.RefreshSession(ctx, orphan, ""); !errors.Is(err, constant.ErrInvalidRefreshToken) {
		t.Errorf("deleted user: expected ErrInvalidRefreshToken, got %v", err)
	}
}

func TestAuthService_EndSession(t *testing.T) {
	svc, _, _, user := newSessionFixture(t)
	ctx := context.Background()

	token, _ := svc.StartSession(ctx, user.ID, "")
	if err := svc.EndSession(ctx, token); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
	if _, _, err := svc.RefreshSession(ctx, token, ""); !errors.Is(err, constant.ErrInvalidRefreshToken) {
		t.Errorf("expected logged-out token to be rejected, got %v", err)
	}
	if err := svc.EndSession(ctx, "unknown-token"); err != nil {
		t.Errorf("unknown token logout should succeed, got %v", err)
	}
}

func TestAuthService_ChangePassword_RevokesSessions(t *testing.T) {
	svc, _, _, user := newSessionFixture(t)
	ctx := context.Background()

	phone, _ := svc.StartSession(ctx, user.ID, "")
	laptop, _ := svc.StartSession(ctx, user.ID, "")
	if err := svc.ChangePassword(ctx, user.ID, "password123", "newpassword123"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	for _, tok := range []string{phone, laptop} {
		if _, _, err := svc.RefreshSession(ctx, tok, ""); !errors.Is(err, constant.ErrInvalidRefreshToken) {
			t.Errorf("expected sessions to be revoked after a password change, got %v", err)
		}
	}
}

func TestAuthService_StartSession_DeletesExpired(t *testing.T) {
	svc, _, tokens, user := newSessionFixture(t)
	ctx := context.Background()

	old, _ := svc.StartSession(ctx, user.ID, "")
	tokens.byHash[hashRefreshToken(old)].ExpiresAt = time.Now().Add(-time.Minute)
	_, _ = svc.StartSession(ctx, user.ID, "")
	if _, ok := tokens.byHash[hashRefreshToken(old)]; ok {
		t.Error("expected expired tokens to be cleaned up on login")
	}
}
