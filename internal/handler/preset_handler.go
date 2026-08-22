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

type PresetHandler struct {
	svc       service.PresetService
	validator *validator.Validate
	log       *zap.Logger
}

func NewPresetHandler(svc service.PresetService, log *zap.Logger) *PresetHandler {
	return &PresetHandler{
		svc:       svc,
		validator: validator.New(),
		log:       log,
	}
}

func (h *PresetHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreatePresetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	res, err := h.svc.CreatePreset(r.Context(), userID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response.BaseResponse[dto.PresetResponse]{
		Status:  http.StatusCreated,
		Success: true,
		Data:    *res,
	})
}

func (h *PresetHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	res, err := h.svc.GetPreset(r.Context(), userID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.PresetResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *res,
	})
}

func (h *PresetHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	items, err := h.svc.ListPresets(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[[]dto.PresetResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    items,
	})
}

func (h *PresetHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	var req dto.UpdatePresetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	res, err := h.svc.UpdatePreset(r.Context(), userID, id, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.PresetResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *res,
	})
}

func (h *PresetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(constant.UserContextKey).(model.User).ID
	if err := h.svc.DeletePreset(r.Context(), userID, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Preset deleted successfully"})
}
