package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
	"go.uber.org/zap"
)

type CostRouter struct {
	handler *handler.CostHandler
	logger  *zap.Logger
}

// NewCostRouter creates a new instance of CostRouter
func NewCostRouter(handler *handler.CostHandler, logger *zap.Logger) *CostRouter {
	return &CostRouter{
		handler: handler,
		logger:  logger,
	}
}

// RegisterRoutes registers all category-related routes to the router
func (r *CostRouter) RegisterRoutes(router chi.Router) {
	// Add middleware here if needed (e.g., authentication, logging)
	router.Route("/costs", func(costsRoute chi.Router) {
		costsRoute.Use(middleware.AuthMiddleware)
		costsRoute.Post("/", r.handler.Create)
		costsRoute.Get("/", r.handler.List)
		costsRoute.Get("/{id}", r.handler.Get)
		costsRoute.Put("/{id}", r.handler.Update)
		costsRoute.Delete("/{id}", r.handler.Delete)
	})
}
