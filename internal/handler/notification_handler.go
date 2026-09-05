package handler

import (
	"encoding/json"
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type NotificationHandler struct {
	svc service.NotificationService
	log *zap.Logger
}

func NewNotificationHandler(svc service.NotificationService, log *zap.Logger) *NotificationHandler {
	return &NotificationHandler{
		svc: svc,
		log: log,
	}
}

// GetVapidPublicKey godoc
// @Summary Get VAPID Public Key for Web Push
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.VapidPublicKeyResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /notifications/vapid-public-key [get]
func (h *NotificationHandler) GetVapidPublicKey(w http.ResponseWriter, r *http.Request) {
	key := h.svc.GetVapidPublicKey()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.VapidPublicKeyResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    dto.VapidPublicKeyResponse{PublicKey: key},
	})
}

// Subscribe godoc
// @Summary Subscribe a device to Web Push
// @Tags Notifications
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.SubscribePushRequest true "Subscription payload"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /notifications/subscribe [post]
func (h *NotificationHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userVal := r.Context().Value(constant.UserContextKey)
	if userVal == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	user := userVal.(model.User)

	var req dto.SubscribePushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Dữ liệu đăng ký không hợp lệ", http.StatusBadRequest)
		return
	}

	if req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		http.Error(w, "Thiếu thông tin endpoint hoặc keys của PushSubscription", http.StatusBadRequest)
		return
	}

	if err := h.svc.Subscribe(r.Context(), user.ID, &req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[interface{}]{
		Status:  http.StatusOK,
		Success: true,
		Message: "Đăng ký nhận thông báo thành công",
	})
}

// Unsubscribe godoc
// @Summary Unsubscribe a device from Web Push
// @Tags Notifications
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UnsubscribePushRequest true "Unsubscribe payload"
// @Success 200 {object} map[string]string
// @Router /notifications/unsubscribe [post]
func (h *NotificationHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	var req dto.UnsubscribePushRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.svc.Unsubscribe(r.Context(), &req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[interface{}]{
		Status:  http.StatusOK,
		Success: true,
		Message: "Đã hủy đăng ký nhận thông báo",
	})
}

// SendTestNotification godoc
// @Summary Send a test push notification to the current user's devices
// @Tags Notifications
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.TestPushRequest false "Custom test payload"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /notifications/test [post]
func (h *NotificationHandler) SendTestNotification(w http.ResponseWriter, r *http.Request) {
	userVal := r.Context().Value(constant.UserContextKey)
	if userVal == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	user := userVal.(model.User)

	var req dto.TestPushRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.svc.SendTestPush(r.Context(), user.ID, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[interface{}]{
		Status:  http.StatusOK,
		Success: true,
		Message: "Đã gửi thông báo thử nghiệm thành công! Vui lòng kiểm tra màn hình khóa hoặc Notification Center.",
	})
}
