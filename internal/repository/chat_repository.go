package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type ChatRepository interface {
	CreateSession(ctx context.Context, session *model.ChatSession) error
	GetSessionByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.ChatSession, error)
	ListSessionsByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]model.ChatSession, error)
	UpdateSessionTitle(ctx context.Context, id uuid.UUID, userID uuid.UUID, title string) error
	DeleteSession(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	ClearUserSessions(ctx context.Context, userID uuid.UUID) error

	CreateMessage(ctx context.Context, message *model.ChatMessage) error
	UpdateMessage(ctx context.Context, id uuid.UUID, content string, status model.ChatMessageStatus) error
	ListMessagesBySessionID(ctx context.Context, sessionID uuid.UUID, limit int) ([]model.ChatMessage, error)
}

type chatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) ChatRepository {
	return &chatRepository{db: db}
}

func (r *chatRepository) CreateSession(ctx context.Context, session *model.ChatSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *chatRepository) GetSessionByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.ChatSession, error) {
	var session model.ChatSession
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *chatRepository) ListSessionsByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]model.ChatSession, error) {
	var sessions []model.ChatSession
	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("updated_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&sessions).Error
	return sessions, err
}

func (r *chatRepository) UpdateSessionTitle(ctx context.Context, id uuid.UUID, userID uuid.UUID, title string) error {
	return r.db.WithContext(ctx).
		Model(&model.ChatSession{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("title", title).Error
}

func (r *chatRepository) DeleteSession(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.ChatSession{}).Error
}

func (r *chatRepository) ClearUserSessions(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&model.ChatSession{}).Error
}

func (r *chatRepository) CreateMessage(ctx context.Context, message *model.ChatMessage) error {
	return r.db.WithContext(ctx).Create(message).Error
}

func (r *chatRepository) UpdateMessage(ctx context.Context, id uuid.UUID, content string, status model.ChatMessageStatus) error {
	return r.db.WithContext(ctx).Model(&model.ChatMessage{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"content": content,
			"status":  status,
		}).Error
}

func (r *chatRepository) ListMessagesBySessionID(ctx context.Context, sessionID uuid.UUID, limit int) ([]model.ChatMessage, error) {
	var messages []model.ChatMessage
	query := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&messages).Error
	return messages, err
}
