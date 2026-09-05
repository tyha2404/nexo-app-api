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
	Upsert(ctx context.Context, sub *model.PushSubscription) error
	DeleteByEndpoint(ctx context.Context, endpoint string) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.PushSubscription, error)
	ListAll(ctx context.Context) ([]model.PushSubscription, error)
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
		Columns: []clause.Column{{Name: "endpoint"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_id",
			"p256dh",
			"auth",
			"user_agent",
			"device_type",
			"updated_at",
		}),
	}).Create(sub).Error
}

func (r *pushSubscriptionRepository) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	return r.db.WithContext(ctx).Where("endpoint = ?", endpoint).Delete(&model.PushSubscription{}).Error
}

func (r *pushSubscriptionRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.PushSubscription{}).Error
}

func (r *pushSubscriptionRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.PushSubscription, error) {
	var subs []model.PushSubscription
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&subs).Error
	return subs, err
}

func (r *pushSubscriptionRepository) ListAll(ctx context.Context) ([]model.PushSubscription, error) {
	var subs []model.PushSubscription
	err := r.db.WithContext(ctx).Find(&subs).Error
	return subs, err
}
