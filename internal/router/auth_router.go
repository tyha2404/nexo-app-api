package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"go.uber.org/zap"
)

// AuthLimitPerMinute is how many login/register/change-password attempts one
// client IP may make per minute (shared between the endpoints).
const AuthLimitPerMinute = 10

type AuthRouter struct {
	handler     *handler.AuthHandler
	logger      *zap.Logger
	authLimiter *middleware.RateLimiter
	// DisableRegister makes POST /auth/register answer 403. Local sign-up
	// does not verify the email, so it could squat a Supabase user's address.
	DisableRegister bool
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
		register := r.handler.Register
		if r.DisableRegister {
			register = registerDisabled
		}
		authRoute.With(r.authLimiter.Middleware).Post("/register", register)
		authRoute.With(r.authLimiter.Middleware).Post("/login", r.handler.Login)

		// Protected routes - require authentication
		authRoute.Group(func(protectedRoute chi.Router) {
			protectedRoute.Use(middleware.AuthMiddleware)
			protectedRoute.Get("/whoami", r.handler.WhoAmI)
			protectedRoute.With(r.authLimiter.Middleware).Post("/change-password", r.handler.ChangePassword)
		})
	})
}

// registerDisabled answers POST /auth/register when local sign-up is off.
func registerDisabled(w http.ResponseWriter, _ *http.Request) {
	response.SendError(w, http.StatusForbidden, "Đăng ký tài khoản cục bộ đã bị tắt, vui lòng đăng ký bằng email", nil)
}
