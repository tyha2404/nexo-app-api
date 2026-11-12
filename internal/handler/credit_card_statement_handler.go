package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type CreditCardStatementHandler struct {
	svc          service.CreditCardStatementService
	errorHandler *ErrorHandler
}

func NewCreditCardStatementHandler(svc service.CreditCardStatementService, log *zap.Logger) *CreditCardStatementHandler {
	return &CreditCardStatementHandler{
		svc:          svc,
		errorHandler: NewErrorHandler(log),
	}
}

// ListStatements godoc
// @Summary List credit card statements
// @Description Retrieve credit card statements with optional filtering by wallet, year and month
// @Tags CreditCardStatements
// @Produce json
// @Security BearerAuth
// @Param walletId query string false "Wallet ID"
// @Param year query int false "Year"
// @Param month query int false "Month"
// @Success 200 {object} StatementListEnvelope
// @Failure 400 {object} response.ErrorResponse "Invalid wallet ID"
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /credit-card-statements [get]
func (h *CreditCardStatementHandler) ListStatements(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "list_statements")
		return
	}

	var walletIDPtr *uuid.UUID
	if walletIDStr := r.URL.Query().Get("walletId"); walletIDStr != "" {
		u, err := uuid.Parse(walletIDStr)
		if err != nil {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid wallet ID", "list_statements")
			return
		}
		walletIDPtr = &u
	}

	var yearPtr *int
	if yStr := r.URL.Query().Get("year"); yStr != "" {
		if y, err := strconv.Atoi(yStr); err == nil && y > 0 {
			yearPtr = &y
		}
	}

	var monthPtr *int
	if mStr := r.URL.Query().Get("month"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil && m > 0 {
			monthPtr = &m
		}
	}

	res, err := h.svc.ListStatements(r.Context(), user.ID, walletIDPtr, yearPtr, monthPtr)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to list statements", "list_statements")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, res)
}

// CreateStatement godoc
// @Summary Create a credit card statement
// @Description Create a monthly credit card statement record
// @Tags CreditCardStatements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateStatementRequest true "Statement payload"
// @Success 201 {object} StatementEnvelope
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Wallet not found"
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /credit-card-statements [post]
func (h *CreditCardStatementHandler) CreateStatement(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "create_statement")
		return
	}

	var req dto.CreateStatementRequest
	if !h.errorHandler.decodeAndValidate(w, r, &req, "create_statement") {
		return
	}

	res, err := h.svc.CreateStatement(r.Context(), user.ID, req)
	if err != nil {
		if errors.Is(err, service.ErrWalletNotFound) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Wallet not found", "create_statement")
			return
		}
		if errors.Is(err, service.ErrInvalidDate) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid statement date", "create_statement")
			return
		}
		if errors.Is(err, constant.ErrAlreadyExists) {
			h.errorHandler.HandleError(w, err, http.StatusConflict, "Kỳ sao kê này đã tồn tại", "create_statement")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to create statement", "create_statement")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusCreated, res)
}

// GetStatementByID godoc
// @Summary Get statement by ID
// @Description Retrieve a credit card statement by its ID
// @Tags CreditCardStatements
// @Produce json
// @Security BearerAuth
// @Param id path string true "Statement ID"
// @Success 200 {object} StatementEnvelope
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /credit-card-statements/{id} [get]
func (h *CreditCardStatementHandler) GetStatementByID(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_statement_by_id")
		return
	}

	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid statement ID", "get_statement_by_id")
		return
	}

	res, err := h.svc.GetStatementByID(r.Context(), user.ID, id)
	if err != nil {
		if errors.Is(err, service.ErrStatementNotFound) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Statement not found", "get_statement_by_id")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get statement", "get_statement_by_id")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, res)
}

// UpdateStatement godoc
// @Summary Update credit card statement
// @Description Update fields of a credit card statement
// @Tags CreditCardStatements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Statement ID"
// @Param request body dto.UpdateStatementRequest true "Update payload"
// @Success 200 {object} StatementEnvelope
// @Failure 400 {object} response.ErrorResponse "Invalid ID, payload or date (YYYY-MM-DD)"
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse "A statement for this period already exists"
// @Failure 500 {object} response.ErrorResponse
// @Router /credit-card-statements/{id} [put]
func (h *CreditCardStatementHandler) UpdateStatement(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "update_statement")
		return
	}

	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid statement ID", "update_statement")
		return
	}

	var req dto.UpdateStatementRequest
	if !h.errorHandler.decodeAndValidate(w, r, &req, "update_statement") {
		return
	}

	res, err := h.svc.UpdateStatement(r.Context(), user.ID, id, req)
	if err != nil {
		if errors.Is(err, service.ErrStatementNotFound) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Statement not found", "update_statement")
			return
		}
		if errors.Is(err, service.ErrInvalidDate) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid statement date", "update_statement")
			return
		}
		if errors.Is(err, constant.ErrAlreadyExists) {
			h.errorHandler.HandleError(w, err, http.StatusConflict, "Kỳ sao kê này đã tồn tại", "update_statement")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to update statement", "update_statement")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, res)
}

// PayStatement godoc
// @Summary Pay a credit card statement
// @Description Record a payment against a monthly credit card statement. Atomic: debits the source wallet (if given, must belong to the user), credits the card wallet and updates the statement in one DB transaction. Rejects payments above the remaining balance.
// @Tags CreditCardStatements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Statement ID"
// @Param request body dto.PayStatementRequest true "Payment payload"
// @Success 200 {object} StatementEnvelope
// @Failure 400 {object} response.ErrorResponse "Invalid amount or overpayment"
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Statement or wallet not found"
// @Failure 409 {object} response.ErrorResponse "Statement already paid"
// @Failure 500 {object} response.ErrorResponse
// @Router /credit-card-statements/{id}/pay [post]
func (h *CreditCardStatementHandler) PayStatement(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "pay_statement")
		return
	}

	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid statement ID", "pay_statement")
		return
	}

	var req dto.PayStatementRequest
	if !h.errorHandler.decodeAndValidate(w, r, &req, "pay_statement") {
		return
	}

	res, err := h.svc.PayStatement(r.Context(), user.ID, id, req)
	if err != nil {
		if errors.Is(err, service.ErrStatementNotFound) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Statement not found", "pay_statement")
			return
		}
		if errors.Is(err, service.ErrStatementAlreadyPaid) {
			h.errorHandler.HandleError(w, err, http.StatusConflict, "Kỳ sao kê đã được thanh toán đủ", "pay_statement")
			return
		}
		if errors.Is(err, service.ErrStatementInvalidPayment) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Số tiền thanh toán không hợp lệ", "pay_statement")
			return
		}
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Wallet not found", "pay_statement")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to pay statement", "pay_statement")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, res)
}

// DeleteStatement godoc
// @Summary Delete statement
// @Description Delete a credit card statement
// @Tags CreditCardStatements
// @Produce json
// @Security BearerAuth
// @Param id path string true "Statement ID"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Statement not found"
// @Failure 500 {object} response.ErrorResponse
// @Router /credit-card-statements/{id} [delete]
func (h *CreditCardStatementHandler) DeleteStatement(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "delete_statement")
		return
	}

	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid statement ID", "delete_statement")
		return
	}

	if err := h.svc.DeleteStatement(r.Context(), user.ID, id); err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Statement not found", "delete_statement")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to delete statement", "delete_statement")
		return
	}

	h.errorHandler.HandleSuccessWithMessage(w, http.StatusOK, nil, "Xóa kỳ sao kê thành công")
}
