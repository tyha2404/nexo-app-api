package handler

import (
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type PresetHandler struct {
	svc          service.PresetService
	errorHandler *ErrorHandler
}

func NewPresetHandler(svc service.PresetService, log *zap.Logger) *PresetHandler {
	return &PresetHandler{
		svc:          svc,
		errorHandler: NewErrorHandler(log),
	}
}

// Create handles POST /presets
// @Summary Create a quick-add preset
// @Tags Presets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreatePresetRequest true "Preset"
// @Success 201 {object} PresetEnvelope
// @Failure 400 {object} response.ErrorResponse "Invalid request payload / Validation failed"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Category not found"
// @Failure 500 {object} response.ErrorResponse "Failed to create preset"
// @Router /presets [post]
func (h *PresetHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreatePresetRequest
	if !h.errorHandler.decodeAndValidate(w, r, &req, "create_preset") {
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

	h.errorHandler.HandleSuccess(w, http.StatusCreated, *res)
}

// Get handles GET /presets/{id}
// @Summary Get a preset
// @Tags Presets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Preset ID"
// @Success 200 {object} PresetEnvelope
// @Failure 400 {object} response.ErrorResponse "Invalid ID format"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Preset not found"
// @Failure 500 {object} response.ErrorResponse "Failed to get preset"
// @Router /presets/{id} [get]
func (h *PresetHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUIDFromPath(r, "id")
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

	h.errorHandler.HandleSuccess(w, http.StatusOK, *res)
}

// List handles GET /presets
// @Summary List presets
// @Description All quick-add presets of the caller (not paginated).
// @Tags Presets
// @Produce json
// @Security BearerAuth
// @Success 200 {object} PresetListEnvelope
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Failed to list presets"
// @Router /presets [get]
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

	h.errorHandler.HandleSuccess(w, http.StatusOK, items)
}

// Update handles PUT /presets/{id}
// @Summary Update a preset
// @Tags Presets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Preset ID"
// @Param request body dto.UpdatePresetRequest true "Fields to update"
// @Success 200 {object} PresetEnvelope
// @Failure 400 {object} response.ErrorResponse "Invalid ID format / Invalid request payload / Validation failed"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Preset or category not found"
// @Failure 500 {object} response.ErrorResponse "Failed to update preset"
// @Router /presets/{id} [put]
func (h *PresetHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "update_preset")
		return
	}

	var req dto.UpdatePresetRequest
	if !h.errorHandler.decodeAndValidate(w, r, &req, "update_preset") {
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

	h.errorHandler.HandleSuccess(w, http.StatusOK, *res)
}

// Delete handles DELETE /presets/{id}
// @Summary Delete a preset
// @Tags Presets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Preset ID"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid ID format"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Preset not found"
// @Failure 500 {object} response.ErrorResponse "Failed to delete preset"
// @Router /presets/{id} [delete]
func (h *PresetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUIDFromPath(r, "id")
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

	h.errorHandler.HandleSuccessWithMessage(w, http.StatusOK, nil, "Preset deleted successfully")
}
