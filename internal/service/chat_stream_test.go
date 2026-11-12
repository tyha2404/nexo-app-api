package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

// streamChatRepo is an in-memory, user-scoped ChatRepository.
type streamChatRepo struct {
	repository.ChatRepository
	mu           sync.Mutex
	sessions     map[uuid.UUID]*model.ChatSession
	messages     map[uuid.UUID]*model.ChatMessage
	order        []uuid.UUID
	createErr    error
	listLimit    int
	deleted      []uuid.UUID
	clearedUsers []uuid.UUID
}

func newStreamChatRepo() *streamChatRepo {
	return &streamChatRepo{sessions: map[uuid.UUID]*model.ChatSession{}, messages: map[uuid.UUID]*model.ChatMessage{}}
}

func (r *streamChatRepo) CreateSession(_ context.Context, s *model.ChatSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	s.ID = uuid.New()
	cp := *s
	r.sessions[s.ID] = &cp
	return nil
}

func (r *streamChatRepo) GetSessionByID(_ context.Context, userID, id uuid.UUID) (*model.ChatSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok || s.UserID != userID {
		return nil, constant.ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *streamChatRepo) ListSessionsByUserID(_ context.Context, userID uuid.UUID, limit int) ([]model.ChatSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listLimit = limit
	var out []model.ChatSession
	for _, s := range r.sessions {
		if s.UserID == userID {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (r *streamChatRepo) DeleteSession(_ context.Context, userID, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok || s.UserID != userID {
		return constant.ErrNotFound
	}
	delete(r.sessions, id)
	r.deleted = append(r.deleted, id)
	return nil
}

func (r *streamChatRepo) ClearUserSessions(_ context.Context, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.sessions {
		if s.UserID == userID {
			delete(r.sessions, id)
		}
	}
	r.clearedUsers = append(r.clearedUsers, userID)
	return nil
}

func (r *streamChatRepo) CreateMessage(_ context.Context, m *model.ChatMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m.ID = uuid.New()
	cp := *m
	r.messages[m.ID] = &cp
	r.order = append(r.order, m.ID)
	return nil
}

func (r *streamChatRepo) UpdateMessage(_ context.Context, _, id uuid.UUID, content string, status model.ChatMessageStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.messages[id]
	if !ok {
		return constant.ErrNotFound
	}
	m.Content, m.Status = content, status
	return nil
}

func (r *streamChatRepo) UpdateMessageToolResults(_ context.Context, _, id uuid.UUID, toolResults *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.messages[id]
	if !ok {
		return constant.ErrNotFound
	}
	m.ToolResults = toolResults
	return nil
}

func (r *streamChatRepo) GetMessageByID(_ context.Context, _, id uuid.UUID) (*model.ChatMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.messages[id]
	if !ok {
		return nil, constant.ErrNotFound
	}
	cp := *m
	return &cp, nil
}

func (r *streamChatRepo) ListRecentMessagesBySessionID(_ context.Context, sessionID uuid.UUID, limit int) ([]model.ChatMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.ChatMessage
	for _, id := range r.order {
		if m := r.messages[id]; m.SessionID == sessionID {
			out = append(out, *m)
		}
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

func (r *streamChatRepo) sessionMessages(sessionID uuid.UUID) []model.ChatMessage {
	msgs, _ := r.ListRecentMessagesBySessionID(context.Background(), sessionID, 1000)
	return msgs
}

// fakeNineRouter is a scripted model gateway; it never touches the network.
type fakeNineRouter struct {
	mu             sync.Mutex
	configured     bool
	chatResp       *NineRouterChatResponse
	chatErr        error
	streamChunks   []string
	streamErr      error
	models         []string
	chatModel      string
	chatMessages   []NineRouterMessage
	chatTools      []ToolDefinition
	streamModel    string
	streamMessages []NineRouterMessage
	streamCalls    int
}

func (f *fakeNineRouter) IsConfigured() bool { return f.configured }
func (f *fakeNineRouter) ListModels(context.Context) ([]string, error) {
	return f.models, nil
}
func (f *fakeNineRouter) ChatCompletionWithModel(_ context.Context, model string, msgs []NineRouterMessage, tools []ToolDefinition) (*NineRouterChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chatModel, f.chatMessages, f.chatTools = model, append([]NineRouterMessage(nil), msgs...), tools
	return f.chatResp, f.chatErr
}
func (f *fakeNineRouter) StreamChatCompletionsWithModel(_ context.Context, model string, msgs []NineRouterMessage, onChunk func(string) error) error {
	f.mu.Lock()
	f.streamModel, f.streamMessages = model, append([]NineRouterMessage(nil), msgs...)
	f.streamCalls++
	chunks, err := f.streamChunks, f.streamErr
	f.mu.Unlock()
	for _, c := range chunks {
		if cbErr := onChunk(c); cbErr != nil {
			return cbErr
		}
	}
	return err
}
func (f *fakeNineRouter) EmbedText(context.Context, string) ([]float32, error) {
	return nil, errors.New("not used")
}

// chatResponse builds a gateway response (the type has anonymous structs).
func chatResponse(t *testing.T, content string, calls ...ToolCall) *NineRouterChatResponse {
	t.Helper()
	payload := map[string]interface{}{"choices": []interface{}{map[string]interface{}{
		"message":       map[string]interface{}{"role": "assistant", "content": content, "tool_calls": calls},
		"finish_reason": "stop",
	}}}
	var resp NineRouterChatResponse
	require.NoError(t, json.Unmarshal([]byte(mustJSON(t, payload)), &resp))
	return &resp
}

func toolCall(t *testing.T, id, name string, args map[string]interface{}) ToolCall {
	return ToolCall{ID: id, Type: "function", Function: ToolCallFunction{Name: name, Arguments: mustJSON(t, args)}}
}

type streamEnv struct {
	*toolEnv
	repo *streamChatRepo
	ai   *fakeNineRouter
}

func newStreamEnv(t *testing.T) *streamEnv {
	e := &streamEnv{toolEnv: newToolEnv(t), repo: newStreamChatRepo(), ai: &fakeNineRouter{configured: true}}
	e.svc.chatRepo = e.repo
	e.svc.nineRouterService = e.ai
	return e
}

func (e *streamEnv) send(t *testing.T, req dto.SendMessageRequest) ([]dto.ChatStreamEvent, error) {
	t.Helper()
	ch := make(chan dto.ChatStreamEvent, 1024)
	err := e.svc.ProcessMessageStream(context.Background(), e.userID, req, ch)
	var events []dto.ChatStreamEvent
	for ev := range ch { // the service closes the channel
		events = append(events, ev)
	}
	return events, err
}

func eventsOfType(events []dto.ChatStreamEvent, typ string) []dto.ChatStreamEvent {
	var out []dto.ChatStreamEvent
	for _, ev := range events {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

func joinedDeltas(events []dto.ChatStreamEvent) string {
	var b strings.Builder
	for _, ev := range eventsOfType(events, "text_delta") {
		b.WriteString(ev.Delta)
	}
	return b.String()
}

func TestProcessMessageStream_NotConfiguredFallsBack(t *testing.T) {
	e := newStreamEnv(t)
	e.ai.configured = false
	long := strings.Repeat("Tiết kiệm ", 10) // 100 runes

	events, err := e.send(t, dto.SendMessageRequest{Message: long})
	require.NoError(t, err)
	require.Len(t, events, 3)
	assert.Equal(t, "session_info", events[0].Type)
	assert.Equal(t, "text_delta", events[1].Type)
	assert.Contains(t, events[1].Delta, "Chưa cấu hình API Key")
	assert.Equal(t, "done", events[2].Type)

	sessionID := *events[0].SessionID
	sess, err := e.repo.GetSessionByID(context.Background(), e.userID, sessionID)
	require.NoError(t, err)
	assert.Equal(t, string([]rune(strings.TrimSpace(long))[:40])+"...", sess.Title, "title is the first 40 runes")

	msgs := e.repo.sessionMessages(sessionID)
	require.Len(t, msgs, 2)
	assert.Equal(t, model.ChatRoleUser, msgs[0].Role)
	assert.Equal(t, long, msgs[0].Content)
	assert.Equal(t, model.ChatRoleModel, msgs[1].Role)
	assert.Equal(t, model.MessageStatusSuccess, msgs[1].Status)
	assert.Zero(t, e.ai.streamCalls)
}

func TestProcessMessageStream_OtherUsersSessionIsNotReused(t *testing.T) {
	e := newStreamEnv(t)
	e.ai.configured = false
	foreign := &model.ChatSession{UserID: uuid.New(), Title: "của người khác"}
	require.NoError(t, e.repo.CreateSession(context.Background(), foreign))

	events, err := e.send(t, dto.SendMessageRequest{SessionID: &foreign.ID, Message: "Xin chào"})
	require.NoError(t, err)
	got := *events[0].SessionID
	assert.NotEqual(t, foreign.ID, got, "a session of another user must never be written to")
	assert.Empty(t, e.repo.sessionMessages(foreign.ID))
	sess, err := e.repo.GetSessionByID(context.Background(), e.userID, got)
	require.NoError(t, err)
	assert.Equal(t, "Xin chào", sess.Title)

	// The user's own session is reused.
	events, err = e.send(t, dto.SendMessageRequest{SessionID: &got, Message: "Lần 2"})
	require.NoError(t, err)
	assert.Equal(t, got, *events[0].SessionID)
	assert.Len(t, e.repo.sessionMessages(got), 4)
}

func TestProcessMessageStream_CreateSessionFailure(t *testing.T) {
	e := newStreamEnv(t)
	e.repo.createErr = errInternalRepo
	events, err := e.send(t, dto.SendMessageRequest{Message: "hi"})
	require.ErrorIs(t, err, errInternalRepo)
	require.Len(t, events, 1)
	assert.Equal(t, "error", events[0].Type)
	assert.NotContains(t, events[0].ErrorMessage, "db down")
}

func TestProcessMessageStream_WriteToolIsOnlyProposed(t *testing.T) {
	e := newStreamEnv(t)
	e.addCategory("Ăn uống", model.CategoryTypeExpense)
	e.wallet.summary = &dto.WalletSummaryResponse{TotalWallets: 5}
	e.ai.chatResp = chatResponse(t, "",
		toolCall(t, "call-w", "create_transaction", map[string]interface{}{"amount": 50_000.0, "categoryName": "Ăn uống", "walletName": "tiền mặt"}),
		toolCall(t, "call-r", "list_wallets", map[string]interface{}{}),
	)
	modelName := "gpt-test"

	events, err := e.send(t, dto.SendMessageRequest{Message: "vừa ăn phở 50k", Model: &modelName})
	require.NoError(t, err)

	assert.Empty(t, e.txnSvc.created, "a write tool never executes from the stream")
	assert.Equal(t, "gpt-test", e.ai.chatModel)
	assert.Equal(t, "gpt-test", e.ai.streamModel)
	assert.Len(t, eventsOfType(events, "tool_start"), 2)
	assert.Len(t, eventsOfType(events, "tool_done"), 2)
	cards := eventsOfType(events, "action_card")
	require.Len(t, cards, 1)
	assert.Equal(t, ActionTypePendingConfirmation, cards[0].ActionCard.ActionType)

	// The model sees the assistant tool_calls message and one tool message per call.
	msgs := e.ai.streamMessages
	require.GreaterOrEqual(t, len(msgs), 4)
	assistant := msgs[len(msgs)-3]
	assert.Equal(t, "assistant", assistant.Role)
	assert.Len(t, assistant.ToolCalls, 2)
	assert.Equal(t, "tool", msgs[len(msgs)-2].Role)
	assert.Equal(t, "call-w", msgs[len(msgs)-2].ToolCallID)
	assert.Contains(t, msgs[len(msgs)-2].Content, "pending_confirmation")
	assert.Equal(t, "call-r", msgs[len(msgs)-1].ToolCallID)
	assert.Contains(t, msgs[len(msgs)-1].Content, `"totalWallets":5`)

	// The stream answered nothing: a fallback asks the user to confirm.
	text := joinedDeltas(events)
	assert.Contains(t, text, "Xác nhận")
	last := events[len(events)-1]
	assert.Equal(t, "done", last.Type)

	ai, err := e.repo.GetMessageByID(context.Background(), e.userID, *last.MessageID)
	require.NoError(t, err)
	assert.Equal(t, model.MessageStatusSuccess, ai.Status)
	assert.Equal(t, text, ai.Content)
	require.NotNil(t, ai.ToolResults, "the proposal card is stored with the AI message")
	var stored []dto.ActionCard
	require.NoError(t, json.Unmarshal([]byte(*ai.ToolResults), &stored))
	require.Len(t, stored, 1)
	assert.Equal(t, "PENDING", stored[0].Data.(map[string]interface{})["status"])

	// System prompt carries today's date in the app timezone; history includes the user message.
	require.NotEmpty(t, e.ai.chatMessages)
	assert.Equal(t, "system", e.ai.chatMessages[0].Role)
	assert.Contains(t, e.ai.chatMessages[0].Content, "Hôm nay là: "+util.Now().Format("2006-01-02"))
	assert.Equal(t, "vừa ăn phở 50k", e.ai.chatMessages[len(e.ai.chatMessages)-1].Content)
}

func TestProcessMessageStream_StreamsAnswerWithoutThinking(t *testing.T) {
	e := newStreamEnv(t)
	e.ai.chatResp = chatResponse(t, "", toolCall(t, "c1", "list_wallets", nil))
	e.ai.streamChunks = []string{"<think>nội bộ</think>", "Bạn có ", "5 ví."}

	events, err := e.send(t, dto.SendMessageRequest{Message: "tôi có bao nhiêu ví"})
	require.NoError(t, err)
	assert.Equal(t, "Bạn có 5 ví.", joinedDeltas(events))
	assert.NotContains(t, joinedDeltas(events), "nội bộ")
	last := events[len(events)-1]
	ai, _ := e.repo.GetMessageByID(context.Background(), e.userID, *last.MessageID)
	assert.Equal(t, "Bạn có 5 ví.", ai.Content)
	assert.Nil(t, ai.ToolResults, "no cards, nothing stored")
}

func TestProcessMessageStream_DirectAnswerAndEmptyAnswer(t *testing.T) {
	e := newStreamEnv(t)
	e.ai.chatResp = chatResponse(t, "<think>plan</think>Xin chào bạn")
	events, err := e.send(t, dto.SendMessageRequest{Message: "hi"})
	require.NoError(t, err)
	assert.Equal(t, "Xin chào bạn", joinedDeltas(events))
	assert.Zero(t, e.ai.streamCalls, "a direct answer is not streamed again")
	assert.Equal(t, "done", events[len(events)-1].Type)

	// No tool calls and no content: fall back to streaming.
	e2 := newStreamEnv(t)
	e2.ai.chatResp = chatResponse(t, "")
	e2.ai.streamChunks = []string{"Đây là ", "câu trả lời"}
	events, err = e2.send(t, dto.SendMessageRequest{Message: "hi"})
	require.NoError(t, err)
	assert.Equal(t, 1, e2.ai.streamCalls)
	assert.Equal(t, "Đây là câu trả lời", joinedDeltas(events))
}

func TestProcessMessageStream_UpstreamErrorsAreNotLeaked(t *testing.T) {
	e := newStreamEnv(t)
	e.ai.chatErr = errors.New("POST http://10.0.0.5:20128/v1/chat/completions: 401 invalid key sk-secret")
	e.ai.streamChunks = []string{"Một phần"}
	e.ai.streamErr = errors.New("upstream reset by http://10.0.0.5")

	events, err := e.send(t, dto.SendMessageRequest{Message: "hi"})
	require.NoError(t, err)
	assert.Equal(t, 1, e.ai.streamCalls, "a failed tool-call request falls back to streaming")
	last := events[len(events)-1]
	assert.Equal(t, "error", last.Type)
	assert.Equal(t, "ERROR", last.Status)
	assert.NotContains(t, last.ErrorMessage, "10.0.0.5")
	assert.NotContains(t, last.ErrorMessage, "sk-secret")

	ai, err := e.repo.GetMessageByID(context.Background(), e.userID, *last.MessageID)
	require.NoError(t, err)
	assert.Equal(t, model.MessageStatusError, ai.Status)
	assert.True(t, strings.HasPrefix(ai.Content, "Một phần"), "partial answer is kept: %q", ai.Content)
	assert.NotContains(t, ai.Content, "10.0.0.5")
}

func TestProcessMessageStream_CapsToolCallsAndHidesToolErrors(t *testing.T) {
	e := newStreamEnv(t)
	e.report.summaryErr = errors.New("pq: connection refused")
	var calls []ToolCall
	for i := 0; i < maxToolCallsPerTurn+3; i++ {
		calls = append(calls, toolCall(t, "c"+string(rune('a'+i)), "get_financial_overview", nil))
	}
	e.ai.chatResp = chatResponse(t, "", calls...)
	e.ai.streamChunks = []string{"ok"}

	events, err := e.send(t, dto.SendMessageRequest{Message: "tổng quan"})
	require.NoError(t, err)
	assert.Len(t, eventsOfType(events, "tool_start"), maxToolCallsPerTurn)

	var toolMsgs []NineRouterMessage
	for _, m := range e.ai.streamMessages {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			assert.Len(t, m.ToolCalls, maxToolCallsPerTurn, "every tool_call sent upstream gets an answer")
		}
		if m.Role == "tool" {
			toolMsgs = append(toolMsgs, m)
		}
	}
	require.Len(t, toolMsgs, maxToolCallsPerTurn)
	for _, m := range toolMsgs {
		assert.Contains(t, m.Content, hiddenSystemError)
		assert.NotContains(t, m.Content, "pq:")
	}
}

func TestProcessMessageStream_AutoInjectsKnowledge(t *testing.T) {
	e := newStreamEnv(t)
	e.rag.results = []KnowledgeSearchResult{
		{Topic: "budget", Title: "Quy tắc 50/30/20", Content: "Chia thu nhập 50/30/20", Score: 0.82},
		{Topic: "weak", Title: "Yếu", Content: "KHÔNG_DÙNG_1", Score: 0.1},
		{Topic: "fb", Title: "Fallback", Content: "KHÔNG_DÙNG_2", Score: 0.99, Fallback: true},
	}
	e.ai.chatResp = chatResponse(t, "Trả lời")

	events, err := e.send(t, dto.SendMessageRequest{Message: "nên chia lương thế nào"})
	require.NoError(t, err)
	cards := eventsOfType(events, "action_card")
	require.Len(t, cards, 1)
	assert.Equal(t, "KNOWLEDGE_SOURCE", cards[0].ActionCard.ActionType)
	assert.Equal(t, "auto_inject", cards[0].ActionCard.Data.(map[string]interface{})["source"])
	assert.Equal(t, []int{ragAutoInjectTopK}, e.rag.limits)

	system := e.ai.chatMessages[0].Content
	assert.Contains(t, system, "Chia thu nhập 50/30/20")
	assert.NotContains(t, system, "KHÔNG_DÙNG_1", "weak matches are not injected")
	assert.NotContains(t, system, "KHÔNG_DÙNG_2", "fallback matches are not injected")
	for _, tool := range e.ai.chatTools {
		assert.NotEqual(t, "search_financial_knowledge", tool.Function.Name, "retrieval tool hidden after auto-inject")
	}

	// RAG failure: no card, retrieval tool still offered.
	e2 := newStreamEnv(t)
	e2.rag.err = errInternalRepo
	e2.ai.chatResp = chatResponse(t, "ok")
	events, err = e2.send(t, dto.SendMessageRequest{Message: "hi"})
	require.NoError(t, err)
	assert.Empty(t, eventsOfType(events, "action_card"))
	assert.Len(t, e2.ai.chatTools, len(GetFinancialToolDefinitions()))
}

func TestAutoInjectKnowledge_Guards(t *testing.T) {
	e := newToolEnv(t)
	prompt := "base"
	assert.Nil(t, e.svc.autoInjectKnowledge(context.Background(), "   ", &prompt, uuid.New()))
	assert.Empty(t, e.rag.queries, "blank messages never hit RAG")

	e.svc.ragService = nil
	assert.Nil(t, e.svc.autoInjectKnowledge(context.Background(), "hello", &prompt, uuid.New()))
	assert.Equal(t, "base", prompt)
}

func TestWithoutKnowledgeSearchToolAndCard(t *testing.T) {
	all := GetFinancialToolDefinitions()
	filtered := withoutKnowledgeSearchTool(all)
	assert.Len(t, filtered, len(all)-1)
	for _, d := range filtered {
		assert.NotEqual(t, "search_financial_knowledge", d.Function.Name)
	}
	assert.Empty(t, withoutKnowledgeSearchTool(nil))

	card := knowledgeAutoInjectCard("lãi kép", []KnowledgeSearchResult{{Title: "a"}, {Title: "b"}})
	assert.Equal(t, "KNOWLEDGE_SOURCE", card.ActionType)
	assert.Contains(t, card.Description, "2 tài liệu")
	assert.Contains(t, card.Description, "lãi kép")
}

func TestChatSessionsCRUD(t *testing.T) {
	e := newStreamEnv(t)
	ctx := context.Background()

	s, err := e.svc.CreateSession(ctx, e.userID, "")
	require.NoError(t, err)
	assert.Equal(t, "Cuộc trò chuyện mới", s.Title, "default title")
	named, err := e.svc.CreateSession(ctx, e.userID, "Ngân sách tháng 11")
	require.NoError(t, err)
	assert.Equal(t, "Ngân sách tháng 11", named.Title)
	other := uuid.New()
	_, err = e.svc.CreateSession(ctx, other, "theirs")
	require.NoError(t, err)

	list, err := e.svc.ListSessions(ctx, e.userID, 20)
	require.NoError(t, err)
	assert.Len(t, list, 2, "only the user's own sessions")
	assert.Equal(t, 20, e.repo.listLimit)

	empty, err := e.svc.ListSessions(ctx, uuid.New(), 20)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)

	require.ErrorIs(t, e.svc.DeleteSession(ctx, other, s.ID), constant.ErrNotFound, "cannot delete another user's session")
	require.NoError(t, e.svc.DeleteSession(ctx, e.userID, s.ID))
	assert.Equal(t, []uuid.UUID{s.ID}, e.repo.deleted)

	require.NoError(t, e.svc.ClearSessions(ctx, e.userID))
	list, _ = e.svc.ListSessions(ctx, e.userID, 20)
	assert.Empty(t, list)
	list, _ = e.svc.ListSessions(ctx, other, 20)
	assert.Len(t, list, 1, "clearing never touches other users")

	e.repo.createErr = errInternalRepo
	_, err = e.svc.CreateSession(ctx, e.userID, "x")
	require.ErrorIs(t, err, errInternalRepo)

	e.ai.models = []string{"m1", "m2"}
	models, err := e.svc.ListAvailableModels(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"m1", "m2"}, models)
}
