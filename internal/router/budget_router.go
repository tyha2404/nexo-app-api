package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

type BudgetRouter struct {
	handler *handler.BudgetHandler
}

func NewBudgetRouter(handler *handler.BudgetHandler) *BudgetRouter {
	return &BudgetRouter{handler: handler}
}

func (r *BudgetRouter) RegisterRoutes(router chi.Router) {
	router.Route("/budgets", func(router chi.Router) {
		router.Use(middleware.AuthMiddleware)
		router.Post("/", r.handler.Create)
		router.Get("/", r.handler.List)
		router.Get("/{id}", r.handler.Get)
		router.Put("/{id}", r.handler.Update)
		router.Delete("/{id}", r.handler.Delete)
	})
}
