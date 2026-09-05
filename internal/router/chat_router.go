package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

type ChatRouter struct {
	handler *handler.ChatHandler
}

func NewChatRouter(handler *handler.ChatHandler) *ChatRouter {
	return &ChatRouter{handler: handler}
}

func (r *ChatRouter) RegisterRoutes(router chi.Router) {
	router.Route("/chat", func(router chi.Router) {
		router.Use(middleware.AuthMiddleware)
		router.Post("/stream", r.handler.StreamMessage)
		router.Get("/models", r.handler.ListModels)
		router.Get("/sessions", r.handler.ListSessions)
		router.Get("/sessions/{id}", r.handler.GetSessionMessages)
		router.Delete("/sessions/{id}", r.handler.DeleteSession)
		router.Post("/clear", r.handler.ClearSessions)
	})
}
