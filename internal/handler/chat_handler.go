package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type ChatHandler struct {
	chatService service.ChatService
	log         *zap.Logger
}

func NewChatHandler(chatService service.ChatService, log *zap.Logger) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		log:         log,
	}
}

// StreamMessage handles sending a message and streaming AI response via Server-Sent Events (SSE)
// @Summary Send message and stream AI response
// @Description Stream financial advisory and knowledge responses using GLM-4 (Zhipu AI) and RAG
// @Tags chat
// @Accept json
// @Produce text/event-stream
// @Security BearerAuth
// @Param request body dto.SendMessageRequest true "Send Message Request"
// @Success 200 {string} string "SSE stream of dto.ChatStreamEvent"
// @Failure 400 {object} response.ErrorResponse "Bad Request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Router /chat/stream [post]
func (h *ChatHandler) StreamMessage(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req dto.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode chat stream request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message cannot be empty", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
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
// @Router /chat/sessions [get]
func (h *ChatHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessions, err := h.chatService.ListSessions(r.Context(), user.ID, 30)
	if err != nil {
		h.log.Error("failed to list chat sessions", zap.Error(err))
		http.Error(w, "Failed to list chat sessions", http.StatusInternalServerError)
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
// @Description Get messages for a specific session ID
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 200 {object} dto.ChatSessionResponse
// @Router /chat/sessions/{id} [get]
func (h *ChatHandler) GetSessionMessages(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	sess, err := h.chatService.GetSessionMessages(r.Context(), user.ID, sessionID, 50)
	if err != nil {
		h.log.Error("failed to get session messages", zap.Error(err))
		http.Error(w, "Session not found", http.StatusNotFound)
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
// @Router /chat/sessions/{id} [delete]
func (h *ChatHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	if err := h.chatService.DeleteSession(r.Context(), user.ID, sessionID); err != nil {
		h.log.Error("failed to delete chat session", zap.Error(err))
		http.Error(w, "Failed to delete session", http.StatusInternalServerError)
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
// @Router /chat/clear [post]
func (h *ChatHandler) ClearSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.chatService.ClearSessions(r.Context(), user.ID); err != nil {
		h.log.Error("failed to clear chat sessions", zap.Error(err))
		http.Error(w, "Failed to clear chat sessions", http.StatusInternalServerError)
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
