package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type chatMessagesStub struct {
	repository.ChatRepository
	session    *model.ChatSession
	messages   []model.ChatMessage // chronological
	recentUsed bool
}

func (r *chatMessagesStub) GetSessionByID(_ context.Context, _, _ uuid.UUID) (*model.ChatSession, error) {
	return r.session, nil
}

func (r *chatMessagesStub) ListRecentMessagesBySessionID(_ context.Context, _ uuid.UUID, limit int) ([]model.ChatMessage, error) {
	r.recentUsed = true
	if len(r.messages) <= limit {
		return r.messages, nil
	}
	return r.messages[len(r.messages)-limit:], nil
}

func TestGetSessionMessagesReturnsNewestInChronologicalOrder(t *testing.T) {
	userID, sessionID := uuid.New(), uuid.New()
	base := time.Now().Add(-time.Hour)
	var msgs []model.ChatMessage
	for i := 0; i < 60; i++ {
		msgs = append(msgs, model.ChatMessage{ID: uuid.New(), SessionID: sessionID, Role: "user", Content: string(rune('a' + i%26)), CreatedAt: base.Add(time.Duration(i) * time.Minute)})
	}
	repo := &chatMessagesStub{session: &model.ChatSession{ID: sessionID, UserID: userID}, messages: msgs}
	svc := service.NewChatService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())

	res, err := svc.GetSessionMessages(context.Background(), userID, sessionID, 50)
	require.NoError(t, err)
	require.True(t, repo.recentUsed)
	require.Len(t, res.Messages, 50)
	require.Equal(t, msgs[10].ID, res.Messages[0].ID, "oldest of the newest 50")
	require.Equal(t, msgs[59].ID, res.Messages[49].ID, "newest last")
}
