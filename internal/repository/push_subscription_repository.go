package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PushSubscriptionRepository interface {
	// Upsert inserts the subscription or, when the endpoint already exists,
	// moves it to sub.UserID and refreshes its keys.
	Upsert(ctx context.Context, sub *model.PushSubscription) error
	// DeleteForUser removes the user's subscription with that endpoint.
	DeleteForUser(ctx context.Context, userID uuid.UUID, endpoint string) error
	// DeleteByEndpoint removes a subscription the push service reported gone (404/410).
	DeleteByEndpoint(ctx context.Context, endpoint string) error
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.PushSubscription, error)
}

type pushSubscriptionRepository struct {
	db *gorm.DB
}

func NewPushSubscriptionRepository(db *gorm.DB) PushSubscriptionRepository {
	return &pushSubscriptionRepository{db: db}
}

func (r *pushSubscriptionRepository) Upsert(ctx context.Context, sub *model.PushSubscription) error {
	sub.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "endpoint"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "p256dh", "auth", "user_agent", "device_type", "updated_at"}),
	}).Create(sub).Error
}

func (r *pushSubscriptionRepository) DeleteForUser(ctx context.Context, userID uuid.UUID, endpoint string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND endpoint = ?", userID, endpoint).
		Delete(&model.PushSubscription{}).Error
}

func (r *pushSubscriptionRepository) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	return r.db.WithContext(ctx).
		Where("endpoint = ?", endpoint).
		Delete(&model.PushSubscription{}).Error
}

func (r *pushSubscriptionRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.PushSubscription, error) {
	var subs []model.PushSubscription
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("updated_at DESC").
		Find(&subs).Error
	return subs, err
}
