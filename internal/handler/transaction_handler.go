package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

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

type TransactionHandler struct {
	transactionService service.TransactionService
	validator          *validator.Validate
	log                *zap.Logger
}

func NewTransactionHandler(transactionService service.TransactionService, log *zap.Logger) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
		validator:          validator.New(),
		log:                log,
	}
}

// CreateTransaction creates a new transaction
// @Summary Create a new transaction
// @Description Create a new transaction (Income or Expense)
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateTransactionRequest true "Create transaction request"
// @Success 201 {object} response.BaseResponse[dto.TransactionResponse]
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /transactions [post]
func (h *TransactionHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	transaction, err := h.transactionService.CreateTransaction(r.Context(), userID, req)
	if err != nil {
		h.log.Error("failed to create transaction", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response.BaseResponse[dto.TransactionResponse]{
		Status:  http.StatusCreated,
		Success: true,
		Data:    *transaction,
	})
}

// GetTransaction returns a single transaction
// @Summary Get a transaction by ID
// @Description Get a transaction by ID
// @Tags transactions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Transaction ID"
// @Success 200 {object} response.BaseResponse[dto.TransactionResponse]
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /transactions/{id} [get]
func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	transaction, err := h.transactionService.GetTransaction(r.Context(), userID, id)
	if err != nil {
		if err == constant.ErrNotFound {
			http.Error(w, "Transaction not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to get transaction", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.TransactionResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *transaction,
	})
}

// ListTransactions returns a list of transactions
// @Summary List transactions
// @Description List transactions
// @Tags transactions
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Page limit"
// @Success 200 {object} response.PaginationResponse[dto.TransactionResponse]
// @Failure 500 {object} response.ErrorResponse
// @Router /transactions [get]
func (h *TransactionHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 10
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	transactions, total, err := h.transactionService.ListTransactions(r.Context(), userID, page, limit)
	if err != nil {
		h.log.Error("failed to list transactions", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.PaginationResponse[dto.TransactionResponse]{
		Status:  http.StatusOK,
		Success: true,
		Items:   transactions,
		Total:   int(total),
		Page:    page,
		Limit:   limit,
	})
}

// UpdateTransaction updates an existing transaction
// @Summary Update a transaction
// @Description Update a transaction
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Transaction ID"
// @Param request body dto.UpdateTransactionRequest true "Update transaction request"
// @Success 200 {object} response.BaseResponse[dto.TransactionResponse]
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /transactions/{id} [put]
func (h *TransactionHandler) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	var req dto.UpdateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	transaction, err := h.transactionService.UpdateTransaction(r.Context(), userID, id, req)
	if err != nil {
		h.log.Error("failed to update transaction", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.TransactionResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *transaction,
	})
}

// DeleteTransaction deletes a transaction
// @Summary Delete a transaction
// @Description Delete a transaction
// @Tags transactions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Transaction ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /transactions/{id} [delete]
func (h *TransactionHandler) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	if err := h.transactionService.DeleteTransaction(r.Context(), userID, id); err != nil {
		h.log.Error("failed to delete transaction", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Transaction deleted successfully"})
}
