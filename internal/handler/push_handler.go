package handler

import (
	"errors"
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

// maxPushBodyBytes caps subscribe/unsubscribe bodies.
const maxPushBodyBytes = 8 << 10

type PushHandler struct {
	pushService  service.PushService
	log          *zap.Logger
	errorHandler *ErrorHandler
	validator    *Validator
}

func NewPushHandler(pushService service.PushService, log *zap.Logger) *PushHandler {
	return &PushHandler{
		pushService:  pushService,
		log:          log,
		errorHandler: NewErrorHandler(log),
		validator:    NewValidator(),
	}
}

// GetVAPIDPublicKey returns the application server key used by pushManager.subscribe
// @Summary Get VAPID public key
// @Description Public endpoint. Returns 503 when Web Push is not configured on the server (VAPID keys missing).
// @Tags push
// @Produce json
// @Success 200 {object} dto.VAPIDPublicKeyResponse
// @Failure 503 {object} response.ErrorResponse "Push notifications disabled"
// @Router /push/vapid-public-key [get]
func (h *PushHandler) GetVAPIDPublicKey(w http.ResponseWriter, r *http.Request) {
	key, err := h.pushService.PublicKey()
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusServiceUnavailable, "Thông báo đẩy chưa được cấu hình trên máy chủ", "get_vapid_public_key")
		return
	}
	h.errorHandler.HandleSuccess(w, http.StatusOK, dto.VAPIDPublicKeyResponse{PublicKey: key})
}

// Subscribe stores (upserts by endpoint) the caller's push subscription
// @Summary Subscribe to push notifications
// @Description Upserts the browser PushSubscription by endpoint for the current user.
// @Tags push
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.SubscribePushRequest true "PushSubscription.toJSON()"
// @Success 201 {object} dto.PushSubscriptionResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Failure 503 {object} response.ErrorResponse "Push notifications disabled"
// @Router /push/subscriptions [post]
func (h *PushHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "push_subscribe")
		return
	}
	if !h.pushService.Enabled() {
		h.errorHandler.HandleError(w, service.ErrPushDisabled, http.StatusServiceUnavailable, "Thông báo đẩy chưa được cấu hình trên máy chủ", "push_subscribe")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPushBodyBytes)
	var req dto.SubscribePushRequest
	if err := DecodeJSONBody(r, &req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "push_subscribe")
		return
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "push_subscribe")
		return
	}

	if err := h.pushService.Subscribe(r.Context(), user.ID, req, r.UserAgent()); err != nil {
		if errors.Is(err, service.ErrPushDisabled) {
			h.errorHandler.HandleError(w, err, http.StatusServiceUnavailable, "Thông báo đẩy chưa được cấu hình trên máy chủ", "push_subscribe")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Không thể lưu đăng ký thông báo", "push_subscribe")
		return
	}
	h.errorHandler.HandleSuccess(w, http.StatusCreated, dto.PushSubscriptionResponse{Endpoint: req.Endpoint, Enabled: true})
}

// Unsubscribe removes the caller's push subscription
// @Summary Unsubscribe from push notifications
// @Description Deletes the current user's subscription with that endpoint. Succeeds even if it does not exist.
// @Tags push
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UnsubscribePushRequest true "Endpoint to remove"
// @Success 200 {object} dto.PushSubscriptionResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /push/subscriptions [delete]
func (h *PushHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "push_unsubscribe")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPushBodyBytes)
	var req dto.UnsubscribePushRequest
	if err := DecodeJSONBody(r, &req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "push_unsubscribe")
		return
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "push_unsubscribe")
		return
	}

	if err := h.pushService.Unsubscribe(r.Context(), user.ID, req.Endpoint); err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Không thể huỷ đăng ký thông báo", "push_unsubscribe")
		return
	}
	h.errorHandler.HandleSuccess(w, http.StatusOK, dto.PushSubscriptionResponse{Endpoint: req.Endpoint, Enabled: false})
}
