package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type WalletHandler struct {
	svc          service.WalletService
	validator    *validator.Validate
	log          *zap.Logger
	errorHandler *ErrorHandler
}

func NewWalletHandler(svc service.WalletService, log *zap.Logger) *WalletHandler {
	return &WalletHandler{
		svc:          svc,
		validator:    validator.New(),
		log:          log,
		errorHandler: NewErrorHandler(log),
	}
}

// GetWallets godoc
// @Summary List user wallets
// @Description Retrieve summary and all wallets of the user. totalBalance only sums included wallets in baseCurrency (VND); totalsByCurrency lists each currency separately (no FX conversion).
// @Tags Wallets
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.WalletSummaryResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /wallets [get]
func (h *WalletHandler) GetWallets(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "get_wallets")
		return
	}

	res, err := h.svc.GetWallets(r.Context(), user.ID)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get wallets", "get_wallets")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.WalletSummaryResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    res,
	})
}

// CreateWallet godoc
// @Summary Create a wallet
// @Description Create a new wallet for the user. 400 if the total allocationPercent of the user's wallets would exceed 100.
// @Tags Wallets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateWalletRequest true "Create wallet payload"
// @Success 201 {object} dto.WalletResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /wallets [post]
func (h *WalletHandler) CreateWallet(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "create_wallet")
		return
	}

	var req dto.CreateWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "create_wallet")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "create_wallet")
		return
	}

	res, err := h.svc.CreateWallet(r.Context(), user.ID, req)
	if err != nil {
		if errors.Is(err, service.ErrAllocationPercentExceeded) || errors.Is(err, service.ErrInvalidAllocationPercent) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Tổng tỷ lệ phân bổ của các ví không được vượt quá 100%", "create_wallet")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to create wallet", "create_wallet")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.WalletResponse]{
		Status:  http.StatusCreated,
		Success: true,
		Message: "Wallet created successfully",
		Data:    res,
	})
}

// GetWalletByID godoc
// @Summary Get wallet details
// @Description Get details of a specific wallet
// @Tags Wallets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Wallet ID"
// @Success 200 {object} dto.WalletResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /wallets/{id} [get]
func (h *WalletHandler) GetWalletByID(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "get_wallet_by_id")
		return
	}

	walletIDStr := chi.URLParam(r, "id")
	walletID, err := uuid.Parse(walletIDStr)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid wallet ID", "get_wallet_by_id")
		return
	}

	res, err := h.svc.GetWalletByID(r.Context(), user.ID, walletID)
	if err != nil {
		if errors.Is(err, service.ErrWalletNotFound) || IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Wallet not found", "get_wallet_by_id")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get wallet", "get_wallet_by_id")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.WalletResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    res,
	})
}

// UpdateWallet godoc
// @Summary Update wallet
// @Description Update an existing wallet. Balance is only written when sent. 400 if the total allocationPercent would exceed 100.
// @Tags Wallets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Wallet ID"
// @Param request body dto.UpdateWalletRequest true "Update wallet payload"
// @Success 200 {object} dto.WalletResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /wallets/{id} [put]
func (h *WalletHandler) UpdateWallet(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "update_wallet")
		return
	}

	walletIDStr := chi.URLParam(r, "id")
	walletID, err := uuid.Parse(walletIDStr)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid wallet ID", "update_wallet")
		return
	}

	var req dto.UpdateWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "update_wallet")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "update_wallet")
		return
	}

	res, err := h.svc.UpdateWallet(r.Context(), user.ID, walletID, req)
	if err != nil {
		if errors.Is(err, service.ErrWalletNotFound) || IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Wallet not found", "update_wallet")
			return
		}
		if errors.Is(err, service.ErrAllocationPercentExceeded) || errors.Is(err, service.ErrInvalidAllocationPercent) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Tổng tỷ lệ phân bổ của các ví không được vượt quá 100%", "update_wallet")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to update wallet", "update_wallet")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.WalletResponse]{
		Status:  http.StatusOK,
		Success: true,
		Message: "Wallet updated successfully",
		Data:    res,
	})
}

// DeleteWallet godoc
// @Summary Delete wallet
// @Description Delete a wallet by ID
// @Tags Wallets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Wallet ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /wallets/{id} [delete]
func (h *WalletHandler) DeleteWallet(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "delete_wallet")
		return
	}

	walletIDStr := chi.URLParam(r, "id")
	walletID, err := uuid.Parse(walletIDStr)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid wallet ID", "delete_wallet")
		return
	}

	if err := h.svc.DeleteWallet(r.Context(), user.ID, walletID); err != nil {
		if errors.Is(err, service.ErrWalletNotFound) || IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Wallet not found", "delete_wallet")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to delete wallet", "delete_wallet")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[map[string]string]{
		Status:  http.StatusOK,
		Success: true,
		Message: "Wallet deleted successfully",
	})
}

// TransferMoney godoc
// @Summary Transfer money between wallets
// @Description Transfer funds from one wallet to another. Both wallets are row-locked; amount must be > 0 and fee >= 0; the source must cover amount + fee.
// @Tags Wallets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.TransferMoneyRequest true "Transfer money payload"
// @Success 200 {object} dto.WalletTransferResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Wallet not found"
// @Failure 500 {object} response.ErrorResponse
// @Router /wallets/transfer [post]
func (h *WalletHandler) TransferMoney(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "transfer_money")
		return
	}

	var req dto.TransferMoneyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "transfer_money")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "transfer_money")
		return
	}

	res, err := h.svc.TransferMoney(r.Context(), user.ID, req)
	if err != nil {
		if errors.Is(err, service.ErrInsufficientBalance) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Số dư ví nguồn không đủ", "transfer_money")
			return
		}
		if errors.Is(err, service.ErrSameWalletTransfer) || errors.Is(err, service.ErrInvalidTransfer) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid transfer", "transfer_money")
			return
		}
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Wallet not found", "transfer_money")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to transfer money", "transfer_money")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.WalletTransferResponse]{
		Status:  http.StatusOK,
		Success: true,
		Message: "Money transferred successfully",
		Data:    res,
	})
}

// AutoAllocateIncome godoc
// @Summary Auto allocate income to wallets
// @Description Automatically divide income amount into allocation wallets (relative balance updates in one DB transaction). 400 when no wallet has an allocation percent or the total exceeds 100%.
// @Tags Wallets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.AutoAllocateRequest true "Auto allocate payload"
// @Success 200 {object} dto.AutoAllocateResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Wallet not found"
// @Failure 500 {object} response.ErrorResponse
// @Router /wallets/auto-allocate [post]
func (h *WalletHandler) AutoAllocateIncome(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "auto_allocate_income")
		return
	}

	var req dto.AutoAllocateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "auto_allocate_income")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "auto_allocate_income")
		return
	}

	res, err := h.svc.AutoAllocateIncome(r.Context(), user.ID, req)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Source wallet not found", "auto_allocate_income")
			return
		}
		if errors.Is(err, constant.ErrInvalidInput) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Cấu hình phân bổ không hợp lệ (chưa có ví phân bổ hoặc tổng tỷ lệ vượt 100%)", "auto_allocate_income")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to auto allocate income", "auto_allocate_income")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.AutoAllocateResponse]{
		Status:  http.StatusOK,
		Success: true,
		Message: "Income auto-allocated successfully",
		Data:    res,
	})
}
