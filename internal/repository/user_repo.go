package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type UserRepo interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	List(ctx context.Context, limit, offset int) ([]model.User, error)
	Update(ctx context.Context, user *model.User) error
	UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
}

type userRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) UserRepo {
	return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, constant.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) List(ctx context.Context, limit, offset int) ([]model.User, error) {
	var users []model.User
	err := r.db.WithContext(ctx).Limit(limit).Offset(offset).Find(&users).Error
	return users, err
}

func (r *userRepo) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepo) UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
}

func (r *userRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, "id = ?", id).Error
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, constant.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, constant.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}
