package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"go.uber.org/zap"
)

// Fakes for the collaborators of the AI financial tools. Each one records the
// user it was called for so tests can assert that tools never read or write
// another user's data. Unimplemented interface methods panic (embedded nil).

type tfReportSvc struct {
	ReportService
	mu         sync.Mutex
	summaries  map[string]*dto.SummaryReport // key: startDate
	breakdown  *dto.CategoryBreakdownReport
	summaryErr error
	breakErr   error
	users      []uuid.UUID
	ranges     [][2]string
}

func (r *tfReportSvc) GetSummary(_ context.Context, userID uuid.UUID, start, end string) (*dto.SummaryReport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users = append(r.users, userID)
	r.ranges = append(r.ranges, [2]string{start, end})
	if r.summaryErr != nil {
		return nil, r.summaryErr
	}
	if s, ok := r.summaries[start]; ok {
		return s, nil
	}
	return &dto.SummaryReport{}, nil
}

func (r *tfReportSvc) GetCategoryBreakdown(_ context.Context, userID uuid.UUID, start, end string) (*dto.CategoryBreakdownReport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users = append(r.users, userID)
	r.ranges = append(r.ranges, [2]string{start, end})
	if r.breakErr != nil {
		return nil, r.breakErr
	}
	if r.breakdown == nil {
		return &dto.CategoryBreakdownReport{}, nil
	}
	return r.breakdown, nil
}

type tfWalletSvc struct {
	WalletService
	summary     *dto.WalletSummaryResponse
	summaryErr  error
	transfers   []dto.TransferMoneyRequest
	transferErr error
	users       []uuid.UUID
}

func (w *tfWalletSvc) GetWallets(_ context.Context, userID uuid.UUID) (*dto.WalletSummaryResponse, error) {
	w.users = append(w.users, userID)
	if w.summaryErr != nil {
		return nil, w.summaryErr
	}
	return w.summary, nil
}

func (w *tfWalletSvc) TransferMoney(_ context.Context, userID uuid.UUID, req dto.TransferMoneyRequest) (*dto.WalletTransferResponse, error) {
	w.users = append(w.users, userID)
	if w.transferErr != nil {
		return nil, w.transferErr
	}
	w.transfers = append(w.transfers, req)
	return &dto.WalletTransferResponse{ID: uuid.New(), UserID: userID, FromWalletID: req.FromWalletID, ToWalletID: req.ToWalletID, Amount: req.Amount, Fee: req.Fee}, nil
}

type repaymentCall struct {
	debtID uuid.UUID
	req    dto.AddRepaymentRequest
}

type tfDebtSvc struct {
	DebtService
	debts      []dto.DebtResponse
	summary    *dto.DebtSummaryResponse
	getErr     error
	summaryErr error
	createErr  error
	repayErr   error
	created    []dto.CreateDebtRequest
	repaid     []repaymentCall
	users      []uuid.UUID
}

func (d *tfDebtSvc) GetDebts(_ context.Context, userID uuid.UUID, _ model.DebtType, _ model.DebtStatus) ([]dto.DebtResponse, error) {
	d.users = append(d.users, userID)
	return d.debts, d.getErr
}

func (d *tfDebtSvc) GetDebtSummary(_ context.Context, userID uuid.UUID) (*dto.DebtSummaryResponse, error) {
	d.users = append(d.users, userID)
	if d.summaryErr != nil {
		return nil, d.summaryErr
	}
	return d.summary, nil
}

func (d *tfDebtSvc) CreateDebt(_ context.Context, userID uuid.UUID, req dto.CreateDebtRequest) (*dto.DebtResponse, error) {
	d.users = append(d.users, userID)
	if d.createErr != nil {
		return nil, d.createErr
	}
	d.created = append(d.created, req)
	return &dto.DebtResponse{ID: uuid.New(), UserID: userID, Type: req.Type, Title: req.Title, TotalAmount: req.TotalAmount, Remaining: req.TotalAmount, DueDate: req.DueDate}, nil
}

func (d *tfDebtSvc) AddRepayment(_ context.Context, userID, debtID uuid.UUID, req dto.AddRepaymentRequest) (*dto.DebtResponse, error) {
	d.users = append(d.users, userID)
	if d.repayErr != nil {
		return nil, d.repayErr
	}
	d.repaid = append(d.repaid, repaymentCall{debtID: debtID, req: req})
	for _, debt := range d.debts {
		if debt.ID == debtID {
			debt.PaidAmount += req.Amount
			debt.Remaining = debt.TotalAmount - debt.PaidAmount
			return &debt, nil
		}
	}
	return nil, ErrDebtNotFound
}

type tfBudgetSvc struct {
	BudgetService
	budgets   []dto.BudgetResponse
	listErr   error
	createErr error
	created   []dto.CreateBudgetRequest
	users     []uuid.UUID
}

func (b *tfBudgetSvc) ListBudgets(_ context.Context, userID uuid.UUID, _, _ int) ([]dto.BudgetResponse, int64, error) {
	b.users = append(b.users, userID)
	if b.listErr != nil {
		return nil, 0, b.listErr
	}
	return b.budgets, int64(len(b.budgets)), nil
}

func (b *tfBudgetSvc) CreateBudget(_ context.Context, userID uuid.UUID, req dto.CreateBudgetRequest) (*dto.BudgetResponse, error) {
	b.users = append(b.users, userID)
	if b.createErr != nil {
		return nil, b.createErr
	}
	b.created = append(b.created, req)
	return &dto.BudgetResponse{ID: uuid.New(), UserID: userID, CategoryID: req.CategoryID, Amount: req.Amount, PeriodType: req.PeriodType,
		PeriodStart: req.PeriodStart.Format("2006-01-02")}, nil
}

type tfTargetSvc struct {
	TargetService
	summary    *dto.TargetSummaryResponse
	summaryErr error
	upserts    []dto.UpsertTargetRequest
	gotMonth   int
	gotYear    int
	users      []uuid.UUID
}

func (s *tfTargetSvc) GetSummary(_ context.Context, userID uuid.UUID, month, year int) (*dto.TargetSummaryResponse, error) {
	s.users = append(s.users, userID)
	s.gotMonth, s.gotYear = month, year
	if s.summaryErr != nil {
		return nil, s.summaryErr
	}
	return s.summary, nil
}

func (s *tfTargetSvc) UpsertTarget(_ context.Context, userID uuid.UUID, req *dto.UpsertTargetRequest) error {
	s.users = append(s.users, userID)
	if err := validateTarget(req.TargetType, req.TargetAmount, req.Month, req.Year); err != nil {
		return err
	}
	s.upserts = append(s.upserts, *req)
	return nil
}

type tfTxnSvc struct {
	TransactionService
	list        []dto.TransactionResponse
	summary     *dto.TransactionSummaryDTO
	listErr     error
	lastLimit   int
	lastFilters map[string]interface{}
	created     []dto.CreateTransactionRequest
	users       []uuid.UUID
}

func (s *tfTxnSvc) ListTransactions(_ context.Context, userID uuid.UUID, _, limit int, filters map[string]interface{}) ([]dto.TransactionResponse, int64, *dto.TransactionSummaryDTO, error) {
	s.users = append(s.users, userID)
	s.lastLimit, s.lastFilters = limit, filters
	if s.listErr != nil {
		return nil, 0, nil, s.listErr
	}
	summary := s.summary
	if summary == nil {
		summary = &dto.TransactionSummaryDTO{}
	}
	return s.list, int64(len(s.list)), summary, nil
}

func (s *tfTxnSvc) CreateTransaction(_ context.Context, userID uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
	s.users = append(s.users, userID)
	s.created = append(s.created, req)
	return &dto.TransactionResponse{ID: uuid.New(), UserID: userID, Amount: req.Amount, Type: req.Type}, nil
}

type tfTxnRepo struct {
	repository.TransactionRepository
	rows        []model.Transaction
	err         error
	lastLimit   int
	lastFilters map[string]interface{}
	users       []uuid.UUID
}

func (r *tfTxnRepo) SearchByUserID(_ context.Context, userID uuid.UUID, limit, _ int, filters map[string]interface{}) ([]model.Transaction, int64, error) {
	r.users = append(r.users, userID)
	r.lastLimit, r.lastFilters = limit, filters
	if r.err != nil {
		return nil, 0, r.err
	}
	return r.rows, int64(len(r.rows)), nil
}

type tfWalletRepo struct {
	repository.WalletRepository
	wallets    []model.Wallet
	summary    *dto.WalletSummaryResponse
	listErr    error
	summaryErr error
	users      []uuid.UUID
}

func (r *tfWalletRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]model.Wallet, error) {
	r.users = append(r.users, userID)
	return r.wallets, r.listErr
}

func (r *tfWalletRepo) GetSummaryByUserID(_ context.Context, userID uuid.UUID) (*dto.WalletSummaryResponse, error) {
	r.users = append(r.users, userID)
	return r.summary, r.summaryErr
}

type tfRAG struct {
	RAGService
	results []KnowledgeSearchResult
	err     error
	queries []string
	limits  []int
}

func (r *tfRAG) SearchKnowledge(_ context.Context, query string, limit int) ([]KnowledgeSearchResult, error) {
	r.queries = append(r.queries, query)
	r.limits = append(r.limits, limit)
	return r.results, r.err
}

// toolEnv wires a chatService with fakes for every collaborator.
type toolEnv struct {
	svc     *chatService
	userID  uuid.UUID
	report  *tfReportSvc
	wallet  *tfWalletSvc
	debt    *tfDebtSvc
	budget  *tfBudgetSvc
	target  *tfTargetSvc
	txnSvc  *tfTxnSvc
	txnRepo *tfTxnRepo
	wallets *tfWalletRepo
	cats    *recordingCategoryRepo
	rag     *tfRAG
	chat    *proposalChatRepo
	clock   *time.Time
}

func newToolEnv(t *testing.T) *toolEnv {
	t.Helper()
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	e := &toolEnv{
		userID:  uuid.New(),
		report:  &tfReportSvc{summaries: map[string]*dto.SummaryReport{}},
		wallet:  &tfWalletSvc{summary: &dto.WalletSummaryResponse{}},
		debt:    &tfDebtSvc{summary: &dto.DebtSummaryResponse{}},
		budget:  &tfBudgetSvc{},
		target:  &tfTargetSvc{summary: &dto.TargetSummaryResponse{}},
		txnSvc:  &tfTxnSvc{},
		txnRepo: &tfTxnRepo{},
		rag:     &tfRAG{},
		chat:    &proposalChatRepo{messages: map[uuid.UUID]*model.ChatMessage{}},
		clock:   &now,
	}
	e.wallets = &tfWalletRepo{wallets: defaultWallets()}
	e.cats = &recordingCategoryRepo{}
	clock := func() time.Time { return *e.clock }
	svc := NewChatService(e.chat, nil, e.rag, e.txnSvc, e.report, e.budget, e.debt, e.wallet, e.target, e.cats, e.txnRepo, e.wallets,
		zap.NewNop(), WithChatClock(clock), WithPendingActionStore(NewInMemoryPendingActionStore(PendingActionTTL, clock)))
	e.svc = svc.(*chatService)
	return e
}

// addCategory stores a category owned by the env user (or by owner if given).
func (e *toolEnv) addCategory(name string, typ model.CategoryType, owner ...uuid.UUID) model.Category {
	uid := e.userID
	if len(owner) > 0 {
		uid = owner[0]
	}
	c := model.Category{ID: uuid.New(), UserID: uid, Name: name, Type: typ}
	e.cats.rows = append(e.cats.rows, c)
	return c
}

func (e *toolEnv) call(t *testing.T, name string, args map[string]interface{}) *FinancialToolResult {
	t.Helper()
	raw, err := json.Marshal(args)
	require.NoError(t, err)
	res, err := e.svc.executeFinancialTool(context.Background(), e.userID, ToolCall{ID: "c1", Type: "function", Function: ToolCallFunction{Name: name, Arguments: string(raw)}})
	require.NoError(t, err)
	require.NotNil(t, res)
	return res
}

// decodeTool parses a tool's ResultJSON.
func decodeTool(t *testing.T, res *FinancialToolResult) map[string]interface{} {
	t.Helper()
	require.NotNil(t, res)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(res.ResultJSON), &out), "tool result must be valid JSON: %s", res.ResultJSON)
	return out
}

// requireOnlyUser asserts every recorded call was made for the given user.
func requireOnlyUser(t *testing.T, want uuid.UUID, got []uuid.UUID) {
	t.Helper()
	for _, u := range got {
		require.Equal(t, want, u, "a collaborator was called for another user")
	}
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}
