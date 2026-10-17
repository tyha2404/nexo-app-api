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

type BudgetHandler struct {
	svc          service.BudgetService
	validator    *validator.Validate
	log          *zap.Logger
	errorHandler *ErrorHandler
}

func NewBudgetHandler(svc service.BudgetService, log *zap.Logger) *BudgetHandler {
	return &BudgetHandler{
		svc:          svc,
		validator:    validator.New(),
		log:          log,
		errorHandler: NewErrorHandler(log),
	}
}

func (h *BudgetHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "create_budget")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "create_budget")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "create_budget")
		return
	}
	userID := user.ID
	res, err := h.svc.CreateBudget(r.Context(), userID, req)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Category not found", "create_budget")
			return
		}
		if errors.Is(err, constant.ErrAlreadyExists) {
			h.errorHandler.HandleError(w, err, http.StatusConflict, "Ngân sách cho danh mục và kỳ này đã tồn tại", "create_budget")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to create budget", "create_budget")
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
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "get_budget")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_budget")
		return
	}
	userID := user.ID
	res, err := h.svc.GetBudget(r.Context(), userID, id)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Budget not found", "get_budget")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get budget", "get_budget")
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
	page, limit := ParsePagination(r)

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "list_budgets")
		return
	}
	userID := user.ID
	items, total, err := h.svc.ListBudgets(r.Context(), userID, page, limit)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to list budgets", "list_budgets")
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
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "update_budget")
		return
	}

	var req dto.UpdateBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "update_budget")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "update_budget")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "update_budget")
		return
	}
	userID := user.ID
	res, err := h.svc.UpdateBudget(r.Context(), userID, id, req)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Budget not found", "update_budget")
			return
		}
		if errors.Is(err, constant.ErrAlreadyExists) {
			h.errorHandler.HandleError(w, err, http.StatusConflict, "Ngân sách cho danh mục và kỳ này đã tồn tại", "update_budget")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to update budget", "update_budget")
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
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "delete_budget")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "delete_budget")
		return
	}
	userID := user.ID
	if err := h.svc.DeleteBudget(r.Context(), userID, id); err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Budget not found", "delete_budget")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to delete budget", "delete_budget")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Budget deleted successfully"})
}
