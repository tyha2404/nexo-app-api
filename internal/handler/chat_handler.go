package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type ChatHandler struct {
	chatService  service.ChatService
	log          *zap.Logger
	errorHandler *ErrorHandler
}

func NewChatHandler(chatService service.ChatService, log *zap.Logger) *ChatHandler {
	return &ChatHandler{
		chatService:  chatService,
		log:          log,
		errorHandler: NewErrorHandler(log),
	}
}

// StreamMessage handles sending a message and streaming AI response via Server-Sent Events (SSE)
// @Summary Send message and stream AI response
// @Description Stream financial advisory and tool responses using Requesty AI (OpenAI Compatible)
// @Tags chat
// @Accept json
// @Produce text/event-stream
// @Security BearerAuth
// @Param request body dto.SendMessageRequest true "Send Message Request"
// @Success 200 {string} string "SSE stream of dto.ChatStreamEvent"
// @Description Messages longer than 4000 characters are rejected. `model` must be one of GET /chat/models (otherwise the server default is used). Rate limited to 20 messages per minute per user.
// @Failure 400 {object} response.ErrorResponse "Bad Request (empty or too long message)"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 429 {object} response.ErrorResponse "Too many messages"
// @Failure 500 {object} response.ErrorResponse "Streaming unsupported"
// @Router /chat/stream [post]
func (h *ChatHandler) StreamMessage(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "stream_message")
		return
	}

	var req dto.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "stream_message")
		return
	}

	if strings.TrimSpace(req.Message) == "" {
		h.errorHandler.HandleError(w, constant.ErrInvalidInput, http.StatusBadRequest, "Message cannot be empty", "stream_message")
		return
	}
	if utf8.RuneCountInString(req.Message) > dto.MaxChatMessageLength {
		h.errorHandler.HandleError(w, constant.ErrInvalidInput, http.StatusBadRequest,
			fmt.Sprintf("Message is too long (max %d characters)", dto.MaxChatMessageLength), "stream_message")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.errorHandler.HandleError(w, errors.New("response writer does not support flushing"), http.StatusInternalServerError, "Streaming unsupported", "stream_message")
		return
	}

	// The server's WriteTimeout (cmd/server/main.go) would cut long AI
	// answers; lift the write deadline for this streaming response only.
	// Streams are still bounded by the upstream stream timeout in
	// NineRouterService and end when the client disconnects.
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.log.Warn("failed to clear write deadline for chat stream", zap.Error(err))
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	eventChan := make(chan dto.ChatStreamEvent, 20)
	go func() {
		if err := h.chatService.ProcessMessageStream(r.Context(), user.ID, req, eventChan); err != nil {
			h.log.Error("error processing chat message stream", zap.Error(err))
		}
	}()

	for event := range eventChan {
		eventData, err := json.Marshal(event)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", eventData)
		flusher.Flush()
	}
}

// ListSessions returns user's active chat sessions
// @Summary List chat sessions
// @Description Get user's chat sessions
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Success 200 {array} dto.ChatSessionResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /chat/sessions [get]
func (h *ChatHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "list_sessions")
		return
	}

	sessions, err := h.chatService.ListSessions(r.Context(), user.ID, 30)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to list chat sessions", "list_sessions")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response.BaseResponse[[]dto.ChatSessionResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    sessions,
	})
}

// GetSessionMessages returns messages in a chat session
// @Summary Get session messages
// @Description Get the newest 50 messages of a session, in chronological order (oldest first)
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 200 {object} dto.ChatSessionResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /chat/sessions/{id} [get]
func (h *ChatHandler) GetSessionMessages(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "get_session_messages")
		return
	}

	sessionID, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid session ID", "get_session_messages")
		return
	}

	sess, err := h.chatService.GetSessionMessages(r.Context(), user.ID, sessionID, 50)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Session not found", "get_session_messages")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get session messages", "get_session_messages")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response.BaseResponse[dto.ChatSessionResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *sess,
	})
}

// DeleteSession deletes a single chat session
// @Summary Delete chat session
// @Description Delete a chat session and all its messages
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /chat/sessions/{id} [delete]
func (h *ChatHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "delete_session")
		return
	}

	sessionID, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid session ID", "delete_session")
		return
	}

	if err := h.chatService.DeleteSession(r.Context(), user.ID, sessionID); err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Session not found", "delete_session")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to delete session", "delete_session")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response.BaseResponse[string]{
		Status:  http.StatusOK,
		Success: true,
		Data:    "Phiên trò chuyện đã được xóa thành công",
	})
}

// ClearSessions deletes all chat sessions for user
// @Summary Clear all chat history
// @Description Delete all chat sessions and messages for current user
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /chat/clear [post]
func (h *ChatHandler) ClearSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "clear_sessions")
		return
	}

	if err := h.chatService.ClearSessions(r.Context(), user.ID); err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to clear chat sessions", "clear_sessions")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response.BaseResponse[string]{
		Status:  http.StatusOK,
		Success: true,
		Data:    "Đã xóa toàn bộ lịch sử trò chuyện",
	})
}

// ListModels returns available AI models dynamically from 9Router providers
// @Summary List available models from 9Router
// @Description Fetch available models and combos from 9Router Gateway
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.ChatModelsResponse
// @Router /chat/models [get]
func (h *ChatHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.chatService.ListAvailableModels(r.Context())
	if err != nil {
		h.log.Warn("failed to fetch models from 9Router, returning default", zap.Error(err))
		models = []string{"auto"}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response.BaseResponse[dto.ChatModelsResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    dto.ChatModelsResponse{Models: models},
	})
}

// handleActionError maps pending-action errors to explicit HTTP statuses.
func (h *ChatHandler) handleActionError(w http.ResponseWriter, err error, operation string) {
	switch {
	case errors.Is(err, service.ErrPendingActionHandled):
		h.errorHandler.HandleError(w, err, http.StatusConflict, "Đề xuất này đã được xử lý trước đó", operation)
	case errors.Is(err, service.ErrPendingActionExpired):
		h.errorHandler.HandleError(w, err, http.StatusGone, "Đề xuất đã hết hạn, vui lòng yêu cầu lại trợ lý AI", operation)
	case errors.Is(err, service.ErrPendingActionNotFound):
		h.errorHandler.HandleError(w, err, http.StatusNotFound, "Không tìm thấy đề xuất (có thể đã hết hạn)", operation)
	case errors.Is(err, service.ErrPendingActionFailed):
		msg := strings.TrimPrefix(err.Error(), service.ErrPendingActionFailed.Error()+": ")
		h.errorHandler.HandleError(w, err, http.StatusUnprocessableEntity, msg, operation)
	default:
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Không thể xử lý đề xuất", operation)
	}
}

// ConfirmAction executes an AI write proposal
// @Summary Confirm an AI write proposal
// @Description Executes the pending action proposed by the AI chat (PENDING_CONFIRMATION card, data.actionId). Only the user who received the proposal can confirm it; proposals expire after 15 minutes. A second confirm returns 409.
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param id path string true "Action ID"
// @Success 200 {object} dto.ChatActionResult
// @Failure 400 {object} response.ErrorResponse "Invalid action ID"
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Unknown action or another user's action"
// @Failure 409 {object} response.ErrorResponse "Already confirmed or cancelled"
// @Failure 410 {object} response.ErrorResponse "Proposal expired"
// @Failure 422 {object} response.ErrorResponse "Execution rejected (e.g. wallet no longer exists, insufficient balance); the proposal stays pending"
// @Failure 500 {object} response.ErrorResponse
// @Router /chat/actions/{id}/confirm [post]
func (h *ChatHandler) ConfirmAction(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "confirm_chat_action")
		return
	}
	actionID, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid action ID", "confirm_chat_action")
		return
	}
	result, err := h.chatService.ConfirmAction(r.Context(), user.ID, actionID)
	if err != nil {
		h.handleActionError(w, err, "confirm_chat_action")
		return
	}
	h.errorHandler.HandleSuccess(w, http.StatusOK, result)
}

// CancelAction discards an AI write proposal
// @Summary Cancel an AI write proposal
// @Description Cancels a pending action proposed by the AI chat. Nothing is written.
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param id path string true "Action ID"
// @Success 200 {object} dto.ChatActionResult
// @Failure 400 {object} response.ErrorResponse "Invalid action ID"
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "Unknown action or another user's action"
// @Failure 409 {object} response.ErrorResponse "Already confirmed or cancelled"
// @Failure 410 {object} response.ErrorResponse "Proposal expired"
// @Failure 500 {object} response.ErrorResponse
// @Router /chat/actions/{id}/cancel [post]
func (h *ChatHandler) CancelAction(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "cancel_chat_action")
		return
	}
	actionID, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid action ID", "cancel_chat_action")
		return
	}
	result, err := h.chatService.CancelAction(r.Context(), user.ID, actionID)
	if err != nil {
		h.handleActionError(w, err, "cancel_chat_action")
		return
	}
	h.errorHandler.HandleSuccess(w, http.StatusOK, result)
}
