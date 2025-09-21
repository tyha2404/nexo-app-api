package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
	"go.uber.org/zap"
)

type AuthRouter struct {
	handler *handler.AuthHandler
	logger  *zap.Logger
}

// NewAuthRouter creates a new instance of AuthRouter
func NewAuthRouter(handler *handler.AuthHandler, logger *zap.Logger) *AuthRouter {
	return &AuthRouter{
		handler: handler,
		logger:  logger,
	}
}

// RegisterRoutes registers all auth-related routes to the router
func (r *AuthRouter) RegisterRoutes(router chi.Router) {
	router.Route("/auth", func(authRoute chi.Router) {
		// Public routes
		authRoute.Post("/register", r.handler.Register)
		authRoute.Post("/login", r.handler.Login)

		// Protected routes - require authentication
		authRoute.Group(func(protectedRoute chi.Router) {
			protectedRoute.Use(middleware.AuthMiddleware)
			protectedRoute.Get("/whoami", r.handler.WhoAmI)
		})
	})
}
