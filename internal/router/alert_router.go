package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

type AlertRouter struct {
	handler *handler.AlertHandler
}

func NewAlertRouter(handler *handler.AlertHandler) *AlertRouter {
	return &AlertRouter{handler: handler}
}

func (r *AlertRouter) RegisterRoutes(router chi.Router) {
	router.Route("/alerts", func(router chi.Router) {
		router.Use(middleware.AuthMiddleware)
		router.Get("/", r.handler.List)
		router.Delete("/{id}", r.handler.Delete)
	})
}
