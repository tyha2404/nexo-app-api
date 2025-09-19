package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
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

// RegisterRoutes registers all cost-related routes to the router
func (r *CostRouter) RegisterRoutes(router chi.Router) {
	// Add middleware here if needed (e.g., authentication, logging)
	router.Route("/costs", func(costRouter chi.Router) {
		costRouter.Post("/", r.handler.Create)
		costRouter.Get("/", r.handler.List)
		costRouter.Get("/{id}", r.handler.Get)
		costRouter.Put("/{id}", r.handler.Update)
		costRouter.Delete("/{id}", r.handler.Delete)
	})
}
