package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

type DebtRouter struct {
	handler *handler.DebtHandler
}

func NewDebtRouter(handler *handler.DebtHandler) *DebtRouter {
	return &DebtRouter{handler: handler}
}

func (r *DebtRouter) RegisterRoutes(router chi.Router) {
	router.Route("/debts", func(router chi.Router) {
		router.Use(middleware.AuthMiddleware)
		router.Get("/summary", r.handler.GetSummary)
		router.Get("/", r.handler.GetDebts)
		router.Post("/", r.handler.CreateDebt)
		router.Post("/{id}/repayments", r.handler.AddRepayment)
		router.Delete("/{id}", r.handler.DeleteDebt)
	})
}
