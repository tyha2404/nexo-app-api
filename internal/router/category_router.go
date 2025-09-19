package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"go.uber.org/zap"
)

type CategoryRouter struct {
	handler *handler.CategoryHandler
	logger  *zap.Logger
}

// NewCategoryRouter creates a new instance of CategoryRouter
func NewCategoryRouter(handler *handler.CategoryHandler, logger *zap.Logger) *CategoryRouter {
	return &CategoryRouter{
		handler: handler,
		logger:  logger,
	}
}

// RegisterRoutes registers all category-related routes to the router
func (r *CategoryRouter) RegisterRoutes(router chi.Router) {
	// Add middleware here if needed (e.g., authentication, logging)
	router.Route("/categories", func(categoriesRoute chi.Router) {
		categoriesRoute.Post("/", r.handler.Create)
		categoriesRoute.Get("/", r.handler.List)
		categoriesRoute.Get("/{id}", r.handler.Get)
		categoriesRoute.Put("/{id}", r.handler.Update)
		categoriesRoute.Delete("/{id}", r.handler.Delete)
	})
}
