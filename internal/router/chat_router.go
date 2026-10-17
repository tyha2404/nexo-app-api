package router

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

// ChatStreamLimitPerMinute is how many AI chat messages one user may send per minute.
const ChatStreamLimitPerMinute = 20

type ChatRouter struct {
	handler       *handler.ChatHandler
	streamLimiter *middleware.RateLimiter
}

func NewChatRouter(handler *handler.ChatHandler) *ChatRouter {
	return &ChatRouter{
		handler: handler,
		streamLimiter: middleware.NewRateLimiter(ChatStreamLimitPerMinute, time.Minute, middleware.UserOrIPKey,
			"Bạn gửi tin nhắn quá nhanh, vui lòng chờ một chút rồi thử lại"),
	}
}

func (r *ChatRouter) RegisterRoutes(router chi.Router) {
	router.Route("/chat", func(router chi.Router) {
		router.Use(middleware.AuthMiddleware)
		// Rate limited per user (the key needs the user from AuthMiddleware).
		router.With(r.streamLimiter.Middleware).Post("/stream", r.handler.StreamMessage)
		router.Get("/models", r.handler.ListModels)
		router.Get("/sessions", r.handler.ListSessions)
		router.Get("/sessions/{id}", r.handler.GetSessionMessages)
		router.Delete("/sessions/{id}", r.handler.DeleteSession)
		router.Post("/clear", r.handler.ClearSessions)
		router.Post("/actions/{id}/confirm", r.handler.ConfirmAction)
		router.Post("/actions/{id}/cancel", r.handler.CancelAction)
	})
}
