package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
)

// TagRouter serves the user's transaction tags. Tags live on transactions
// (transactions.tags), so the handler is the TransactionHandler.
type TagRouter struct {
	handler        *handler.TransactionHandler
	authMiddleware func(http.Handler) http.Handler
}

func NewTagRouter(handler *handler.TransactionHandler, authMiddleware func(http.Handler) http.Handler) *TagRouter {
	return &TagRouter{handler: handler, authMiddleware: authMiddleware}
}

func (r *TagRouter) RegisterRoutes(router chi.Router) {
	router.Route("/tags", func(router chi.Router) {
		router.Use(r.authMiddleware)
		router.Get("/", r.handler.ListTags)
	})
}
