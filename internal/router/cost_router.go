package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

type CostRouter struct {
	handler *handler.CostHandler
}

// NewCostRouter creates a new instance of CostRouter
func NewCostRouter(handler *handler.CostHandler) *CostRouter {
	return &CostRouter{handler: handler}
}

// RegisterRoutes registers the legacy /costs routes (clients use /transactions).
func (r *CostRouter) RegisterRoutes(router chi.Router) {
	router.Route("/costs", func(costsRoute chi.Router) {
		costsRoute.Use(middleware.AuthMiddleware)
		costsRoute.Post("/", r.handler.Create)
		costsRoute.Get("/", r.handler.List)
		costsRoute.Get("/{id}", r.handler.Get)
		costsRoute.Put("/{id}", r.handler.Update)
		costsRoute.Delete("/{id}", r.handler.Delete)
	})
}
