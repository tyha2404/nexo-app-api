package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type KnowledgeRepository interface {
	Create(ctx context.Context, knowledge *model.FinancialKnowledge) error
	ListAll(ctx context.Context) ([]model.FinancialKnowledge, error)
	Count(ctx context.Context) (int64, error)
	GetByTopic(ctx context.Context, topic string) ([]model.FinancialKnowledge, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.FinancialKnowledge, error)
	Update(ctx context.Context, knowledge *model.FinancialKnowledge) error
	DeleteByTopic(ctx context.Context, topic string) error
	ReplaceByTopic(ctx context.Context, topic string, docs []*model.FinancialKnowledge) error
}

type knowledgeRepository struct {
	db *gorm.DB
}

func NewKnowledgeRepository(db *gorm.DB) KnowledgeRepository {
	return &knowledgeRepository{db: db}
}

func (r *knowledgeRepository) Create(ctx context.Context, knowledge *model.FinancialKnowledge) error {
	return r.db.WithContext(ctx).Create(knowledge).Error
}

func (r *knowledgeRepository) Update(ctx context.Context, knowledge *model.FinancialKnowledge) error {
	return r.db.WithContext(ctx).Save(knowledge).Error
}

func (r *knowledgeRepository) DeleteByTopic(ctx context.Context, topic string) error {
	return r.db.WithContext(ctx).Where("topic = ?", topic).Delete(&model.FinancialKnowledge{}).Error
}

// ReplaceByTopic atomically deletes all existing chunks of a topic and inserts
// the new set in a single transaction, so readers never observe a partial topic.
func (r *knowledgeRepository) ReplaceByTopic(ctx context.Context, topic string, docs []*model.FinancialKnowledge) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("topic = ?", topic).Delete(&model.FinancialKnowledge{}).Error; err != nil {
			return err
		}
		if len(docs) == 0 {
			return nil
		}
		return tx.Create(&docs).Error
	})
}

func (r *knowledgeRepository) ListAll(ctx context.Context) ([]model.FinancialKnowledge, error) {
	var list []model.FinancialKnowledge
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}

func (r *knowledgeRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.FinancialKnowledge{}).Count(&count).Error
	return count, err
}

func (r *knowledgeRepository) GetByTopic(ctx context.Context, topic string) ([]model.FinancialKnowledge, error) {
	var list []model.FinancialKnowledge
	err := r.db.WithContext(ctx).Where("topic = ?", topic).Find(&list).Error
	return list, err
}

func (r *knowledgeRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.FinancialKnowledge, error) {
	var item model.FinancialKnowledge
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}
