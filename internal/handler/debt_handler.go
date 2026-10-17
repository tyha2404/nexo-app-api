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

type DebtHandler struct {
	svc          service.DebtService
	validator    *validator.Validate
	log          *zap.Logger
	errorHandler *ErrorHandler
}

func NewDebtHandler(svc service.DebtService, log *zap.Logger) *DebtHandler {
	return &DebtHandler{
		svc:          svc,
		validator:    validator.New(),
		log:          log,
		errorHandler: NewErrorHandler(log),
	}
}

func (h *DebtHandler) CreateDebt(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "create_debt")
		return
	}

	var req dto.CreateDebtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "create_debt")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "create_debt")
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.DebtResponse]{
		Status:  http.StatusCreated,
		Success: true,
		Message: "Debt created successfully",
		Data:    res,
	})
}

func (h *DebtHandler) GetDebts(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "get_debts")
		return
	}

	debtType := model.DebtType(r.URL.Query().Get("type"))
	status := model.DebtStatus(r.URL.Query().Get("status"))

	debts, err := h.svc.GetDebts(r.Context(), user.ID, debtType, status)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get debts", "get_debts")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[[]dto.DebtResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    debts,
	})
}

func (h *DebtHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "get_debt_summary")
		return
	}

	summary, err := h.svc.GetDebtSummary(r.Context(), user.ID)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get debt summary", "get_debt_summary")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.DebtSummaryResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    summary,
	})
}

// AddRepayment records a payment without exceeding the outstanding balance.
// @Summary Record a debt repayment
// @Tags Debts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Debt ID"
// @Param request body dto.AddRepaymentRequest true "Repayment"
// @Success 200 {object} RepaymentResponse
// @Failure 400 {object} response.ErrorResponse "Invalid amount, debt already completed, or repayment exceeds remaining balance"
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Debt or wallet not found"
// @Failure 500 {object} response.ErrorResponse
// @Router /debts/{id}/repayments [post]
func (h *DebtHandler) AddRepayment(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "add_repayment")
		return
	}

	debtIDStr := chi.URLParam(r, "id")
	debtID, err := uuid.Parse(debtIDStr)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid debt ID", "add_repayment")
		return
	}

	var req dto.AddRepaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "add_repayment")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "add_repayment")
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.DebtResponse]{
		Status:  http.StatusOK,
		Success: true,
		Message: "Repayment recorded successfully",
		Data:    res,
	})
}

// DeleteDebt soft-deletes a debt and its repayments and reverses their wallet effects.
// @Summary Delete a debt
// @Description Soft-deletes the debt and its repayments and, in one DB transaction, reverses the wallet balance effects of the debt creation and of every repayment.
// @Tags Debts
// @Produce json
// @Security BearerAuth
// @Param id path string true "Debt ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Debt not found"
// @Failure 500 {object} response.ErrorResponse
// @Router /debts/{id} [delete]
func (h *DebtHandler) DeleteDebt(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "delete_debt")
		return
	}

	debtIDStr := chi.URLParam(r, "id")
	debtID, err := uuid.Parse(debtIDStr)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[map[string]string]{
		Status:  http.StatusOK,
		Success: true,
		Message: "Debt deleted successfully",
	})
}

// RepaymentResponse describes the repayment envelope for Swagger.
type RepaymentResponse struct {
	Status  int               `json:"status"`
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Data    *dto.DebtResponse `json:"data"`
}
