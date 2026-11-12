package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

// TagRouter serves the user's transaction tags. Tags live on transactions
// (transactions.tags), so the handler is the TransactionHandler.
type TagRouter struct {
	handler *handler.TransactionHandler
}

func NewTagRouter(handler *handler.TransactionHandler) *TagRouter {
	return &TagRouter{handler: handler}
}

func (r *TagRouter) RegisterRoutes(router chi.Router) {
	router.Route("/tags", func(router chi.Router) {
		router.Use(middleware.AuthMiddleware)
		router.Get("/", r.handler.ListTags)
	})
}
