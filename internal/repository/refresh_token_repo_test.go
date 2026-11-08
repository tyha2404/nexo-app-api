package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

// Integration test (skipped unless NEXO_TEST_DATABASE_URL is set).
func TestRefreshTokenRotateAndReuse(t *testing.T) {
	db := testDatabase(t)
	repo := NewRefreshTokenRepo(db)
	ctx := context.Background()
	now := time.Now()
	user, family := uuid.New(), uuid.New()

	first := &model.RefreshToken{UserID: user, FamilyID: family, TokenHash: "h1", ExpiresAt: now.Add(time.Hour)}
	other := &model.RefreshToken{UserID: user, FamilyID: uuid.New(), TokenHash: "other", ExpiresAt: now.Add(time.Hour)}
	require.NoError(t, repo.Create(ctx, first))
	require.NoError(t, repo.Create(ctx, other))

	// Two concurrent refreshes with the same token: exactly one rotates.
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			next := &model.RefreshToken{TokenHash: "h2-" + uuid.NewString(), ExpiresAt: now.Add(time.Hour)}
			_, results[i] = repo.Rotate(ctx, "h1", next, now)
		}(i)
	}
	wg.Wait()
	ok, reused := 0, 0
	for _, err := range results {
		switch err {
		case nil:
			ok++
		case ErrRefreshTokenReused:
			reused++
		default:
			t.Fatalf("unexpected error %v", err)
		}
	}
	require.Equal(t, 1, ok)
	require.Equal(t, 1, reused)

	// The reuse revoked the whole family, including the successor.
	var live int64
	require.NoError(t, db.Model(&model.RefreshToken{}).Where("family_id = ? AND revoked_at IS NULL", family).Count(&live).Error)
	require.Zero(t, live)
	// ...but not the user's other session.
	require.NoError(t, db.Model(&model.RefreshToken{}).Where("token_hash = 'other' AND revoked_at IS NULL").Count(&live).Error)
	require.EqualValues(t, 1, live)

	// Unknown and expired tokens.
	_, err := repo.Rotate(ctx, "nope", &model.RefreshToken{TokenHash: "x1", ExpiresAt: now.Add(time.Hour)}, now)
	require.ErrorIs(t, err, constant.ErrInvalidRefreshToken)
	expired := &model.RefreshToken{UserID: user, FamilyID: uuid.New(), TokenHash: "old", ExpiresAt: now.Add(-time.Minute)}
	require.NoError(t, repo.Create(ctx, expired))
	_, err = repo.Rotate(ctx, "old", &model.RefreshToken{TokenHash: "x2", ExpiresAt: now.Add(time.Hour)}, now)
	require.ErrorIs(t, err, constant.ErrInvalidRefreshToken)

	// Logout revokes the family; cleanup drops expired rows only.
	require.NoError(t, repo.RevokeFamilyByHash(ctx, "other", now))
	require.NoError(t, db.Model(&model.RefreshToken{}).Where("token_hash = 'other' AND revoked_at IS NULL").Count(&live).Error)
	require.Zero(t, live)
	require.NoError(t, repo.DeleteExpiredForUser(ctx, user, now))
	var count int64
	require.NoError(t, db.Model(&model.RefreshToken{}).Where("token_hash = 'old'").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Model(&model.RefreshToken{}).Where("user_id = ?", user).Count(&count).Error)
	require.EqualValues(t, 3, count) // h1, its successor, other
}
