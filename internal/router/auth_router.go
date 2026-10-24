package router

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
	"go.uber.org/zap"
)

// AuthLimitPerMinute is how many login/register/change-password attempts one
// client IP may make per minute (shared between the endpoints).
const AuthLimitPerMinute = 10

type AuthRouter struct {
	handler     *handler.AuthHandler
	logger      *zap.Logger
	authLimiter *middleware.RateLimiter
}

// NewAuthRouter creates a new instance of AuthRouter
func NewAuthRouter(handler *handler.AuthHandler, logger *zap.Logger) *AuthRouter {
	return &AuthRouter{
		handler: handler,
		logger:  logger,
		authLimiter: middleware.NewRateLimiter(AuthLimitPerMinute, time.Minute, middleware.ClientIPKey,
			"Quá nhiều lần thử, vui lòng thử lại sau ít phút"),
	}
}

// RegisterRoutes registers all auth-related routes to the router
func (r *AuthRouter) RegisterRoutes(router chi.Router) {
	router.Route("/auth", func(authRoute chi.Router) {
		// Public routes, rate limited per client IP against brute force.
		authRoute.With(r.authLimiter.Middleware).Post("/register", r.handler.Register)
		authRoute.With(r.authLimiter.Middleware).Post("/login", r.handler.Login)

		// Protected routes - require authentication
		authRoute.Group(func(protectedRoute chi.Router) {
			protectedRoute.Use(middleware.AuthMiddleware)
			protectedRoute.Get("/whoami", r.handler.WhoAmI)
			protectedRoute.With(r.authLimiter.Middleware).Post("/change-password", r.handler.ChangePassword)
		})
	})
}
