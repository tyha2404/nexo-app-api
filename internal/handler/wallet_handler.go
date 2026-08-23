package handler

import (
	"encoding/json"
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
	svc       service.WalletService
	validator *validator.Validate
	log       *zap.Logger
}

func NewWalletHandler(svc service.WalletService, log *zap.Logger) *WalletHandler {
	return &WalletHandler{
		svc:       svc,
		validator: validator.New(),
		log:       log,
	}
}

// GetWallets godoc
// @Summary List user wallets
// @Description Retrieve summary and all wallets of the user
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	res, err := h.svc.GetWallets(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
// @Description Create a new wallet for the user
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req dto.CreateWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.CreateWallet(r.Context(), user.ID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	walletIDStr := chi.URLParam(r, "id")
	walletID, err := uuid.Parse(walletIDStr)
	if err != nil {
		http.Error(w, "Invalid wallet ID", http.StatusBadRequest)
		return
	}

	res, err := h.svc.GetWalletByID(r.Context(), user.ID, walletID)
	if err != nil {
		if err == service.ErrWalletNotFound {
			http.Error(w, "Wallet not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
// @Description Update an existing wallet
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	walletIDStr := chi.URLParam(r, "id")
	walletID, err := uuid.Parse(walletIDStr)
	if err != nil {
		http.Error(w, "Invalid wallet ID", http.StatusBadRequest)
		return
	}

	var req dto.UpdateWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.UpdateWallet(r.Context(), user.ID, walletID, req)
	if err != nil {
		if err == service.ErrWalletNotFound {
			http.Error(w, "Wallet not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	walletIDStr := chi.URLParam(r, "id")
	walletID, err := uuid.Parse(walletIDStr)
	if err != nil {
		http.Error(w, "Invalid wallet ID", http.StatusBadRequest)
		return
	}

	if err := h.svc.DeleteWallet(r.Context(), user.ID, walletID); err != nil {
		if err == service.ErrWalletNotFound {
			http.Error(w, "Wallet not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
// @Description Transfer funds from one wallet to another
// @Tags Wallets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.TransferMoneyRequest true "Transfer money payload"
// @Success 200 {object} dto.WalletTransferResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /wallets/transfer [post]
func (h *WalletHandler) TransferMoney(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req dto.TransferMoneyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.TransferMoney(r.Context(), user.ID, req)
	if err != nil {
		if err == service.ErrSameWalletTransfer {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
// @Description Automatically divide income amount into allocation wallets
// @Tags Wallets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.AutoAllocateRequest true "Auto allocate payload"
// @Success 200 {object} dto.AutoAllocateResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /wallets/auto-allocate [post]
func (h *WalletHandler) AutoAllocateIncome(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req dto.AutoAllocateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.AutoAllocateIncome(r.Context(), user.ID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
