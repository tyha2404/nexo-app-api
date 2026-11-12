package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

type CategoryRouter struct {
	handler *handler.CategoryHandler
}

// NewCategoryRouter creates a new instance of CategoryRouter
func NewCategoryRouter(handler *handler.CategoryHandler) *CategoryRouter {
	return &CategoryRouter{handler: handler}
}

// RegisterRoutes registers all category-related routes to the router
func (r *CategoryRouter) RegisterRoutes(router chi.Router) {
	router.Route("/categories", func(categoriesRoute chi.Router) {
		categoriesRoute.Use(middleware.AuthMiddleware)
		categoriesRoute.Post("/", r.handler.Create)
		categoriesRoute.Get("/", r.handler.List)
		categoriesRoute.Get("/{id}", r.handler.Get)
		categoriesRoute.Put("/{id}", r.handler.Update)
		categoriesRoute.Delete("/{id}", r.handler.Delete)
	})
}
