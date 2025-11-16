package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
)

type HealthRouter struct {
	handler *handler.HealthHandler
}

func NewHealthRouter(handler *handler.HealthHandler) *HealthRouter {
	return &HealthRouter{handler: handler}
}

func (r *HealthRouter) RegisterRoutes(router chi.Router) {
	router.Get("/health", r.handler.Health)
	router.Get("/ready", r.handler.Ready)
	router.Get("/live", r.handler.Live)
}
