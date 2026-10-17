package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type PresetHandler struct {
	svc          service.PresetService
	validator    *validator.Validate
	log          *zap.Logger
	errorHandler *ErrorHandler
}

func NewPresetHandler(svc service.PresetService, log *zap.Logger) *PresetHandler {
	return &PresetHandler{
		svc:          svc,
		validator:    validator.New(),
		log:          log,
		errorHandler: NewErrorHandler(log),
	}
}

func (h *PresetHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreatePresetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "create_preset")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "create_preset")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "create_preset")
		return
	}
	userID := user.ID
	res, err := h.svc.CreatePreset(r.Context(), userID, req)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Category not found", "create_preset")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to create preset", "create_preset")
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
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "get_preset")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_preset")
		return
	}
	userID := user.ID
	res, err := h.svc.GetPreset(r.Context(), userID, id)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Preset not found", "get_preset")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get preset", "get_preset")
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
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "list_presets")
		return
	}
	userID := user.ID
	items, err := h.svc.ListPresets(r.Context(), userID)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to list presets", "list_presets")
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
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "update_preset")
		return
	}

	var req dto.UpdatePresetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "update_preset")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "update_preset")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "update_preset")
		return
	}
	userID := user.ID
	res, err := h.svc.UpdatePreset(r.Context(), userID, id, req)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Preset or category not found", "update_preset")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to update preset", "update_preset")
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
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "delete_preset")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "delete_preset")
		return
	}
	userID := user.ID
	if err := h.svc.DeletePreset(r.Context(), userID, id); err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Preset not found", "delete_preset")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to delete preset", "delete_preset")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Preset deleted successfully"})
}
