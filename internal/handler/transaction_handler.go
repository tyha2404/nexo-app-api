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
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type TransactionHandler struct {
	transactionService service.TransactionService
	validator          *validator.Validate
	log                *zap.Logger
	errorHandler       *ErrorHandler
}

func NewTransactionHandler(transactionService service.TransactionService, log *zap.Logger) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
		validator:          validator.New(),
		log:                log,
		errorHandler:       NewErrorHandler(log),
	}
}

// CreateTransaction creates a new transaction
// @Summary Create a new transaction
// @Description Create a new transaction (INCOME, EXPENSE or INVESTMENT). When walletId is set the wallet must belong to the user and its balance moves atomically: INCOME adds, EXPENSE and INVESTMENT (HOLDING) subtract, a closed INVESTMENT (SOLD/MATURED/CANCELLED) nets +realizedPnl.
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateTransactionRequest true "Create transaction request"
// @Success 201 {object} dto.TransactionResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Category or wallet not found"
// @Failure 500 {object} response.ErrorResponse
// @Router /transactions [post]
func (h *TransactionHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "create_transaction")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "create_transaction")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "create_transaction")
		return
	}

	transaction, err := h.transactionService.CreateTransaction(r.Context(), user.ID, req)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Category or wallet not found", "create_transaction")
			return
		}
		if errors.Is(err, constant.ErrInvalidInput) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid transaction", "create_transaction")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to create transaction", "create_transaction")
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
// @Success 200 {object} dto.TransactionResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /transactions/{id} [get]
func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "get_transaction")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_transaction")
		return
	}

	transaction, err := h.transactionService.GetTransaction(r.Context(), user.ID, id)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Transaction not found", "get_transaction")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get transaction", "get_transaction")
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
// @Param keyword query string false "Search transaction descriptions (case-insensitive literal text)"
// @Description List transactions
// @Tags transactions
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Page limit (default 10, max 100)"
// @Success 200 {array} dto.TransactionResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /transactions [get]
func (h *TransactionHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	page, limit := ParsePagination(r)

	filters := BuildFilterMap(r, []string{"type", "status", "categoryId", "startDate", "endDate", "keyword"})

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "list_transactions")
		return
	}

	transactions, total, summary, err := h.transactionService.ListTransactions(r.Context(), user.ID, page, limit, filters)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to list transactions", "list_transactions")
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
		Summary: summary,
	})
}

// UpdateTransaction updates an existing transaction
// @Summary Update a transaction
// @Description Update a transaction. The previous wallet effect is reversed and the new one applied atomically (wallet, amount, type or status change).
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Transaction ID"
// @Param request body dto.UpdateTransactionRequest true "Update transaction request"
// @Success 200 {object} dto.TransactionResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /transactions/{id} [put]
func (h *TransactionHandler) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "update_transaction")
		return
	}

	var req dto.UpdateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "update_transaction")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "update_transaction")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "update_transaction")
		return
	}

	transaction, err := h.transactionService.UpdateTransaction(r.Context(), user.ID, id, req)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Transaction, category or wallet not found", "update_transaction")
			return
		}
		if errors.Is(err, constant.ErrInvalidInput) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid transaction", "update_transaction")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to update transaction", "update_transaction")
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
// @Description Soft-delete a transaction and reverse its effect on the wallet balance.
// @Tags transactions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Transaction ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /transactions/{id} [delete]
func (h *TransactionHandler) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "delete_transaction")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "delete_transaction")
		return
	}

	if err := h.transactionService.DeleteTransaction(r.Context(), user.ID, id); err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Transaction not found", "delete_transaction")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to delete transaction", "delete_transaction")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Transaction deleted successfully"})
}
