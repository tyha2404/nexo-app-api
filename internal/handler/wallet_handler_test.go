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
	"gorm.io/gorm"
)

type stubWalletService struct {
	service.WalletService
	listFn     func(ctx context.Context, userID uuid.UUID) (*dto.WalletSummaryResponse, error)
	createFn   func(ctx context.Context, userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error)
	getFn      func(ctx context.Context, userID, id uuid.UUID) (*dto.WalletResponse, error)
	updateFn   func(ctx context.Context, userID, id uuid.UUID, req dto.UpdateWalletRequest) (*dto.WalletResponse, error)
	deleteFn   func(ctx context.Context, userID, id uuid.UUID) error
	transferFn func(ctx context.Context, userID uuid.UUID, req dto.TransferMoneyRequest) (*dto.WalletTransferResponse, error)
	allocateFn func(ctx context.Context, userID uuid.UUID, req dto.AutoAllocateRequest) (*dto.AutoAllocateResponse, error)
	calls      int
}

func (s *stubWalletService) GetWallets(ctx context.Context, userID uuid.UUID) (*dto.WalletSummaryResponse, error) {
	s.calls++
	return s.listFn(ctx, userID)
}

func (s *stubWalletService) CreateWallet(ctx context.Context, userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error) {
	s.calls++
	return s.createFn(ctx, userID, req)
}

func (s *stubWalletService) GetWalletByID(ctx context.Context, userID, id uuid.UUID) (*dto.WalletResponse, error) {
	s.calls++
	return s.getFn(ctx, userID, id)
}

func (s *stubWalletService) UpdateWallet(ctx context.Context, userID, id uuid.UUID, req dto.UpdateWalletRequest) (*dto.WalletResponse, error) {
	s.calls++
	return s.updateFn(ctx, userID, id, req)
}

func (s *stubWalletService) DeleteWallet(ctx context.Context, userID, id uuid.UUID) error {
	s.calls++
	return s.deleteFn(ctx, userID, id)
}

func (s *stubWalletService) TransferMoney(ctx context.Context, userID uuid.UUID, req dto.TransferMoneyRequest) (*dto.WalletTransferResponse, error) {
	s.calls++
	return s.transferFn(ctx, userID, req)
}

func (s *stubWalletService) AutoAllocateIncome(ctx context.Context, userID uuid.UUID, req dto.AutoAllocateRequest) (*dto.AutoAllocateResponse, error) {
	s.calls++
	return s.allocateFn(ctx, userID, req)
}

func TestWalletHandler_GetWallets(t *testing.T) {
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		var gotUser uuid.UUID
		svc := &stubWalletService{listFn: func(ctx context.Context, uid uuid.UUID) (*dto.WalletSummaryResponse, error) {
			gotUser = uid
			return &dto.WalletSummaryResponse{TotalBalance: 1500, BaseCurrency: "VND", TotalWallets: 2, Wallets: []dto.WalletResponse{{ID: uuid.New()}, {ID: uuid.New()}}}, nil
		}}
		h := NewWalletHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.GetWallets(rr, txnTestReq(http.MethodGet, "/wallets", "", userID, ""))

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, userID, gotUser)
		body := decodeJSON(t, rr)
		assert.Equal(t, true, body["success"])
		data := body["data"].(map[string]interface{})
		assert.Equal(t, float64(1500), data["totalBalance"])
		assert.Len(t, data["wallets"], 2)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		svc := &stubWalletService{}
		rr := httptest.NewRecorder()
		NewWalletHandler(svc, zap.NewNop()).GetWallets(rr, txnTestReq(http.MethodGet, "/wallets", "", uuid.Nil, ""))
		txnAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
		assert.Zero(t, svc.calls)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &stubWalletService{listFn: func(ctx context.Context, uid uuid.UUID) (*dto.WalletSummaryResponse, error) {
			return nil, errors.New("boom")
		}}
		rr := httptest.NewRecorder()
		NewWalletHandler(svc, zap.NewNop()).GetWallets(rr, txnTestReq(http.MethodGet, "/wallets", "", userID, ""))
		txnAssertError(t, rr, http.StatusInternalServerError, "Failed to get wallets")
	})
}

func TestWalletHandler_CreateWallet(t *testing.T) {
	userID := uuid.New()
	const valid = `{"name":"Tiền mặt","type":"CASH","balance":1000,"currency":"VND","allocationPercent":20}`
	const allocMsg = "Tổng tỷ lệ phân bổ của các ví không được vượt quá 100%"
	tests := []struct {
		name       string
		body       string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", valid, userID, nil, http.StatusCreated, ""},
		{"unauthenticated", valid, uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized"},
		{"malformed json", `{"name":`, userID, nil, http.StatusBadRequest, "Invalid request payload"},
		{"missing name", `{"type":"CASH"}`, userID, nil, http.StatusBadRequest, "Validation failed"},
		{"unknown type", `{"name":"x","type":"CRYPTO"}`, userID, nil, http.StatusBadRequest, "Validation failed"},
		{"allocation over 100", `{"name":"x","type":"CASH","allocationPercent":101}`, userID, nil, http.StatusBadRequest, "Validation failed"},
		{"statement day out of range", `{"name":"x","type":"CREDIT","statementDay":32}`, userID, nil, http.StatusBadRequest, "Validation failed"},
		{"allocation total exceeded", valid, userID, service.ErrAllocationPercentExceeded, http.StatusBadRequest, allocMsg},
		{"invalid allocation percent", valid, userID, service.ErrInvalidAllocationPercent, http.StatusBadRequest, allocMsg},
		{"unexpected", valid, userID, errors.New("boom"), http.StatusInternalServerError, "Failed to create wallet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			var gotReq dto.CreateWalletRequest
			svc := &stubWalletService{createFn: func(ctx context.Context, uid uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error) {
				gotUser, gotReq = uid, req
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &dto.WalletResponse{ID: uuid.New(), UserID: uid, Name: req.Name, Type: req.Type, Balance: req.Balance}, nil
			}}
			rr := httptest.NewRecorder()
			NewWalletHandler(svc, zap.NewNop()).CreateWallet(rr, txnTestReq(http.MethodPost, "/wallets", tt.body, tt.user, ""))

			if tt.wantStatus != http.StatusCreated {
				txnAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, "Tiền mặt", gotReq.Name)
			body := decodeJSON(t, rr)
			assert.Equal(t, "Wallet created successfully", body["message"])
			assert.Equal(t, userID.String(), body["data"].(map[string]interface{})["userId"])
		})
	}
}

func TestWalletHandler_GetWalletByID(t *testing.T) {
	userID, walletID := uuid.New(), uuid.New()
	tests := []struct {
		name       string
		id         string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", walletID.String(), userID, nil, http.StatusOK, ""},
		{"unauthenticated", walletID.String(), uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized"},
		{"invalid uuid", "wallet-1", userID, nil, http.StatusBadRequest, "Invalid wallet ID"},
		{"service ErrWalletNotFound", walletID.String(), userID, service.ErrWalletNotFound, http.StatusNotFound, "Wallet not found"},
		{"gorm not found", walletID.String(), userID, gorm.ErrRecordNotFound, http.StatusNotFound, "Wallet not found"},
		{"unexpected", walletID.String(), userID, errors.New("boom"), http.StatusInternalServerError, "Failed to get wallet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotID uuid.UUID
			svc := &stubWalletService{getFn: func(ctx context.Context, uid, id uuid.UUID) (*dto.WalletResponse, error) {
				gotUser, gotID = uid, id
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &dto.WalletResponse{ID: id, UserID: uid}, nil
			}}
			rr := httptest.NewRecorder()
			NewWalletHandler(svc, zap.NewNop()).GetWalletByID(rr, txnTestReq(http.MethodGet, "/wallets/"+tt.id, "", tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				txnAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, walletID, gotID)
			body := decodeJSON(t, rr)
			assert.Equal(t, walletID.String(), body["data"].(map[string]interface{})["id"])
		})
	}
}

func TestWalletHandler_UpdateWallet(t *testing.T) {
	userID, walletID := uuid.New(), uuid.New()
	tests := []struct {
		name       string
		id         string
		body       string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", walletID.String(), `{"name":"Ví mới","balance":0}`, userID, nil, http.StatusOK, ""},
		{"unauthenticated", walletID.String(), `{"name":"x"}`, uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized"},
		{"invalid uuid", "x", `{"name":"x"}`, userID, nil, http.StatusBadRequest, "Invalid wallet ID"},
		{"malformed json", walletID.String(), `[`, userID, nil, http.StatusBadRequest, "Invalid request payload"},
		{"empty name", walletID.String(), `{"name":""}`, userID, nil, http.StatusBadRequest, "Validation failed"},
		{"negative allocation", walletID.String(), `{"allocationPercent":-1}`, userID, nil, http.StatusBadRequest, "Validation failed"},
		{"not found", walletID.String(), `{"name":"x"}`, userID, service.ErrWalletNotFound, http.StatusNotFound, "Wallet not found"},
		{"wrapped not found", walletID.String(), `{"name":"x"}`, userID, fmt.Errorf("w: %w", constant.ErrNotFound), http.StatusNotFound, "Wallet not found"},
		{"allocation exceeded", walletID.String(), `{"allocationPercent":90}`, userID, service.ErrAllocationPercentExceeded, http.StatusBadRequest, "Tổng tỷ lệ phân bổ của các ví không được vượt quá 100%"},
		{"unexpected", walletID.String(), `{"name":"x"}`, userID, errors.New("boom"), http.StatusInternalServerError, "Failed to update wallet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotID uuid.UUID
			var gotReq dto.UpdateWalletRequest
			svc := &stubWalletService{updateFn: func(ctx context.Context, uid, id uuid.UUID, req dto.UpdateWalletRequest) (*dto.WalletResponse, error) {
				gotUser, gotID, gotReq = uid, id, req
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &dto.WalletResponse{ID: id, UserID: uid, Name: *req.Name}, nil
			}}
			rr := httptest.NewRecorder()
			NewWalletHandler(svc, zap.NewNop()).UpdateWallet(rr, txnTestReq(http.MethodPut, "/wallets/"+tt.id, tt.body, tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				txnAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, walletID, gotID)
			require.NotNil(t, gotReq.Balance, "an explicit balance of 0 must reach the service")
			assert.Zero(t, *gotReq.Balance)
			assert.Nil(t, gotReq.Currency, "omitted fields stay nil")
			body := decodeJSON(t, rr)
			assert.Equal(t, "Wallet updated successfully", body["message"])
		})
	}
}

func TestWalletHandler_DeleteWallet(t *testing.T) {
	userID, walletID := uuid.New(), uuid.New()
	tests := []struct {
		name       string
		id         string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", walletID.String(), userID, nil, http.StatusOK, ""},
		{"unauthenticated", walletID.String(), uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized"},
		{"invalid uuid", "x", userID, nil, http.StatusBadRequest, "Invalid wallet ID"},
		{"not found", walletID.String(), userID, service.ErrWalletNotFound, http.StatusNotFound, "Wallet not found"},
		{"unexpected", walletID.String(), userID, errors.New("boom"), http.StatusInternalServerError, "Failed to delete wallet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotID uuid.UUID
			svc := &stubWalletService{deleteFn: func(ctx context.Context, uid, id uuid.UUID) error {
				gotUser, gotID = uid, id
				return tt.svcErr
			}}
			rr := httptest.NewRecorder()
			NewWalletHandler(svc, zap.NewNop()).DeleteWallet(rr, txnTestReq(http.MethodDelete, "/wallets/"+tt.id, "", tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				txnAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, walletID, gotID)
			body := decodeJSON(t, rr)
			assert.Equal(t, true, body["success"])
			assert.Equal(t, "Wallet deleted successfully", body["message"])
		})
	}
}

func TestWalletHandler_TransferMoney(t *testing.T) {
	userID, from, to := uuid.New(), uuid.New(), uuid.New()
	valid := fmt.Sprintf(`{"fromWalletId":%q,"toWalletId":%q,"amount":500000,"fee":1000,"transferDate":"2026-11-02"}`, from, to)
	tests := []struct {
		name       string
		body       string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", valid, userID, nil, http.StatusOK, ""},
		{"unauthenticated", valid, uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized"},
		{"malformed json", `{"amount":`, userID, nil, http.StatusBadRequest, "Invalid request payload"},
		{"invalid wallet uuid", `{"fromWalletId":"abc","toWalletId":"def","amount":1}`, userID, nil, http.StatusBadRequest, "Invalid request payload"},
		{"missing from wallet", fmt.Sprintf(`{"toWalletId":%q,"amount":1}`, to), userID, nil, http.StatusBadRequest, "Validation failed"},
		{"zero amount", fmt.Sprintf(`{"fromWalletId":%q,"toWalletId":%q,"amount":0}`, from, to), userID, nil, http.StatusBadRequest, "Validation failed"},
		{"negative fee", fmt.Sprintf(`{"fromWalletId":%q,"toWalletId":%q,"amount":1,"fee":-1}`, from, to), userID, nil, http.StatusBadRequest, "Validation failed"},
		{"insufficient balance", valid, userID, constant.ErrInsufficientBalance, http.StatusBadRequest, "Số dư ví nguồn không đủ"},
		{"same wallet", valid, userID, service.ErrSameWalletTransfer, http.StatusBadRequest, "Invalid transfer"},
		{"invalid transfer", valid, userID, service.ErrInvalidTransfer, http.StatusBadRequest, "Invalid transfer"},
		{"wallet not found", valid, userID, fmt.Errorf("source wallet not found: %w", gorm.ErrRecordNotFound), http.StatusNotFound, "Wallet not found"},
		{"unexpected", valid, userID, errors.New("boom"), http.StatusInternalServerError, "Failed to transfer money"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			var gotReq dto.TransferMoneyRequest
			svc := &stubWalletService{transferFn: func(ctx context.Context, uid uuid.UUID, req dto.TransferMoneyRequest) (*dto.WalletTransferResponse, error) {
				gotUser, gotReq = uid, req
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &dto.WalletTransferResponse{ID: uuid.New(), UserID: uid, FromWalletID: req.FromWalletID, ToWalletID: req.ToWalletID, Amount: req.Amount, Fee: req.Fee}, nil
			}}
			rr := httptest.NewRecorder()
			NewWalletHandler(svc, zap.NewNop()).TransferMoney(rr, txnTestReq(http.MethodPost, "/wallets/transfer", tt.body, tt.user, ""))

			if tt.wantStatus != http.StatusOK {
				txnAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, from, gotReq.FromWalletID)
			assert.Equal(t, to, gotReq.ToWalletID)
			assert.Equal(t, 1000.0, gotReq.Fee)
			require.NotNil(t, gotReq.TransferDate)
			assert.Equal(t, "2026-11-02", gotReq.TransferDate.Format("2006-01-02"))
			body := decodeJSON(t, rr)
			assert.Equal(t, "Money transferred successfully", body["message"])
			assert.Equal(t, float64(500000), body["data"].(map[string]interface{})["amount"])
		})
	}
}

func TestWalletHandler_AutoAllocateIncome(t *testing.T) {
	userID, source := uuid.New(), uuid.New()
	valid := fmt.Sprintf(`{"sourceWalletId":%q,"incomeAmount":10000000,"preset":"50_30_20"}`, source)
	tests := []struct {
		name       string
		body       string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", valid, userID, nil, http.StatusOK, ""},
		{"unauthenticated", valid, uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized"},
		{"malformed json", `nope`, userID, nil, http.StatusBadRequest, "Invalid request payload"},
		{"zero income", `{"incomeAmount":0}`, userID, nil, http.StatusBadRequest, "Validation failed"},
		{"negative income", `{"incomeAmount":-10}`, userID, nil, http.StatusBadRequest, "Validation failed"},
		{"source wallet not found", valid, userID, fmt.Errorf("source wallet: %w", gorm.ErrRecordNotFound), http.StatusNotFound, "Source wallet not found"},
		{"no allocation wallets", valid, userID, fmt.Errorf("no allocation wallet: %w", constant.ErrInvalidInput), http.StatusBadRequest, "Cấu hình phân bổ không hợp lệ (chưa có ví phân bổ hoặc tổng tỷ lệ vượt 100%)"},
		{"allocation total exceeded", valid, userID, service.ErrAllocationPercentExceeded, http.StatusBadRequest, "Cấu hình phân bổ không hợp lệ (chưa có ví phân bổ hoặc tổng tỷ lệ vượt 100%)"},
		{"unexpected", valid, userID, errors.New("boom"), http.StatusInternalServerError, "Failed to auto allocate income"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			var gotReq dto.AutoAllocateRequest
			svc := &stubWalletService{allocateFn: func(ctx context.Context, uid uuid.UUID, req dto.AutoAllocateRequest) (*dto.AutoAllocateResponse, error) {
				gotUser, gotReq = uid, req
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &dto.AutoAllocateResponse{SourceWalletID: &req.SourceWalletID, IncomeAmount: req.IncomeAmount, TotalAllocated: req.IncomeAmount,
					Allocations: []dto.AllocatedWalletDetail{{WalletID: uuid.New(), AllocationPercent: 100, AllocatedAmount: req.IncomeAmount}}}, nil
			}}
			rr := httptest.NewRecorder()
			NewWalletHandler(svc, zap.NewNop()).AutoAllocateIncome(rr, txnTestReq(http.MethodPost, "/wallets/auto-allocate", tt.body, tt.user, ""))

			if tt.wantStatus != http.StatusOK {
				txnAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, source, gotReq.SourceWalletID)
			assert.Equal(t, "50_30_20", string(gotReq.Preset))
			body := decodeJSON(t, rr)
			assert.Equal(t, "Income auto-allocated successfully", body["message"])
			data := body["data"].(map[string]interface{})
			assert.Equal(t, float64(10000000), data["totalAllocated"])
			assert.Len(t, data["allocations"], 1)
		})
	}
}
