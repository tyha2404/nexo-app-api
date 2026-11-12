package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

// These tests pin the exact bytes of each success-envelope kind. The expected
// body is built from the response structs the handlers used to encode by hand
// (json.Encoder appends a trailing newline), so the shared helpers must keep
// the JSON clients receive byte-for-byte identical.

var (
	shapeUserID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	shapeID     = uuid.MustParse("22222222-2222-2222-2222-222222222222")
)

func shapeEncode(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b) + "\n"
}

type shapeBudgetService struct {
	service.BudgetService
}

func (shapeBudgetService) GetBudget(ctx context.Context, userID, id uuid.UUID) (*dto.BudgetResponse, error) {
	return &dto.BudgetResponse{ID: id, UserID: userID, Amount: 10, PeriodType: "monthly"}, nil
}

func (shapeBudgetService) ListBudgets(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.BudgetResponse, int64, error) {
	return []dto.BudgetResponse{{ID: shapeID, UserID: userID, Amount: 5}}, 42, nil
}

type shapeDebtService struct {
	service.DebtService
}

func (shapeDebtService) CreateDebt(ctx context.Context, userID uuid.UUID, req dto.CreateDebtRequest) (*dto.DebtResponse, error) {
	return &dto.DebtResponse{ID: shapeID, UserID: userID, Type: req.Type, Title: req.Title, TotalAmount: req.TotalAmount, Remaining: req.TotalAmount}, nil
}

type shapeTransactionService struct {
	service.TransactionService
	summary *dto.TransactionSummaryDTO
	items   []dto.TransactionResponse
}

func (s shapeTransactionService) ListTransactions(ctx context.Context, userID uuid.UUID, page, limit int, filters map[string]interface{}) ([]dto.TransactionResponse, int64, *dto.TransactionSummaryDTO, error) {
	return s.items, int64(len(s.items)), s.summary, nil
}

func TestEnvelopeShape_PlainData(t *testing.T) {
	rr := httptest.NewRecorder()
	uid := shapeUserID
	NewBudgetHandler(shapeBudgetService{}, zap.NewNop()).Get(rr, debtTestReq(http.MethodGet, "/budgets/x", "", &uid, shapeID.String()))

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	want := shapeEncode(t, response.BaseResponse[dto.BudgetResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    dto.BudgetResponse{ID: shapeID, UserID: shapeUserID, Amount: 10, PeriodType: "monthly"},
	})
	assert.Equal(t, want, rr.Body.String())
}

func TestEnvelopeShape_DataWithMessage(t *testing.T) {
	rr := httptest.NewRecorder()
	uid := shapeUserID
	body := `{"type":"PAYABLE","title":"Vay","totalAmount":1500000}`
	NewDebtHandler(shapeDebtService{}, zap.NewNop()).CreateDebt(rr, debtTestReq(http.MethodPost, "/debts", body, &uid, ""))

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	want := shapeEncode(t, response.BaseResponse[*dto.DebtResponse]{
		Status:  http.StatusCreated,
		Success: true,
		Message: "Debt created successfully",
		Data:    &dto.DebtResponse{ID: shapeID, UserID: shapeUserID, Type: model.DebtTypePayable, Title: "Vay", TotalAmount: 1500000, Remaining: 1500000},
	})
	assert.Equal(t, want, rr.Body.String())
}

func TestEnvelopeShape_Paginated(t *testing.T) {
	rr := httptest.NewRecorder()
	uid := shapeUserID
	NewBudgetHandler(shapeBudgetService{}, zap.NewNop()).List(rr, debtTestReq(http.MethodGet, "/budgets?page=2&limit=5", "", &uid, ""))

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	want := shapeEncode(t, response.PaginationResponse[dto.BudgetResponse]{
		Status:  http.StatusOK,
		Success: true,
		Items:   []dto.BudgetResponse{{ID: shapeID, UserID: shapeUserID, Amount: 5}},
		Total:   42,
		Page:    2,
		Limit:   5,
	})
	assert.Equal(t, want, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"message":""`)
	assert.NotContains(t, rr.Body.String(), `"summary"`)
}

func TestEnvelopeShape_PaginatedWithSummary(t *testing.T) {
	summary := &dto.TransactionSummaryDTO{}
	tests := []struct {
		name    string
		svc     shapeTransactionService
		summary interface{}
	}{
		// A typed-nil *TransactionSummaryDTO is a non-nil interface, so it is
		// encoded as "summary":null rather than omitted.
		{"nil summary and nil items", shapeTransactionService{}, (*dto.TransactionSummaryDTO)(nil)},
		{"with summary", shapeTransactionService{summary: summary, items: []dto.TransactionResponse{{ID: shapeID}}}, summary},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := txnTestReq(http.MethodGet, "/transactions?page=1&limit=10", "", shapeUserID, "")
			NewTransactionHandler(tt.svc, zap.NewNop()).ListTransactions(rr, req)

			require.Equal(t, http.StatusOK, rr.Code)
			want := shapeEncode(t, response.PaginationResponse[dto.TransactionResponse]{
				Status:  http.StatusOK,
				Success: true,
				Items:   tt.svc.items,
				Total:   len(tt.svc.items),
				Page:    1,
				Limit:   10,
				Summary: tt.summary,
			})
			assert.Equal(t, want, rr.Body.String())
		})
	}
}

// The 8 resource handlers now validate with *Validator, so a 400 carries the
// friendly field message instead of go-playground's raw
// "Key: 'X.Name' Error:Field validation ..." text; decode errors keep
// "Invalid request payload".
func TestDecodeAndValidate_ErrorEnvelopes(t *testing.T) {
	uid := shapeUserID
	tests := []struct {
		name      string
		body      string
		wantMsg   string
		wantError string
	}{
		{"malformed json", `{"type":`, "Invalid request payload", "unexpected EOF"},
		{"missing title", `{"type":"PAYABLE","totalAmount":10}`, "Validation failed", "validation failed: Title is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			NewDebtHandler(shapeDebtService{}, zap.NewNop()).CreateDebt(rr, debtTestReq(http.MethodPost, "/debts", tt.body, &uid, ""))

			require.Equal(t, http.StatusBadRequest, rr.Code)
			body := decodeJSON(t, rr)
			assert.Equal(t, tt.wantMsg, body["message"])
			assert.Equal(t, tt.wantError, body["error"])
		})
	}
}

// Invalid path IDs keep each endpoint's message and the uuid parser's text.
func TestParseUUIDFromPath_ErrorEnvelope(t *testing.T) {
	uid := shapeUserID
	rr := httptest.NewRecorder()
	NewBudgetHandler(shapeBudgetService{}, zap.NewNop()).Get(rr, debtTestReq(http.MethodGet, "/budgets/abc", "", &uid, "abc"))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	body := decodeJSON(t, rr)
	assert.Equal(t, "Invalid ID format", body["message"])
	assert.Equal(t, "invalid UUID length: 3", body["error"])
}
