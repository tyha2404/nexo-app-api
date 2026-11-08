package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type stubCCStatementService struct {
	service.CreditCardStatementService
	calls       int
	gotUser     uuid.UUID
	gotID       uuid.UUID
	gotWalletID *uuid.UUID
	gotYear     *int
	gotMonth    *int
	gotCreate   dto.CreateStatementRequest
	gotUpdate   dto.UpdateStatementRequest
	gotPay      dto.PayStatementRequest
	err         error
}

func (s *stubCCStatementService) ListStatements(ctx context.Context, userID uuid.UUID, walletID *uuid.UUID, year *int, month *int) (*dto.StatementListResponse, error) {
	s.calls++
	s.gotUser, s.gotWalletID, s.gotYear, s.gotMonth = userID, walletID, year, month
	if s.err != nil {
		return nil, s.err
	}
	return &dto.StatementListResponse{Items: []dto.StatementResponse{{ID: uuid.New(), UserID: userID}}, Total: 1, TotalBalance: 500, TotalPaid: 100, TotalRemaining: 400}, nil
}

func (s *stubCCStatementService) CreateStatement(ctx context.Context, userID uuid.UUID, req dto.CreateStatementRequest) (*dto.StatementResponse, error) {
	s.calls++
	s.gotUser, s.gotCreate = userID, req
	if s.err != nil {
		return nil, s.err
	}
	return &dto.StatementResponse{ID: uuid.New(), UserID: userID, WalletID: req.WalletID, StatementMonth: req.StatementMonth, StatementYear: req.StatementYear}, nil
}

func (s *stubCCStatementService) GetStatementByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*dto.StatementResponse, error) {
	s.calls++
	s.gotUser, s.gotID = userID, id
	if s.err != nil {
		return nil, s.err
	}
	return &dto.StatementResponse{ID: id, UserID: userID}, nil
}

func (s *stubCCStatementService) UpdateStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID, req dto.UpdateStatementRequest) (*dto.StatementResponse, error) {
	s.calls++
	s.gotUser, s.gotID, s.gotUpdate = userID, id, req
	if s.err != nil {
		return nil, s.err
	}
	return &dto.StatementResponse{ID: id, UserID: userID}, nil
}

func (s *stubCCStatementService) PayStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID, req dto.PayStatementRequest) (*dto.StatementResponse, error) {
	s.calls++
	s.gotUser, s.gotID, s.gotPay = userID, id, req
	if s.err != nil {
		return nil, s.err
	}
	return &dto.StatementResponse{ID: id, UserID: userID, PaidAmount: req.Amount}, nil
}

func (s *stubCCStatementService) DeleteStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	s.calls++
	s.gotUser, s.gotID = userID, id
	return s.err
}

func TestCreditCardStatementHandler_ListStatements_QueryParsing(t *testing.T) {
	userID := uuid.New()
	walletID := uuid.New()
	intp := func(v int) *int { return &v }
	tests := []struct {
		name       string
		query      string
		wantWallet *uuid.UUID
		wantYear   *int
		wantMonth  *int
	}{
		{"no filters", "", nil, nil, nil},
		{"all filters", "?walletId=" + walletID.String() + "&year=2026&month=8", &walletID, intp(2026), intp(8)},
		{"invalid wallet ignored", "?walletId=nope&year=2025", nil, intp(2025), nil},
		{"non-numeric year/month ignored", "?year=abc&month=x", nil, nil, nil},
		{"zero and negative ignored", "?year=0&month=-1", nil, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubCCStatementService{}
			rr := httptest.NewRecorder()
			NewCreditCardStatementHandler(svc, zap.NewNop()).ListStatements(rr, debtTestReq(http.MethodGet, "/credit-card-statements"+tt.query, "", &userID, ""))

			body := debtAssertSuccess(t, rr, http.StatusOK)
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, tt.wantWallet, svc.gotWalletID)
			assert.Equal(t, tt.wantYear, svc.gotYear)
			assert.Equal(t, tt.wantMonth, svc.gotMonth)
			data := body["data"].(map[string]interface{})
			assert.Equal(t, float64(1), data["total"])
			assert.Len(t, data["items"], 1)
			assert.Equal(t, float64(400), data["totalRemaining"])
		})
	}
}

func TestCreditCardStatementHandler_ListStatements_Errors(t *testing.T) {
	svc := &stubCCStatementService{}
	rr := httptest.NewRecorder()
	NewCreditCardStatementHandler(svc, zap.NewNop()).ListStatements(rr, debtTestReq(http.MethodGet, "/credit-card-statements", "", nil, ""))
	debtAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
	assert.Zero(t, svc.calls)

	userID := uuid.New()
	svc = &stubCCStatementService{err: errors.New("db down")}
	rr = httptest.NewRecorder()
	NewCreditCardStatementHandler(svc, zap.NewNop()).ListStatements(rr, debtTestReq(http.MethodGet, "/credit-card-statements", "", &userID, ""))
	debtAssertError(t, rr, http.StatusInternalServerError, "Failed to list statements")
}

func TestCreditCardStatementHandler_CreateStatement(t *testing.T) {
	userID := uuid.New()
	walletID := uuid.New()
	valid := fmt.Sprintf(`{"walletId":%q,"statementMonth":8,"statementYear":2026,"statementDate":"2026-08-20","dueDate":"2026-09-05","statementBalance":5000000,"minimumPayment":250000}`, walletID)
	tests := []struct {
		name       string
		anonymous  bool
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
		wantCalled bool
	}{
		{"success", false, valid, nil, http.StatusCreated, "", true},
		{"unauthenticated", true, valid, nil, http.StatusUnauthorized, "Unauthorized", false},
		{"malformed json", false, `{`, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"invalid wallet uuid", false, `{"walletId":"nope"}`, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"month out of range", false, fmt.Sprintf(`{"walletId":%q,"statementMonth":13,"statementYear":2026,"statementDate":"2026-08-20","dueDate":"2026-09-05","statementBalance":1}`, walletID), nil, http.StatusBadRequest, "Validation failed", false},
		{"missing dates", false, fmt.Sprintf(`{"walletId":%q,"statementMonth":8,"statementYear":2026,"statementBalance":1}`, walletID), nil, http.StatusBadRequest, "Validation failed", false},
		{"missing wallet", false, `{"statementMonth":8,"statementYear":2026,"statementDate":"2026-08-20","dueDate":"2026-09-05","statementBalance":1}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"wallet not found", false, valid, service.ErrWalletNotFound, http.StatusNotFound, "Wallet not found", true},
		{"invalid date", false, valid, service.ErrInvalidDate, http.StatusBadRequest, "Invalid statement date", true},
		{"duplicate period", false, valid, fmt.Errorf("statement: %w", constant.ErrAlreadyExists), http.StatusConflict, "Kỳ sao kê này đã tồn tại", true},
		{"internal error", false, valid, errors.New("boom"), http.StatusInternalServerError, "Failed to create statement", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubCCStatementService{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewCreditCardStatementHandler(svc, zap.NewNop()).CreateStatement(rr, debtTestReq(http.MethodPost, "/credit-card-statements", tt.body, user, ""))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			if tt.wantStatus != http.StatusCreated {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusCreated)
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, walletID, svc.gotCreate.WalletID)
			assert.Equal(t, 8, svc.gotCreate.StatementMonth)
			data := body["data"].(map[string]interface{})
			assert.Equal(t, walletID.String(), data["walletId"])
		})
	}
}

func TestCreditCardStatementHandler_GetStatementByID(t *testing.T) {
	userID := uuid.New()
	id := uuid.New()
	tests := []struct {
		name       string
		anonymous  bool
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", false, id.String(), nil, http.StatusOK, ""},
		{"unauthenticated", true, id.String(), nil, http.StatusUnauthorized, "Unauthorized"},
		{"invalid id", false, "x", nil, http.StatusBadRequest, "Invalid statement ID"},
		{"not found", false, id.String(), service.ErrStatementNotFound, http.StatusNotFound, "Statement not found"},
		{"internal error", false, id.String(), errors.New("boom"), http.StatusInternalServerError, "Failed to get statement"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubCCStatementService{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewCreditCardStatementHandler(svc, zap.NewNop()).GetStatementByID(rr, debtTestReq(http.MethodGet, "/credit-card-statements/"+tt.id, "", user, tt.id))
			if tt.wantStatus != http.StatusOK {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusOK)
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, id, svc.gotID)
			assert.Equal(t, id.String(), body["data"].(map[string]interface{})["id"])
		})
	}
}

func TestCreditCardStatementHandler_UpdateStatement(t *testing.T) {
	userID := uuid.New()
	id := uuid.New()
	tests := []struct {
		name       string
		anonymous  bool
		id         string
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
		wantCalled bool
	}{
		{"success", false, id.String(), `{"statementBalance":4200000,"note":"x"}`, nil, http.StatusOK, "", true},
		{"unauthenticated", true, id.String(), `{}`, nil, http.StatusUnauthorized, "Unauthorized", false},
		{"invalid id", false, "bad", `{}`, nil, http.StatusBadRequest, "Invalid statement ID", false},
		{"malformed json", false, id.String(), `[`, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"month out of range", false, id.String(), `{"statementMonth":0}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"negative balance", false, id.String(), `{"statementBalance":-1}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"not found", false, id.String(), `{}`, service.ErrStatementNotFound, http.StatusNotFound, "Statement not found", true},
		{"internal error", false, id.String(), `{}`, errors.New("boom"), http.StatusInternalServerError, "Failed to update statement", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubCCStatementService{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewCreditCardStatementHandler(svc, zap.NewNop()).UpdateStatement(rr, debtTestReq(http.MethodPut, "/credit-card-statements/"+tt.id, tt.body, user, tt.id))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			if tt.wantStatus != http.StatusOK {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			debtAssertSuccess(t, rr, http.StatusOK)
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, id, svc.gotID)
			require.NotNil(t, svc.gotUpdate.StatementBalance)
			assert.Equal(t, 4200000.0, *svc.gotUpdate.StatementBalance)
		})
	}
}

func TestCreditCardStatementHandler_PayStatement(t *testing.T) {
	userID := uuid.New()
	id := uuid.New()
	source := uuid.New()
	valid := fmt.Sprintf(`{"amount":1000000,"sourceWalletId":%q}`, source)
	tests := []struct {
		name       string
		anonymous  bool
		id         string
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
		wantCalled bool
	}{
		{"success", false, id.String(), valid, nil, http.StatusOK, "", true},
		{"success without source wallet", false, id.String(), `{"amount":5}`, nil, http.StatusOK, "", true},
		{"unauthenticated", true, id.String(), valid, nil, http.StatusUnauthorized, "Unauthorized", false},
		{"invalid id", false, "nope", valid, nil, http.StatusBadRequest, "Invalid statement ID", false},
		{"malformed json", false, id.String(), `{"amount":`, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"zero amount", false, id.String(), `{"amount":0}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"negative amount", false, id.String(), `{"amount":-10}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"statement not found", false, id.String(), valid, service.ErrStatementNotFound, http.StatusNotFound, "Statement not found", true},
		{"already paid", false, id.String(), valid, service.ErrStatementAlreadyPaid, http.StatusConflict, "Kỳ sao kê đã được thanh toán đủ", true},
		{"over remaining", false, id.String(), valid, service.ErrStatementInvalidPayment, http.StatusBadRequest, "Số tiền thanh toán không hợp lệ", true},
		{"source wallet not owned", false, id.String(), valid, fmt.Errorf("source wallet: %w", constant.ErrNotFound), http.StatusNotFound, "Wallet not found", true},
		{"internal error", false, id.String(), valid, errors.New("deadlock"), http.StatusInternalServerError, "Failed to pay statement", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubCCStatementService{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewCreditCardStatementHandler(svc, zap.NewNop()).PayStatement(rr, debtTestReq(http.MethodPost, "/credit-card-statements/"+tt.id+"/pay", tt.body, user, tt.id))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			if tt.wantStatus != http.StatusOK {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusOK)
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, id, svc.gotID)
			assert.Equal(t, svc.gotPay.Amount, body["data"].(map[string]interface{})["paidAmount"])
		})
	}

	t.Run("source wallet forwarded", func(t *testing.T) {
		svc := &stubCCStatementService{}
		rr := httptest.NewRecorder()
		NewCreditCardStatementHandler(svc, zap.NewNop()).PayStatement(rr, debtTestReq(http.MethodPost, "/credit-card-statements/"+id.String()+"/pay", valid, &userID, id.String()))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.NotNil(t, svc.gotPay.SourceWalletID)
		assert.Equal(t, source, *svc.gotPay.SourceWalletID)
	})
}

func TestCreditCardStatementHandler_DeleteStatement(t *testing.T) {
	userID := uuid.New()
	id := uuid.New()
	tests := []struct {
		name       string
		anonymous  bool
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
		wantCalled bool
	}{
		{"success", false, id.String(), nil, http.StatusOK, "Xóa kỳ sao kê thành công", true},
		{"unauthenticated", true, id.String(), nil, http.StatusUnauthorized, "Unauthorized", false},
		{"invalid id", false, "zzz", nil, http.StatusBadRequest, "Invalid statement ID", false},
		{"not found or another user's", false, id.String(), constant.ErrNotFound, http.StatusNotFound, "Statement not found", true},
		{"internal error", false, id.String(), errors.New("boom"), http.StatusInternalServerError, "Failed to delete statement", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubCCStatementService{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewCreditCardStatementHandler(svc, zap.NewNop()).DeleteStatement(rr, debtTestReq(http.MethodDelete, "/credit-card-statements/"+tt.id, "", user, tt.id))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			if tt.wantStatus != http.StatusOK {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusOK)
			assert.Equal(t, tt.wantMsg, body["message"])
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, id, svc.gotID)
		})
	}
}

// A context user without an id (never set by the auth middleware) is not a user.
func TestUserScopedHandlers_RejectNilUserID(t *testing.T) {
	nilUser := uuid.Nil
	id := uuid.New().String()
	check := func(name string, serve http.HandlerFunc) {
		t.Run(name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			serve(rr, debtTestReq(http.MethodGet, "/x/"+id, "", &nilUser, id))
			assert.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
		})
	}
	stmt := NewCreditCardStatementHandler(&stubCCStatementService{}, zap.NewNop())
	check("statements", stmt.DeleteStatement)
	check("statement list", stmt.ListStatements)
}
