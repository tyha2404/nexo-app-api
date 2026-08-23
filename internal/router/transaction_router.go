package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
)

type TransactionRouter struct {
	handler        *handler.TransactionHandler
	authMiddleware func(http.Handler) http.Handler
}

func NewTransactionRouter(handler *handler.TransactionHandler, authMiddleware func(http.Handler) http.Handler) *TransactionRouter {
	return &TransactionRouter{
		handler:        handler,
		authMiddleware: authMiddleware,
	}
}

func (r *TransactionRouter) RegisterRoutes(router chi.Router) {
	router.Route("/transactions", func(router chi.Router) {
		router.Use(r.authMiddleware)
		router.Post("/", r.handler.CreateTransaction)
		router.Get("/", r.handler.ListTransactions)
		router.Get("/{id}", r.handler.GetTransaction)
		router.Put("/{id}", r.handler.UpdateTransaction)
		router.Delete("/{id}", r.handler.DeleteTransaction)
	})
}
