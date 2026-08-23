package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ChatRole string

const (
	ChatRoleUser   ChatRole = "user"
	ChatRoleModel  ChatRole = "model"
	ChatRoleSystem ChatRole = "system"
)

type ChatSession struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	Title     string    `gorm:"type:varchar(255);not null;default:'Đoạn chat mới'" json:"title"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt DeletedAt `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`

	User     *User         `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user,omitempty"`
	Messages []ChatMessage `gorm:"foreignKey:SessionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"messages,omitempty"`
}

func (s *ChatSession) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID, err = uuid.NewV7()
	}
	return err
}

type ChatMessageStatus string

const (
	MessageStatusPending   ChatMessageStatus = "PENDING"
	MessageStatusStreaming ChatMessageStatus = "STREAMING"
	MessageStatusSuccess   ChatMessageStatus = "SUCCESS"
	MessageStatusError     ChatMessageStatus = "ERROR"
)

type ChatMessage struct {
	ID          uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	SessionID   uuid.UUID         `gorm:"type:uuid;not null;index" json:"sessionId"`
	Role        ChatRole          `gorm:"type:varchar(20);not null" json:"role"`
	Content     string            `gorm:"type:text;not null" json:"content"`
	Status      ChatMessageStatus `gorm:"type:varchar(20);not null;default:'SUCCESS'" json:"status"`
	ToolCalls   *string           `gorm:"type:jsonb" json:"toolCalls,omitempty"`
	ToolResults *string           `gorm:"type:jsonb" json:"toolResults,omitempty"`
	CreatedAt   time.Time         `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time         `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt   DeletedAt         `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`

	Session *ChatSession `gorm:"foreignKey:SessionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

func (m *ChatMessage) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == uuid.Nil {
		m.ID, err = uuid.NewV7()
	}
	return err
}

type FinancialKnowledge struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Topic     string    `gorm:"type:varchar(100);not null;index" json:"topic"`
	Title     string    `gorm:"type:varchar(255);not null" json:"title"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	Embedding string    `gorm:"type:text;not null" json:"embedding"` // JSON string representation of []float32
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt DeletedAt `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`
}

func (k *FinancialKnowledge) BeforeCreate(tx *gorm.DB) (err error) {
	if k.ID == uuid.Nil {
		k.ID, err = uuid.NewV7()
	}
	return err
}
