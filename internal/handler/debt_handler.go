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

type DebtHandler struct {
	svc       service.DebtService
	validator *validator.Validate
	log       *zap.Logger
}

func NewDebtHandler(svc service.DebtService, log *zap.Logger) *DebtHandler {
	return &DebtHandler{
		svc:       svc,
		validator: validator.New(),
		log:       log,
	}
}

func (h *DebtHandler) CreateDebt(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req dto.CreateDebtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.CreateDebt(r.Context(), user.ID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	debtType := model.DebtType(r.URL.Query().Get("type"))
	status := model.DebtStatus(r.URL.Query().Get("status"))

	debts, err := h.svc.GetDebts(r.Context(), user.ID, debtType, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	summary, err := h.svc.GetDebtSummary(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.DebtSummaryResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    summary,
	})
}

func (h *DebtHandler) AddRepayment(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	debtIDStr := chi.URLParam(r, "id")
	debtID, err := uuid.Parse(debtIDStr)
	if err != nil {
		http.Error(w, "Invalid debt ID", http.StatusBadRequest)
		return
	}

	var req dto.AddRepaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.AddRepayment(r.Context(), user.ID, debtID, req)
	if err != nil {
		if err == service.ErrDebtNotFound {
			http.Error(w, "Debt not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

func (h *DebtHandler) DeleteDebt(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	debtIDStr := chi.URLParam(r, "id")
	debtID, err := uuid.Parse(debtIDStr)
	if err != nil {
		http.Error(w, "Invalid debt ID", http.StatusBadRequest)
		return
	}

	if err := h.svc.DeleteDebt(r.Context(), user.ID, debtID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[map[string]string]{
		Status:  http.StatusOK,
		Success: true,
		Message: "Debt deleted successfully",
	})
}
