package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"go.uber.org/zap"
)

type UserRouter struct {
	handler *handler.UserHandler
	logger  *zap.Logger
}

// NewUserRouter creates a new instance of UserRouter
func NewUserRouter(handler *handler.UserHandler, logger *zap.Logger) *UserRouter {
	return &UserRouter{
		handler: handler,
		logger:  logger,
	}
}

// RegisterRoutes registers all category-related routes to the router
func (r *UserRouter) RegisterRoutes(router chi.Router) {
	// Add middleware here if needed (e.g., authentication, logging)
	router.Route("/users", func(usersRoute chi.Router) {
		usersRoute.Post("/", r.handler.Create)
		usersRoute.Get("/", r.handler.List)
		usersRoute.Get("/{id}", r.handler.Get)
		usersRoute.Put("/{id}", r.handler.Update)
		usersRoute.Delete("/{id}", r.handler.Delete)
	})
}
