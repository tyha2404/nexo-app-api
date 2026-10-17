package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
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

// RegisterRoutes registers the user-management routes. They manage arbitrary
// accounts, so every route requires an authenticated admin. Regular users get
// their own profile from GET /auth/whoami.
func (r *UserRouter) RegisterRoutes(router chi.Router) {
	router.Route("/users", func(usersRoute chi.Router) {
		usersRoute.Use(middleware.AuthMiddleware)
		usersRoute.Use(middleware.AdminOnly)
		usersRoute.Post("/", r.handler.Create)
		usersRoute.Get("/", r.handler.List)
		usersRoute.Get("/{id}", r.handler.Get)
		usersRoute.Put("/{id}", r.handler.Update)
		usersRoute.Delete("/{id}", r.handler.Delete)
	})
}
