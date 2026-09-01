package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

type CreditCardStatementRouter struct {
	handler *handler.CreditCardStatementHandler
}

func NewCreditCardStatementRouter(handler *handler.CreditCardStatementHandler) *CreditCardStatementRouter {
	return &CreditCardStatementRouter{handler: handler}
}

func (r *CreditCardStatementRouter) RegisterRoutes(router chi.Router) {
	router.Route("/credit-card-statements", func(router chi.Router) {
		router.Use(middleware.AuthMiddleware)
		router.Get("/", r.handler.ListStatements)
		router.Post("/", r.handler.CreateStatement)
		router.Get("/{id}", r.handler.GetStatementByID)
		router.Put("/{id}", r.handler.UpdateStatement)
		router.Post("/{id}/pay", r.handler.PayStatement)
		router.Delete("/{id}", r.handler.DeleteStatement)
	})
}
