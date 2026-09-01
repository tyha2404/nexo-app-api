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

type CreditCardStatementHandler struct {
	svc       service.CreditCardStatementService
	validator *validator.Validate
	log       *zap.Logger
}

func NewCreditCardStatementHandler(svc service.CreditCardStatementService, log *zap.Logger) *CreditCardStatementHandler {
	return &CreditCardStatementHandler{
		svc:       svc,
		validator: validator.New(),
		log:       log,
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
// @Success 200 {object} dto.StatementListResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /credit-card-statements [get]
func (h *CreditCardStatementHandler) ListStatements(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var walletIDPtr *uuid.UUID
	if walletIDStr := r.URL.Query().Get("walletId"); walletIDStr != "" {
		if u, err := uuid.Parse(walletIDStr); err == nil {
			walletIDPtr = &u
		}
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.StatementListResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    res,
	})
}

// CreateStatement godoc
// @Summary Create a credit card statement
// @Description Create a monthly credit card statement record
// @Tags CreditCardStatements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateStatementRequest true "Statement payload"
// @Success 201 {object} dto.StatementResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /credit-card-statements [post]
func (h *CreditCardStatementHandler) CreateStatement(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req dto.CreateStatementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.CreateStatement(r.Context(), user.ID, req)
	if err != nil {
		if err == service.ErrWalletNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.StatementResponse]{
		Status:  http.StatusCreated,
		Success: true,
		Data:    res,
	})
}

// GetStatementByID godoc
// @Summary Get statement by ID
// @Description Retrieve a credit card statement by its ID
// @Tags CreditCardStatements
// @Produce json
// @Security BearerAuth
// @Param id path string true "Statement ID"
// @Success 200 {object} dto.StatementResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /credit-card-statements/{id} [get]
func (h *CreditCardStatementHandler) GetStatementByID(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid statement ID", http.StatusBadRequest)
		return
	}

	res, err := h.svc.GetStatementByID(r.Context(), user.ID, id)
	if err != nil {
		if err == service.ErrStatementNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.StatementResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    res,
	})
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
// @Success 200 {object} dto.StatementResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /credit-card-statements/{id} [put]
func (h *CreditCardStatementHandler) UpdateStatement(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid statement ID", http.StatusBadRequest)
		return
	}

	var req dto.UpdateStatementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.UpdateStatement(r.Context(), user.ID, id, req)
	if err != nil {
		if err == service.ErrStatementNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.StatementResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    res,
	})
}

// PayStatement godoc
// @Summary Pay a credit card statement
// @Description Record a payment against a monthly credit card statement
// @Tags CreditCardStatements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Statement ID"
// @Param request body dto.PayStatementRequest true "Payment payload"
// @Success 200 {object} dto.StatementResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /credit-card-statements/{id}/pay [post]
func (h *CreditCardStatementHandler) PayStatement(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid statement ID", http.StatusBadRequest)
		return
	}

	var req dto.PayStatementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.PayStatement(r.Context(), user.ID, id, req)
	if err != nil {
		if err == service.ErrStatementNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[*dto.StatementResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    res,
	})
}

// DeleteStatement godoc
// @Summary Delete statement
// @Description Delete a credit card statement
// @Tags CreditCardStatements
// @Produce json
// @Security BearerAuth
// @Param id path string true "Statement ID"
// @Success 200 {string} string "OK"
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /credit-card-statements/{id} [delete]
func (h *CreditCardStatementHandler) DeleteStatement(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid statement ID", http.StatusBadRequest)
		return
	}

	if err := h.svc.DeleteStatement(r.Context(), user.ID, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[map[string]string]{
		Status:  http.StatusOK,
		Success: true,
		Message: "Xóa kỳ sao kê thành công",
	})
}
