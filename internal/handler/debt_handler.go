package handler

import (
	"errors"
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type DebtHandler struct {
	svc          service.DebtService
	errorHandler *ErrorHandler
}

func NewDebtHandler(svc service.DebtService, log *zap.Logger) *DebtHandler {
	return &DebtHandler{
		svc:          svc,
		errorHandler: NewErrorHandler(log),
	}
}

// CreateDebt handles POST /debts
// @Summary Create a debt
// @Description Creates a PAYABLE or RECEIVABLE debt; with a walletId the amount moves that wallet's balance in the same DB transaction.
// @Tags Debts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateDebtRequest true "Debt"
// @Success 201 {object} DebtEnvelopeResponse
// @Failure 400 {object} response.ErrorResponse "Invalid request payload / Validation failed"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Wallet not found"
// @Failure 500 {object} response.ErrorResponse "Failed to create debt"
// @Router /debts [post]
func (h *DebtHandler) CreateDebt(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "create_debt")
		return
	}

	var req dto.CreateDebtRequest
	if !h.errorHandler.decodeAndValidate(w, r, &req, "create_debt") {
		return
	}

	res, err := h.svc.CreateDebt(r.Context(), user.ID, req)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Wallet not found", "create_debt")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to create debt", "create_debt")
		return
	}

	h.errorHandler.HandleSuccessWithMessage(w, http.StatusCreated, res, "Debt created successfully")
}

// GetDebts handles GET /debts
// @Summary List debts
// @Tags Debts
// @Produce json
// @Security BearerAuth
// @Param type query string false "PAYABLE or RECEIVABLE"
// @Param status query string false "PENDING, COMPLETED or OVERDUE"
// @Success 200 {object} DebtListEnvelope
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Failed to get debts"
// @Router /debts [get]
func (h *DebtHandler) GetDebts(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_debts")
		return
	}

	debtType := model.DebtType(r.URL.Query().Get("type"))
	status := model.DebtStatus(r.URL.Query().Get("status"))

	debts, err := h.svc.GetDebts(r.Context(), user.ID, debtType, status)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get debts", "get_debts")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, debts)
}

// GetSummary handles GET /debts/summary
// @Summary Get the debt summary
// @Description Totals still payable/receivable and the number of pending and overdue debts.
// @Tags Debts
// @Produce json
// @Security BearerAuth
// @Success 200 {object} DebtSummaryEnvelope
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Failed to get debt summary"
// @Router /debts/summary [get]
func (h *DebtHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_debt_summary")
		return
	}

	summary, err := h.svc.GetDebtSummary(r.Context(), user.ID)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get debt summary", "get_debt_summary")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, summary)
}

// AddRepayment records a payment without exceeding the outstanding balance.
// @Summary Record a debt repayment
// @Tags Debts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Debt ID"
// @Param request body dto.AddRepaymentRequest true "Repayment"
// @Success 200 {object} DebtEnvelopeResponse
// @Failure 400 {object} response.ErrorResponse "Invalid amount, debt already completed, or repayment exceeds remaining balance"
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Debt or wallet not found"
// @Failure 500 {object} response.ErrorResponse
// @Router /debts/{id}/repayments [post]
func (h *DebtHandler) AddRepayment(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "add_repayment")
		return
	}

	debtID, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid debt ID", "add_repayment")
		return
	}

	var req dto.AddRepaymentRequest
	if !h.errorHandler.decodeAndValidate(w, r, &req, "add_repayment") {
		return
	}

	res, err := h.svc.AddRepayment(r.Context(), user.ID, debtID, req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidRepayment) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid repayment", "add_repayment")
			return
		}
		if errors.Is(err, service.ErrDebtNotFound) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Debt not found", "add_repayment")
			return
		}
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Wallet not found", "add_repayment")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to add repayment", "add_repayment")
		return
	}

	h.errorHandler.HandleSuccessWithMessage(w, http.StatusOK, res, "Repayment recorded successfully")
}

// UpdateDebt edits a debt's title, amount, dates, notes and wallet.
// @Summary Update a debt
// @Description Full replacement of the editable fields (the type cannot change; a null dueDate/walletId clears it). totalAmount may not drop below the amount already repaid. The wallet effect of the debt's creation is moved/resized in the same DB transaction; repayments are untouched.
// @Tags Debts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Debt ID"
// @Param request body dto.UpdateDebtRequest true "Debt"
// @Success 200 {object} DebtEnvelopeResponse
// @Failure 400 {object} response.ErrorResponse "Invalid input or amount below what was repaid"
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Debt or wallet not found"
// @Failure 500 {object} response.ErrorResponse
// @Router /debts/{id} [put]
func (h *DebtHandler) UpdateDebt(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "update_debt")
		return
	}

	debtID, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid debt ID", "update_debt")
		return
	}

	var req dto.UpdateDebtRequest
	if !h.errorHandler.decodeAndValidate(w, r, &req, "update_debt") {
		return
	}

	res, err := h.svc.UpdateDebt(r.Context(), user.ID, debtID, req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidDebtUpdate) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, err.Error(), "update_debt")
			return
		}
		if errors.Is(err, service.ErrDebtNotFound) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Debt not found", "update_debt")
			return
		}
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Wallet not found", "update_debt")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to update debt", "update_debt")
		return
	}

	h.errorHandler.HandleSuccessWithMessage(w, http.StatusOK, res, "Debt updated successfully")
}

// DeleteDebt soft-deletes a debt and its repayments and reverses their wallet effects.
// @Summary Delete a debt
// @Description Soft-deletes the debt and its repayments and, in one DB transaction, reverses the wallet balance effects of the debt creation and of every repayment.
// @Tags Debts
// @Produce json
// @Security BearerAuth
// @Param id path string true "Debt ID"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Debt not found"
// @Failure 500 {object} response.ErrorResponse
// @Router /debts/{id} [delete]
func (h *DebtHandler) DeleteDebt(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "delete_debt")
		return
	}

	debtID, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid debt ID", "delete_debt")
		return
	}

	if err := h.svc.DeleteDebt(r.Context(), user.ID, debtID); err != nil {
		if errors.Is(err, service.ErrDebtNotFound) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Debt not found", "delete_debt")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to delete debt", "delete_debt")
		return
	}

	h.errorHandler.HandleSuccessWithMessage(w, http.StatusOK, nil, "Debt deleted successfully")
}

// DebtEnvelopeResponse describes the {status, success, message, data: debt}
// envelope (create, repayment, update) for Swagger.
type DebtEnvelopeResponse struct {
	Status  int               `json:"status"`
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Data    *dto.DebtResponse `json:"data"`
}
