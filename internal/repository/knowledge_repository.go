package repository

import (
	"context"

	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type KnowledgeRepository interface {
	ListAll(ctx context.Context) ([]model.FinancialKnowledge, error)
	ReplaceByTopic(ctx context.Context, topic string, docs []*model.FinancialKnowledge) error
}

type knowledgeRepository struct {
	db *gorm.DB
}

func NewKnowledgeRepository(db *gorm.DB) KnowledgeRepository {
	return &knowledgeRepository{db: db}
}

// ReplaceByTopic atomically deletes all existing chunks of a topic and inserts
// the new set in a single transaction, so readers never observe a partial topic.
// Knowledge chunks are re-seeded from internal/data/knowledge, not user data, so
// they are hard-deleted (Unscoped) to avoid piling up soft-deleted embeddings on
// every sync.
func (r *knowledgeRepository) ReplaceByTopic(ctx context.Context, topic string, docs []*model.FinancialKnowledge) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("topic = ?", topic).Delete(&model.FinancialKnowledge{}).Error; err != nil {
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
