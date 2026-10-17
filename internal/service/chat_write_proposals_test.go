package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"go.uber.org/zap"
)

type proposalCategoryRepo struct {
	repository.CategoryRepo
	categories []model.Category
	created    int
}

func (r *proposalCategoryRepo) List(_ context.Context, _ uuid.UUID, categoryType string, _, _ int) ([]model.Category, error) {
	var out []model.Category
	for _, c := range r.categories {
		if categoryType == "" || string(c.Type) == categoryType {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *proposalCategoryRepo) Create(_ context.Context, c *model.Category) error {
	c.ID = uuid.New()
	r.categories = append(r.categories, *c)
	r.created++
	return nil
}

type proposalTxnService struct {
	TransactionService
	mu    sync.Mutex
	calls []dto.CreateTransactionRequest
	err   error
}

func (s *proposalTxnService) CreateTransaction(_ context.Context, _ uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	s.calls = append(s.calls, req)
	return &dto.TransactionResponse{ID: uuid.New(), Amount: req.Amount, Type: req.Type}, nil
}

type proposalChatRepo struct {
	repository.ChatRepository
	mu       sync.Mutex
	messages map[uuid.UUID]*model.ChatMessage
}

func (r *proposalChatRepo) GetMessageByID(_ context.Context, id uuid.UUID) (*model.ChatMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.messages[id]
	if !ok {
		return nil, constant.ErrNotFound
	}
	cp := *m
	return &cp, nil
}

func (r *proposalChatRepo) UpdateMessageToolResults(_ context.Context, id uuid.UUID, toolResults *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.messages[id]
	if !ok {
		m = &model.ChatMessage{ID: id}
		r.messages[id] = m
	}
	m.ToolResults = toolResults
	return nil
}

type proposalFixture struct {
	svc     *chatService
	txns    *proposalTxnService
	cats    *proposalCategoryRepo
	chat    *proposalChatRepo
	clock   *time.Time
	userID  uuid.UUID
	session uuid.UUID
	message uuid.UUID
}

func newProposalFixture(t *testing.T) *proposalFixture {
	t.Helper()
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	f := &proposalFixture{
		txns:    &proposalTxnService{},
		cats:    &proposalCategoryRepo{categories: []model.Category{{ID: uuid.New(), Name: "Ăn uống", Type: model.CategoryTypeExpense}}},
		chat:    &proposalChatRepo{messages: map[uuid.UUID]*model.ChatMessage{}},
		clock:   &now,
		userID:  uuid.New(),
		session: uuid.New(),
		message: uuid.New(),
	}
	clock := func() time.Time { return *f.clock }
	svc := NewChatService(f.chat, nil, nil, f.txns, nil, nil, nil, nil, nil, f.cats, nil,
		&stubWalletRepo{wallets: defaultWallets()}, zap.NewNop(),
		WithChatClock(clock), WithPendingActionStore(NewInMemoryPendingActionStore(PendingActionTTL, clock)))
	f.svc = svc.(*chatService)
	return f
}

func (f *proposalFixture) propose(t *testing.T, args map[string]interface{}) (*FinancialToolResult, uuid.UUID) {
	t.Helper()
	raw, _ := json.Marshal(args)
	res, err := f.svc.proposeWriteTool(context.Background(), f.userID, f.session, f.message,
		ToolCall{ID: "call-1", Type: "function", Function: ToolCallFunction{Name: "create_transaction", Arguments: string(raw)}})
	require.NoError(t, err)
	require.NotNil(t, res)
	if res.ActionCard == nil {
		return res, uuid.Nil
	}
	data := res.ActionCard.Data.(map[string]interface{})
	id, err := uuid.Parse(data["actionId"].(string))
	require.NoError(t, err)
	// The stream stores the turn's cards on the AI message when it ends.
	f.svc.persistActionCards(context.Background(), f.userID, f.message, []dto.ActionCard{*res.ActionCard})
	return res, id
}

func (f *proposalFixture) storedStatus(t *testing.T) (string, map[string]interface{}) {
	t.Helper()
	msg, err := f.chat.GetMessageByID(context.Background(), f.message)
	require.NoError(t, err)
	require.NotNil(t, msg.ToolResults)
	var cards []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(*msg.ToolResults), &cards))
	require.Len(t, cards, 1)
	data := cards[0]["data"].(map[string]interface{})
	return data["status"].(string), data
}

var lunchArgs = map[string]interface{}{"amount": 50000.0, "type": "EXPENSE", "categoryName": "ăn", "walletName": "tiền mặt", "description": "Phở"}

func TestWriteToolsAreAllDefined(t *testing.T) {
	defined := map[string]bool{}
	for _, d := range GetFinancialToolDefinitions() {
		defined[d.Function.Name] = true
	}
	for name := range chatWriteTools {
		assert.True(t, defined[name], "write tool %s must exist", name)
	}
	assert.False(t, IsChatWriteTool("get_financial_overview"))
	assert.False(t, IsChatWriteTool("search_transactions"))
}

func TestProposeCreateTransactionDoesNotWrite(t *testing.T) {
	f := newProposalFixture(t)
	res, id := f.propose(t, map[string]interface{}{"amount": 50000.0, "type": "EXPENSE", "categoryName": "Cà phê", "walletName": "tiền mặt"})
	require.NotEqual(t, uuid.Nil, id)
	assert.Empty(t, f.txns.calls, "no transaction before confirmation")
	assert.Equal(t, 0, f.cats.created, "no category auto-created before confirmation")
	assert.Equal(t, ActionTypePendingConfirmation, res.ActionCard.ActionType)
	assert.Contains(t, res.ResultJSON, pendingToolResultMessage)
	data := res.ActionCard.Data.(map[string]interface{})
	assert.Equal(t, "Ví Tiền mặt", data["walletName"], "wallet resolved to its exact name")
	assert.Equal(t, "Cà phê", data["categoryName"])
	assert.Equal(t, true, data["isNewCategoryCreated"])
	assert.Equal(t, "2026-10-17", data["date"])
	assert.Equal(t, "PENDING", data["status"])
}

func TestProposeInvalidArgsReturnsToolErrorWithoutPending(t *testing.T) {
	f := newProposalFixture(t)
	res, id := f.propose(t, map[string]interface{}{"amount": 0.0, "type": "EXPENSE"})
	assert.Equal(t, uuid.Nil, id)
	assert.Nil(t, res.ActionCard)
	assert.NotEmpty(t, toolError(t, res))

	res, id = f.propose(t, map[string]interface{}{"amount": 1000.0, "walletName": "techcom"})
	assert.Equal(t, uuid.Nil, id)
	assert.Contains(t, toolError(t, res), "nhiều ví")
}

func TestConfirmExecutesOnceAndPersistsOutcome(t *testing.T) {
	f := newProposalFixture(t)
	_, id := f.propose(t, lunchArgs)

	result, err := f.svc.ConfirmAction(context.Background(), f.userID, id)
	require.NoError(t, err)
	require.Len(t, f.txns.calls, 1)
	assert.Equal(t, 50000.0, f.txns.calls[0].Amount)
	assert.Equal(t, "CONFIRMED", result.Status)
	require.NotNil(t, result.ActionCard)
	assert.Equal(t, "TRANSACTION_CREATED", result.ActionCard.ActionType)
	assert.NotNil(t, result.Result)

	status, data := f.storedStatus(t)
	assert.Equal(t, "CONFIRMED", status)
	assert.NotNil(t, data["resultCard"], "history keeps the created entity for undo")

	_, err = f.svc.ConfirmAction(context.Background(), f.userID, id)
	assert.ErrorIs(t, err, ErrPendingActionHandled, "double confirm")
	_, err = f.svc.CancelAction(context.Background(), f.userID, id)
	assert.ErrorIs(t, err, ErrPendingActionHandled)
	assert.Len(t, f.txns.calls, 1, "never executed twice")
}

func TestConcurrentConfirmsExecuteOnce(t *testing.T) {
	f := newProposalFixture(t)
	_, id := f.propose(t, lunchArgs)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.svc.ConfirmAction(context.Background(), f.userID, id); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, ok)
	assert.Len(t, f.txns.calls, 1)
}

func TestCancelDiscardsProposal(t *testing.T) {
	f := newProposalFixture(t)
	_, id := f.propose(t, lunchArgs)
	result, err := f.svc.CancelAction(context.Background(), f.userID, id)
	require.NoError(t, err)
	assert.Equal(t, "CANCELLED", result.Status)
	status, _ := f.storedStatus(t)
	assert.Equal(t, "CANCELLED", status)

	_, err = f.svc.ConfirmAction(context.Background(), f.userID, id)
	assert.ErrorIs(t, err, ErrPendingActionHandled)
	assert.Empty(t, f.txns.calls)
}

func TestExpiredProposalCannotBeConfirmed(t *testing.T) {
	f := newProposalFixture(t)
	_, id := f.propose(t, lunchArgs)
	*f.clock = f.clock.Add(PendingActionTTL + time.Second)

	_, err := f.svc.ConfirmAction(context.Background(), f.userID, id)
	assert.ErrorIs(t, err, ErrPendingActionExpired)
	_, err = f.svc.CancelAction(context.Background(), f.userID, id)
	assert.ErrorIs(t, err, ErrPendingActionExpired)
	assert.Empty(t, f.txns.calls)
}

func TestOtherUserCannotConfirm(t *testing.T) {
	f := newProposalFixture(t)
	_, id := f.propose(t, lunchArgs)

	_, err := f.svc.ConfirmAction(context.Background(), uuid.New(), id)
	assert.ErrorIs(t, err, ErrPendingActionNotFound)
	_, err = f.svc.CancelAction(context.Background(), uuid.New(), id)
	assert.ErrorIs(t, err, ErrPendingActionNotFound)
	assert.Empty(t, f.txns.calls)

	_, err = f.svc.ConfirmAction(context.Background(), f.userID, id)
	assert.NoError(t, err, "the owner can still confirm")
}

func TestUnknownActionIsNotFound(t *testing.T) {
	f := newProposalFixture(t)
	_, err := f.svc.ConfirmAction(context.Background(), f.userID, uuid.New())
	assert.ErrorIs(t, err, ErrPendingActionNotFound)
}

func TestFailedExecutionKeepsProposalPending(t *testing.T) {
	f := newProposalFixture(t)
	_, id := f.propose(t, lunchArgs)
	f.txns.err = constant.ErrInsufficientBalance

	_, err := f.svc.ConfirmAction(context.Background(), f.userID, id)
	assert.ErrorIs(t, err, ErrPendingActionFailed)
	status, _ := f.storedStatus(t)
	assert.Equal(t, "PENDING", status)

	f.txns.err = nil
	_, err = f.svc.ConfirmAction(context.Background(), f.userID, id)
	assert.NoError(t, err, "retry after a rejected execution")
}

func TestPreviewOtherWriteTools(t *testing.T) {
	p, errRes := previewCreateDebt(map[string]interface{}{"title": "Vay Nam", "totalAmount": 2000000.0, "type": "RECEIVABLE", "dueDate": "2026-11-01"})
	require.Nil(t, errRes)
	assert.Equal(t, "RECEIVABLE", p.args["type"])
	assert.Equal(t, "2026-11-01", p.args["dueDate"])

	_, errRes = previewCreateDebt(map[string]interface{}{"title": "", "totalAmount": 1.0})
	assert.NotNil(t, errRes)

	p, errRes = previewSetTarget(map[string]interface{}{"targetType": "savings", "targetAmount": 5000000.0, "month": 11.0, "year": 2026.0})
	require.Nil(t, errRes)
	assert.Equal(t, "INVESTMENT", p.args["targetType"])

	f := newProposalFixture(t)
	p, errRes, err := f.svc.previewTransfer(context.Background(), f.userID, map[string]interface{}{"amount": 100000.0, "fromWalletName": "tiền mặt", "toWalletName": "ngân hàng"})
	require.NoError(t, err)
	require.Nil(t, errRes)
	assert.Equal(t, "Ví Tiền mặt", p.args["fromWalletName"])
	assert.Equal(t, "Tài khoản Ngân hàng", p.args["toWalletName"])
}

func TestPendingStorePrunesLongExpiredActions(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	store := NewInMemoryPendingActionStore(time.Minute, clock)
	user, id := uuid.New(), uuid.New()
	store.Save(PendingAction{ID: id, UserID: user, ToolName: "create_debt"})
	got, err := store.Get(user, id)
	require.NoError(t, err)
	assert.Equal(t, PendingActionPending, got.Status)

	now = now.Add(2 * time.Minute)
	got, err = store.Get(user, id)
	require.NoError(t, err)
	assert.Equal(t, PendingActionExpired, got.Status)

	now = now.Add(pendingActionRetention + time.Minute)
	store.Save(PendingAction{ID: uuid.New(), UserID: user})
	_, err = store.Get(user, id)
	assert.ErrorIs(t, err, ErrPendingActionNotFound)
}
