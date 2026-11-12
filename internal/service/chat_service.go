package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

type ChatService interface {
	CreateSession(ctx context.Context, userID uuid.UUID, title string) (*dto.ChatSessionResponse, error)
	ListSessions(ctx context.Context, userID uuid.UUID, limit int) ([]dto.ChatSessionResponse, error)
	GetSessionMessages(ctx context.Context, userID, sessionID uuid.UUID, limit int) (*dto.ChatSessionResponse, error)
	DeleteSession(ctx context.Context, userID, sessionID uuid.UUID) error
	ClearSessions(ctx context.Context, userID uuid.UUID) error
	ListAvailableModels(ctx context.Context) ([]string, error)
	ProcessMessageStream(ctx context.Context, userID uuid.UUID, req dto.SendMessageRequest, eventChan chan<- dto.ChatStreamEvent) error
	// ConfirmAction executes an AI write proposal (same user only, once).
	ConfirmAction(ctx context.Context, userID, actionID uuid.UUID) (*dto.ChatActionResult, error)
	// CancelAction discards an AI write proposal.
	CancelAction(ctx context.Context, userID, actionID uuid.UUID) (*dto.ChatActionResult, error)
}

type chatService struct {
	chatRepo           repository.ChatRepository
	nineRouterService  NineRouterService
	ragService         RAGService
	transactionService TransactionService
	reportService      ReportService
	budgetService      BudgetService
	debtService        DebtService
	walletService      WalletService
	targetService      TargetService
	categoryRepo       repository.CategoryRepo
	transactionRepo    repository.TransactionRepository
	walletRepo         repository.WalletRepository
	logger             *zap.Logger
	pendingActions     PendingActionStore
	historyMu          sync.Mutex
	now                func() time.Time
}

// ChatServiceOption configures optional collaborators of the chat service.
type ChatServiceOption func(*chatService)

// WithPendingActionStore replaces the default in-memory proposal store.
func WithPendingActionStore(store PendingActionStore) ChatServiceOption {
	return func(s *chatService) { s.pendingActions = store }
}

// WithChatClock overrides the clock (tests).
func WithChatClock(now func() time.Time) ChatServiceOption {
	return func(s *chatService) { s.now = now }
}

func NewChatService(
	chatRepo repository.ChatRepository,
	nineRouterService NineRouterService,
	ragService RAGService,
	transactionService TransactionService,
	reportService ReportService,
	budgetService BudgetService,
	debtService DebtService,
	walletService WalletService,
	targetService TargetService,
	categoryRepo repository.CategoryRepo,
	transactionRepo repository.TransactionRepository,
	walletRepo repository.WalletRepository,
	logger *zap.Logger,
	opts ...ChatServiceOption,
) ChatService {
	s := &chatService{
		chatRepo:           chatRepo,
		nineRouterService:  nineRouterService,
		ragService:         ragService,
		transactionService: transactionService,
		reportService:      reportService,
		budgetService:      budgetService,
		debtService:        debtService,
		walletService:      walletService,
		targetService:      targetService,
		categoryRepo:       categoryRepo,
		transactionRepo:    transactionRepo,
		walletRepo:         walletRepo,
		logger:             logger,
		now:                util.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.pendingActions == nil {
		s.pendingActions = NewInMemoryPendingActionStore(PendingActionTTL, time.Now)
	}
	if s.logger == nil {
		s.logger = zap.NewNop()
	}
	return s
}

func (s *chatService) CreateSession(ctx context.Context, userID uuid.UUID, title string) (*dto.ChatSessionResponse, error) {
	if title == "" {
		title = "Cuộc trò chuyện mới"
	}
	session := &model.ChatSession{
		UserID: userID,
		Title:  title,
	}
	if err := s.chatRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	res := toChatSessionResponse(session)
	return &res, nil
}

// toChatSessionResponse maps a session without its messages.
func toChatSessionResponse(sess *model.ChatSession) dto.ChatSessionResponse {
	return dto.ChatSessionResponse{
		ID:        sess.ID,
		Title:     sess.Title,
		CreatedAt: sess.CreatedAt,
		UpdatedAt: sess.UpdatedAt,
	}
}

func (s *chatService) ListSessions(ctx context.Context, userID uuid.UUID, limit int) ([]dto.ChatSessionResponse, error) {
	sessions, err := s.chatRepo.ListSessionsByUserID(ctx, userID, limit)
	if err != nil {
		return nil, err
	}

	res := make([]dto.ChatSessionResponse, 0, len(sessions))
	for i := range sessions {
		res = append(res, toChatSessionResponse(&sessions[i]))
	}
	return res, nil
}

func (s *chatService) GetSessionMessages(ctx context.Context, userID, sessionID uuid.UUID, limit int) (*dto.ChatSessionResponse, error) {
	sess, err := s.chatRepo.GetSessionByID(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	// Newest `limit` messages, returned oldest-first (chronological) so the UI
	// shows the latest conversation; limit <= 0 returns the whole session.
	var messages []model.ChatMessage
	if limit > 0 {
		messages, err = s.chatRepo.ListRecentMessagesBySessionID(ctx, sessionID, limit)
	} else {
		messages, err = s.chatRepo.ListMessagesBySessionID(ctx, sessionID, 0)
	}
	if err != nil {
		return nil, err
	}

	msgResponses := make([]dto.ChatMessageResponse, 0, len(messages))
	for _, m := range messages {
		msgResponses = append(msgResponses, dto.ChatMessageResponse{
			ID:          m.ID,
			SessionID:   m.SessionID,
			Role:        string(m.Role),
			Content:     m.Content,
			Status:      string(m.Status),
			ToolCalls:   m.ToolCalls,
			ToolResults: m.ToolResults,
			CreatedAt:   m.CreatedAt,
		})
	}

	res := toChatSessionResponse(sess)
	res.Messages = msgResponses
	return &res, nil
}

func (s *chatService) DeleteSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	return s.chatRepo.DeleteSession(ctx, userID, sessionID)
}

func (s *chatService) ClearSessions(ctx context.Context, userID uuid.UUID) error {
	return s.chatRepo.ClearUserSessions(ctx, userID)
}

const (
	// ragAutoInjectTopK is the number of knowledge docs auto-injected per turn.
	ragAutoInjectTopK = 2
	// ragAutoInjectMinScore filters out weak semantic matches; fallback-flagged
	// results are always excluded regardless of this threshold.
	ragAutoInjectMinScore = 0.30
	// maxToolCallsPerTurn caps how many tool calls of one model turn are
	// executed, bounding DB work and proposals per chat message.
	maxToolCallsPerTurn = 5
)

// autoInjectKnowledge grounds the system prompt with internal knowledge that
// semantically matches the user's message, avoiding a tool round-trip for
// general financial-advice questions. Failures never block the chat flow.
// Returns the injected docs so the caller can render a citation card.
func (s *chatService) autoInjectKnowledge(ctx context.Context, userMessage string, systemPrompt *string, sessionID uuid.UUID) []KnowledgeSearchResult {
	if s.ragService == nil || strings.TrimSpace(userMessage) == "" {
		return nil
	}

	results, err := s.ragService.SearchKnowledge(ctx, userMessage, ragAutoInjectTopK)
	if err != nil {
		s.logger.Warn("rag auto-inject search failed", zap.Error(err))
		return nil
	}

	confident := make([]KnowledgeSearchResult, 0, len(results))
	for _, r := range results {
		if !r.Fallback && r.Score >= ragAutoInjectMinScore {
			confident = append(confident, r)
		}
	}
	if len(confident) == 0 {
		return nil
	}

	var kb strings.Builder
	kb.WriteString("\n\nKIẾN THỨC NỘI BỘ LIÊN QUAN (tra cứu tự động theo câu hỏi, ưu tiên dùng nếu phù hợp):\n")
	for _, r := range confident {
		fmt.Fprintf(&kb, "- [%s] %s\n%s\n", r.Topic, r.Title, r.Content)
	}

	*systemPrompt += kb.String()
	s.logger.Info("rag auto-injected knowledge into system prompt",
		zap.String("session_id", sessionID.String()),
		zap.Int("docs", len(confident)),
	)
	return confident
}

// knowledgeAutoInjectCard builds the citation card shown for silently
// auto-injected knowledge, mirroring the tool's KNOWLEDGE_SOURCE card.
func knowledgeAutoInjectCard(query string, docs []KnowledgeSearchResult) *dto.ActionCard {
	return &dto.ActionCard{
		ActionType:  "KNOWLEDGE_SOURCE",
		Title:       "Nguồn tri thức nội bộ",
		Description: fmt.Sprintf("Tự động tra cứu: %d tài liệu liên quan tới \"%s\"", len(docs), query),
		Data: map[string]interface{}{
			"query":   query,
			"results": docs,
			"source":  "auto_inject",
		},
	}
}

// withoutKnowledgeSearchTool removes the retrieval tool from the definitions
// for a turn where knowledge was already auto-injected, making the
// "don't re-lookup" rule structural instead of prompt-dependent.
func withoutKnowledgeSearchTool(defs []ToolDefinition) []ToolDefinition {
	filtered := make([]ToolDefinition, 0, len(defs))
	for _, d := range defs {
		if d.Function.Name == "search_financial_knowledge" {
			continue
		}
		filtered = append(filtered, d)
	}
	return filtered
}

const (
	// chatHistoryMaxMessages is how many stored messages of the session are
	// sent upstream as context.
	chatHistoryMaxMessages = 8
	// chatHistoryMaxMessageRunes truncates each history message.
	chatHistoryMaxMessageRunes = 4000
	// chatHistoryMaxTotalRunes caps the history sent upstream; the oldest
	// messages are dropped first.
	chatHistoryMaxTotalRunes = 16000
)

// buildUpstreamMessages builds the system prompt + capped conversation
// history (oldest → newest) sent to the model. skipID is the current AI
// placeholder message.
func buildUpstreamMessages(systemPrompt string, history []model.ChatMessage, skipID uuid.UUID) []NineRouterMessage {
	if len(history) > chatHistoryMaxMessages {
		history = history[len(history)-chatHistoryMaxMessages:]
	}

	kept := make([]NineRouterMessage, 0, len(history))
	total := 0
	// Walk newest → oldest so the latest turns survive the total budget.
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.ID == skipID || strings.TrimSpace(msg.Content) == "" {
			continue
		}
		content := msg.Content
		if runes := []rune(content); len(runes) > chatHistoryMaxMessageRunes {
			content = string(runes[:chatHistoryMaxMessageRunes]) + "…"
		}
		n := utf8.RuneCountInString(content)
		if total+n > chatHistoryMaxTotalRunes && len(kept) > 0 {
			break
		}
		total += n

		role := "user"
		if msg.Role == model.ChatRoleModel {
			role = "assistant"
		}
		kept = append(kept, NineRouterMessage{Role: role, Content: content})
	}

	out := make([]NineRouterMessage, 0, len(kept)+1)
	out = append(out, NineRouterMessage{Role: "system", Content: systemPrompt})
	for i := len(kept) - 1; i >= 0; i-- {
		out = append(out, kept[i])
	}
	return out
}

func (s *chatService) ListAvailableModels(ctx context.Context) ([]string, error) {
	return s.nineRouterService.ListModels(ctx)
}
