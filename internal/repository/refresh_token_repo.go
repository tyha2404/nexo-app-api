package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrRefreshTokenReused is returned by Rotate when the presented token was
// already rotated or revoked. Its whole family has been revoked by then.
var ErrRefreshTokenReused = errors.New("refresh token reused")

type RefreshTokenRepo interface {
	Create(ctx context.Context, token *model.RefreshToken) error
	// Rotate revokes the live token whose hash is tokenHash and stores next
	// as its successor (same user and family), in one DB transaction. next
	// only needs TokenHash, ExpiresAt and UserAgent; UserID and FamilyID are
	// copied from the old token. It returns the old token.
	//
	// Errors: constant.ErrInvalidRefreshToken when the token is unknown or
	// expired; ErrRefreshTokenReused when it was already revoked, in which
	// case every token of its family is revoked (and that is committed).
	Rotate(ctx context.Context, tokenHash string, next *model.RefreshToken, now time.Time) (*model.RefreshToken, error)
	// RevokeFamilyByHash revokes the session the token belongs to. Unknown
	// tokens are ignored.
	RevokeFamilyByHash(ctx context.Context, tokenHash string, now time.Time) error
	// RevokeAllForUser signs the user out of every session.
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, now time.Time) error
	// DeleteExpiredForUser removes the user's expired rows. Revoked rows that
	// have not expired are kept: they are what detects reuse.
	DeleteExpiredForUser(ctx context.Context, userID uuid.UUID, now time.Time) error
}

type refreshTokenRepo struct {
	db *gorm.DB
}

func NewRefreshTokenRepo(db *gorm.DB) RefreshTokenRepo {
	return &refreshTokenRepo{db: db}
}

func (r *refreshTokenRepo) Create(ctx context.Context, token *model.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *refreshTokenRepo) Rotate(ctx context.Context, tokenHash string, next *model.RefreshToken, now time.Time) (*model.RefreshToken, error) {
	var old model.RefreshToken
	reused := false

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the row so two concurrent refreshes with the same token can't
		// both succeed: the second one sees it revoked.
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ?", tokenHash).
			First(&old).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return constant.ErrInvalidRefreshToken
		}
		if err != nil {
			return err
		}

		if old.RevokedAt != nil {
			reused = true
			return tx.Model(&model.RefreshToken{}).
				Where("family_id = ? AND revoked_at IS NULL", old.FamilyID).
				Update("revoked_at", now).Error
		}
		if !old.ExpiresAt.After(now) {
			return constant.ErrInvalidRefreshToken
		}

		next.UserID = old.UserID
		next.FamilyID = old.FamilyID
		if err := tx.Create(next).Error; err != nil {
			return err
		}
		return tx.Model(&model.RefreshToken{}).
			Where("id = ?", old.ID).
			Updates(map[string]any{"revoked_at": now, "replaced_by": next.ID}).Error
	})
	if err != nil {
		return nil, err
	}
	if reused {
		return nil, ErrRefreshTokenReused
	}
	return &old, nil
}

func (r *refreshTokenRepo) RevokeFamilyByHash(ctx context.Context, tokenHash string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.RefreshToken{}).
		Where("family_id = (SELECT family_id FROM refresh_tokens WHERE token_hash = ?) AND revoked_at IS NULL", tokenHash).
		Update("revoked_at", now).Error
}

func (r *refreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

func (r *refreshTokenRepo) DeleteExpiredForUser(ctx context.Context, userID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND expires_at <= ?", userID, now).
		Delete(&model.RefreshToken{}).Error
}
