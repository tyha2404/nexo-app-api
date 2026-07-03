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

type BudgetHandler struct {
	svc       service.BudgetService
	validator *validator.Validate
	log       *zap.Logger
}

func NewBudgetHandler(svc service.BudgetService, log *zap.Logger) *BudgetHandler {
	return &BudgetHandler{
		svc:       svc,
		validator: validator.New(),
		log:       log,
	}
}

func (h *BudgetHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	res, err := h.svc.CreateBudget(r.Context(), userID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response.BaseResponse[dto.BudgetResponse]{
		Status:  http.StatusCreated,
		Success: true,
		Data:    *res,
	})
}

func (h *BudgetHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	res, err := h.svc.GetBudget(r.Context(), userID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.BudgetResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *res,
	})
}

func (h *BudgetHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 10
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	items, total, err := h.svc.ListBudgets(r.Context(), userID, page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.PaginationResponse[dto.BudgetResponse]{
		Status:  http.StatusOK,
		Success: true,
		Items:   items,
		Total:   int(total),
		Page:    page,
		Limit:   limit,
	})
}

func (h *BudgetHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	var req dto.UpdateBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	res, err := h.svc.UpdateBudget(r.Context(), userID, id, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.BudgetResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *res,
	})
}

func (h *BudgetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	if err := h.svc.DeleteBudget(r.Context(), userID, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Budget deleted successfully"})
}
