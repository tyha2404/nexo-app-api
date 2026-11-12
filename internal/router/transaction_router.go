package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

type TransactionRouter struct {
	handler *handler.TransactionHandler
}

func NewTransactionRouter(handler *handler.TransactionHandler) *TransactionRouter {
	return &TransactionRouter{handler: handler}
}

func (r *TransactionRouter) RegisterRoutes(router chi.Router) {
	router.Route("/transactions", func(router chi.Router) {
		router.Use(middleware.AuthMiddleware)
		router.Post("/", r.handler.CreateTransaction)
		router.Get("/", r.handler.ListTransactions)
		router.Get("/{id}", r.handler.GetTransaction)
		router.Put("/{id}", r.handler.UpdateTransaction)
		router.Delete("/{id}", r.handler.DeleteTransaction)
		router.Post("/{id}/restore", r.handler.RestoreTransaction)
	})
}
