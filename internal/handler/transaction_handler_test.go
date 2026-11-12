package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type stubTransactionService struct {
	service.TransactionService
	createFn  func(ctx context.Context, userID uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error)
	getFn     func(ctx context.Context, userID, id uuid.UUID) (*dto.TransactionResponse, error)
	listFn    func(ctx context.Context, userID uuid.UUID, page, limit int, filters map[string]interface{}) ([]dto.TransactionResponse, int64, *dto.TransactionSummaryDTO, error)
	updateFn  func(ctx context.Context, userID, id uuid.UUID, req dto.UpdateTransactionRequest) (*dto.TransactionResponse, error)
	deleteFn  func(ctx context.Context, userID, id uuid.UUID) error
	restoreFn func(ctx context.Context, userID, id uuid.UUID) (*dto.TransactionResponse, error)
	tagsFn    func(ctx context.Context, userID uuid.UUID) ([]dto.TagSummary, error)
	calls     int
}

func (s *stubTransactionService) CreateTransaction(ctx context.Context, userID uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
	s.calls++
	return s.createFn(ctx, userID, req)
}

func (s *stubTransactionService) GetTransaction(ctx context.Context, userID, id uuid.UUID) (*dto.TransactionResponse, error) {
	s.calls++
	return s.getFn(ctx, userID, id)
}

func (s *stubTransactionService) ListTransactions(ctx context.Context, userID uuid.UUID, page, limit int, filters map[string]interface{}) ([]dto.TransactionResponse, int64, *dto.TransactionSummaryDTO, error) {
	s.calls++
	return s.listFn(ctx, userID, page, limit, filters)
}

func (s *stubTransactionService) UpdateTransaction(ctx context.Context, userID, id uuid.UUID, req dto.UpdateTransactionRequest) (*dto.TransactionResponse, error) {
	s.calls++
	return s.updateFn(ctx, userID, id, req)
}

func (s *stubTransactionService) DeleteTransaction(ctx context.Context, userID, id uuid.UUID) error {
	s.calls++
	return s.deleteFn(ctx, userID, id)
}

func (s *stubTransactionService) RestoreTransaction(ctx context.Context, userID, id uuid.UUID) (*dto.TransactionResponse, error) {
	s.calls++
	return s.restoreFn(ctx, userID, id)
}

func (s *stubTransactionService) ListTags(ctx context.Context, userID uuid.UUID) ([]dto.TagSummary, error) {
	s.calls++
	return s.tagsFn(ctx, userID)
}

// txnTestReq builds a request with an optional authenticated user and chi {id} param.
func txnTestReq(method, target, body string, user uuid.UUID, id string) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if id != "" {
		req = withURLParam(req, "id", id)
	}
	if user != uuid.Nil {
		req = withUser(req, user)
	}
	return req
}

// txnAssertError checks the standard error envelope and that 5xx bodies do not leak the cause.
func txnAssertError(t *testing.T, rr *httptest.ResponseRecorder, wantStatus int, wantMsg string) {
	t.Helper()
	require.Equal(t, wantStatus, rr.Code, rr.Body.String())
	body := decodeJSON(t, rr)
	assert.Equal(t, false, body["success"])
	assert.Equal(t, float64(wantStatus), body["status"])
	if wantMsg != "" {
		assert.Equal(t, wantMsg, body["message"])
	}
	if wantStatus >= http.StatusInternalServerError {
		_, leaked := body["error"]
		assert.False(t, leaked, "5xx must not expose error detail")
	}
}

const txnValidCreateBody = `{"categoryId":"550e8400-e29b-41d4-a716-446655440000","amount":120000,"type":"EXPENSE","transactionDate":"2026-11-01","tags":["Đà Lạt"]}`

func TestTransactionHandler_Create(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name       string
		body       string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
		wantCalled bool
	}{
		{"success", txnValidCreateBody, userID, nil, http.StatusCreated, "", true},
		{"malformed json", `{"amount":`, userID, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"bad date format", `{"categoryId":"550e8400-e29b-41d4-a716-446655440000","amount":1,"type":"EXPENSE","transactionDate":"01/11/2026"}`, userID, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"missing category", `{"amount":1,"type":"EXPENSE","transactionDate":"2026-11-01"}`, userID, nil, http.StatusBadRequest, "Validation failed", false},
		{"zero amount", `{"categoryId":"550e8400-e29b-41d4-a716-446655440000","amount":0,"type":"EXPENSE","transactionDate":"2026-11-01"}`, userID, nil, http.StatusBadRequest, "Validation failed", false},
		{"negative amount", `{"categoryId":"550e8400-e29b-41d4-a716-446655440000","amount":-5,"type":"EXPENSE","transactionDate":"2026-11-01"}`, userID, nil, http.StatusBadRequest, "Validation failed", false},
		{"unknown type", `{"categoryId":"550e8400-e29b-41d4-a716-446655440000","amount":1,"type":"COST","transactionDate":"2026-11-01"}`, userID, nil, http.StatusBadRequest, "Validation failed", false},
		{"unknown status", `{"categoryId":"550e8400-e29b-41d4-a716-446655440000","amount":1,"type":"INVESTMENT","status":"LOST","transactionDate":"2026-11-01"}`, userID, nil, http.StatusBadRequest, "Validation failed", false},
		{"too many tags", `{"categoryId":"550e8400-e29b-41d4-a716-446655440000","amount":1,"type":"EXPENSE","transactionDate":"2026-11-01","tags":["1","2","3","4","5","6","7","8","9","10","11"]}`, userID, nil, http.StatusBadRequest, "Validation failed", false},
		{"receipt url not http", `{"categoryId":"550e8400-e29b-41d4-a716-446655440000","amount":1,"type":"EXPENSE","transactionDate":"2026-11-01","receiptUrl":"javascript:alert(1)"}`, userID, nil, http.StatusBadRequest, "Validation failed", false},
		{"unauthenticated", txnValidCreateBody, uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized", false},
		{"category or wallet not found", txnValidCreateBody, userID, fmt.Errorf("wallet: %w", gorm.ErrRecordNotFound), http.StatusNotFound, "Category or wallet not found", true},
		{"invalid input from service", txnValidCreateBody, userID, fmt.Errorf("currency mismatch: %w", constant.ErrInvalidInput), http.StatusBadRequest, "Invalid transaction", true},
		{"unexpected error", txnValidCreateBody, userID, errors.New("pq: deadlock"), http.StatusInternalServerError, "Failed to create transaction", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			var gotReq dto.CreateTransactionRequest
			svc := &stubTransactionService{
				createFn: func(ctx context.Context, uid uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
					gotUser, gotReq = uid, req
					if tt.svcErr != nil {
						return nil, tt.svcErr
					}
					return &dto.TransactionResponse{ID: uuid.New(), UserID: uid, CategoryID: req.CategoryID, Amount: req.Amount, Type: req.Type, Tags: req.Tags}, nil
				},
			}
			h := NewTransactionHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()

			h.CreateTransaction(rr, txnTestReq(http.MethodPost, "/transactions", tt.body, tt.user, ""))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service called")
			if tt.wantStatus != http.StatusCreated {
				txnAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
			assert.Equal(t, userID, gotUser, "service must be scoped to the authenticated user")
			assert.Equal(t, 120000.0, gotReq.Amount)
			assert.Equal(t, "2026-11-01", gotReq.TransactionDate.Format("2006-01-02"))
			body := decodeJSON(t, rr)
			assert.Equal(t, true, body["success"])
			assert.Equal(t, float64(http.StatusCreated), body["status"])
			data := body["data"].(map[string]interface{})
			assert.Equal(t, userID.String(), data["userId"])
			assert.Equal(t, "EXPENSE", data["type"])
		})
	}
}

func TestTransactionHandler_Get(t *testing.T) {
	userID, txnID := uuid.New(), uuid.New()
	tests := []struct {
		name       string
		id         string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", txnID.String(), userID, nil, http.StatusOK, ""},
		{"invalid uuid", "abc", userID, nil, http.StatusBadRequest, "Invalid ID format"},
		{"unauthenticated", txnID.String(), uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized"},
		{"not found", txnID.String(), userID, constant.ErrNotFound, http.StatusNotFound, "Transaction not found"},
		{"gorm not found", txnID.String(), userID, gorm.ErrRecordNotFound, http.StatusNotFound, "Transaction not found"},
		{"unexpected", txnID.String(), userID, errors.New("boom"), http.StatusInternalServerError, "Failed to get transaction"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotID uuid.UUID
			svc := &stubTransactionService{
				getFn: func(ctx context.Context, uid, id uuid.UUID) (*dto.TransactionResponse, error) {
					gotUser, gotID = uid, id
					if tt.svcErr != nil {
						return nil, tt.svcErr
					}
					return &dto.TransactionResponse{ID: id, UserID: uid, Amount: 50}, nil
				},
			}
			h := NewTransactionHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()

			h.GetTransaction(rr, txnTestReq(http.MethodGet, "/transactions/"+tt.id, "", tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				txnAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, txnID, gotID)
			body := decodeJSON(t, rr)
			assert.Equal(t, true, body["success"])
			assert.Equal(t, txnID.String(), body["data"].(map[string]interface{})["id"])
		})
	}
}

func TestTransactionHandler_List(t *testing.T) {
	userID := uuid.New()

	t.Run("passes pagination, whitelisted filters and returns summary", func(t *testing.T) {
		var gotUser uuid.UUID
		var gotPage, gotLimit int
		var gotFilters map[string]interface{}
		svc := &stubTransactionService{
			listFn: func(ctx context.Context, uid uuid.UUID, page, limit int, filters map[string]interface{}) ([]dto.TransactionResponse, int64, *dto.TransactionSummaryDTO, error) {
				gotUser, gotPage, gotLimit, gotFilters = uid, page, limit, filters
				return []dto.TransactionResponse{{ID: uuid.New(), UserID: uid, Amount: 10}}, 41, &dto.TransactionSummaryDTO{SumAmount: 10, Total: 41}, nil
			},
		}
		h := NewTransactionHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		target := "/transactions?page=3&limit=20&type=EXPENSE&categoryId=c1&startDate=2026-11-01&endDate=2026-11-30&keyword=cafe&tag=trip&userId=" + uuid.NewString() + "&status="

		h.ListTransactions(rr, txnTestReq(http.MethodGet, target, "", userID, ""))

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Equal(t, userID, gotUser)
		assert.Equal(t, 3, gotPage)
		assert.Equal(t, 20, gotLimit)
		assert.Equal(t, map[string]interface{}{
			"type": "EXPENSE", "categoryId": "c1", "startDate": "2026-11-01", "endDate": "2026-11-30", "keyword": "cafe", "tag": "trip",
		}, gotFilters, "only whitelisted, non-empty filters reach the service (never a client userId)")

		body := decodeJSON(t, rr)
		assert.Equal(t, true, body["success"])
		assert.Equal(t, float64(41), body["total"])
		assert.Equal(t, float64(3), body["page"])
		assert.Equal(t, float64(20), body["limit"])
		assert.Len(t, body["items"], 1)
		assert.Equal(t, float64(10), body["summary"].(map[string]interface{})["sumAmount"])
	})

	t.Run("unauthenticated", func(t *testing.T) {
		svc := &stubTransactionService{}
		h := NewTransactionHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.ListTransactions(rr, txnTestReq(http.MethodGet, "/transactions", "", uuid.Nil, ""))
		txnAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
		assert.Zero(t, svc.calls)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &stubTransactionService{
			listFn: func(ctx context.Context, uid uuid.UUID, page, limit int, filters map[string]interface{}) ([]dto.TransactionResponse, int64, *dto.TransactionSummaryDTO, error) {
				return nil, 0, nil, errors.New("pq: timeout")
			},
		}
		h := NewTransactionHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.ListTransactions(rr, txnTestReq(http.MethodGet, "/transactions", "", userID, ""))
		txnAssertError(t, rr, http.StatusInternalServerError, "Failed to list transactions")
	})
}

func TestTransactionHandler_Update(t *testing.T) {
	userID, txnID := uuid.New(), uuid.New()
	tests := []struct {
		name       string
		id         string
		body       string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", txnID.String(), `{"amount":99,"tags":[]}`, userID, nil, http.StatusOK, ""},
		{"invalid uuid", "123", `{"amount":99}`, userID, nil, http.StatusBadRequest, "Invalid ID format"},
		{"malformed json", txnID.String(), `{"amount":"x"}`, userID, nil, http.StatusBadRequest, "Invalid request payload"},
		{"zero amount", txnID.String(), `{"amount":0}`, userID, nil, http.StatusBadRequest, "Validation failed"},
		{"bad type", txnID.String(), `{"type":"GIFT"}`, userID, nil, http.StatusBadRequest, "Validation failed"},
		{"unauthenticated", txnID.String(), `{"amount":99}`, uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized"},
		{"not found", txnID.String(), `{"amount":99}`, userID, constant.ErrNotFound, http.StatusNotFound, "Transaction, category or wallet not found"},
		{"invalid input", txnID.String(), `{"amount":99}`, userID, fmt.Errorf("x: %w", constant.ErrInvalidInput), http.StatusBadRequest, "Invalid transaction"},
		{"unexpected", txnID.String(), `{"amount":99}`, userID, errors.New("boom"), http.StatusInternalServerError, "Failed to update transaction"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotID uuid.UUID
			var gotReq dto.UpdateTransactionRequest
			svc := &stubTransactionService{
				updateFn: func(ctx context.Context, uid, id uuid.UUID, req dto.UpdateTransactionRequest) (*dto.TransactionResponse, error) {
					gotUser, gotID, gotReq = uid, id, req
					if tt.svcErr != nil {
						return nil, tt.svcErr
					}
					return &dto.TransactionResponse{ID: id, UserID: uid, Amount: *req.Amount}, nil
				},
			}
			h := NewTransactionHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()

			h.UpdateTransaction(rr, txnTestReq(http.MethodPut, "/transactions/"+tt.id, tt.body, tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				txnAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, txnID, gotID)
			require.NotNil(t, gotReq.Tags, "explicit [] must reach the service so it clears the tags")
			assert.Empty(t, *gotReq.Tags)
			body := decodeJSON(t, rr)
			assert.Equal(t, float64(99), body["data"].(map[string]interface{})["amount"])
		})
	}
}

func TestTransactionHandler_Delete(t *testing.T) {
	userID, txnID := uuid.New(), uuid.New()
	tests := []struct {
		name       string
		id         string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", txnID.String(), userID, nil, http.StatusOK, ""},
		{"invalid uuid", "nope", userID, nil, http.StatusBadRequest, "Invalid ID format"},
		{"unauthenticated", txnID.String(), uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized"},
		{"not found", txnID.String(), userID, gorm.ErrRecordNotFound, http.StatusNotFound, "Transaction not found"},
		{"unexpected", txnID.String(), userID, errors.New("boom"), http.StatusInternalServerError, "Failed to delete transaction"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotID uuid.UUID
			svc := &stubTransactionService{
				deleteFn: func(ctx context.Context, uid, id uuid.UUID) error {
					gotUser, gotID = uid, id
					return tt.svcErr
				},
			}
			h := NewTransactionHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()

			h.DeleteTransaction(rr, txnTestReq(http.MethodDelete, "/transactions/"+tt.id, "", tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				txnAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, txnID, gotID)
			assert.JSONEq(t, `{"status":200,"success":true,"message":"Transaction deleted successfully","data":null}`, rr.Body.String())
		})
	}
}

func TestTransactionHandler_Restore(t *testing.T) {
	userID, txnID := uuid.New(), uuid.New()
	tests := []struct {
		name       string
		id         string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", txnID.String(), userID, nil, http.StatusOK, ""},
		{"invalid uuid", "nope", userID, nil, http.StatusBadRequest, "Invalid ID format"},
		{"unauthenticated", txnID.String(), uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized"},
		{"no deleted transaction", txnID.String(), userID, fmt.Errorf("restore: %w", constant.ErrNotFound), http.StatusNotFound, "Deleted transaction not found"},
		{"unexpected", txnID.String(), userID, errors.New("boom"), http.StatusInternalServerError, "Failed to restore transaction"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotID uuid.UUID
			svc := &stubTransactionService{
				restoreFn: func(ctx context.Context, uid, id uuid.UUID) (*dto.TransactionResponse, error) {
					gotUser, gotID = uid, id
					if tt.svcErr != nil {
						return nil, tt.svcErr
					}
					return &dto.TransactionResponse{ID: id, UserID: uid}, nil
				},
			}
			h := NewTransactionHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()

			h.RestoreTransaction(rr, txnTestReq(http.MethodPost, "/transactions/"+tt.id+"/restore", "", tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				txnAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, txnID, gotID)
			body := decodeJSON(t, rr)
			assert.Equal(t, true, body["success"])
			assert.Equal(t, txnID.String(), body["data"].(map[string]interface{})["id"])
		})
	}
}

func TestTransactionHandler_ListTags(t *testing.T) {
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		var gotUser uuid.UUID
		svc := &stubTransactionService{
			tagsFn: func(ctx context.Context, uid uuid.UUID) ([]dto.TagSummary, error) {
				gotUser = uid
				return []dto.TagSummary{{Name: "Đà Lạt", Count: 3, TotalExpense: 500, LastUsed: "2026-11-04"}}, nil
			},
		}
		h := NewTransactionHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.ListTags(rr, txnTestReq(http.MethodGet, "/tags", "", userID, ""))

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, userID, gotUser)
		var resp TagListResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.True(t, resp.Success)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "Đà Lạt", resp.Data[0].Name)
		assert.Equal(t, int64(3), resp.Data[0].Count)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		svc := &stubTransactionService{}
		h := NewTransactionHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.ListTags(rr, txnTestReq(http.MethodGet, "/tags", "", uuid.Nil, ""))
		txnAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
		assert.Zero(t, svc.calls)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &stubTransactionService{
			tagsFn: func(ctx context.Context, uid uuid.UUID) ([]dto.TagSummary, error) {
				return nil, errors.New("boom")
			},
		}
		h := NewTransactionHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.ListTags(rr, txnTestReq(http.MethodGet, "/tags", "", userID, ""))
		txnAssertError(t, rr, http.StatusInternalServerError, "Failed to list tags")
	})
}

// A transaction without transactionDate is rejected before the service, instead
// of being stored as 0001-01-01 (`required` on the CustomTime struct needs
// validator.WithRequiredStructEnabled()).
func TestTransactionHandler_Create_MissingDateRejected(t *testing.T) {
	svc := &stubTransactionService{
		createFn: func(ctx context.Context, uid uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
			return &dto.TransactionResponse{ID: uuid.New(), UserID: uid}, nil
		},
	}
	h := NewTransactionHandler(svc, zap.NewNop())
	rr := httptest.NewRecorder()
	body := `{"categoryId":"550e8400-e29b-41d4-a716-446655440000","amount":1,"type":"EXPENSE"}`

	h.CreateTransaction(rr, txnTestReq(http.MethodPost, "/transactions", body, uuid.New(), ""))

	txnAssertError(t, rr, http.StatusBadRequest, "Validation failed")
	assert.Zero(t, svc.calls)
}
