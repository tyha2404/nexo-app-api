package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
	"github.com/tyha2404/nexo-app-api/internal/response"
)

// AuthLimitPerMinute is how many login/register/change-password attempts one
// client IP may make per minute (shared between the endpoints).
const AuthLimitPerMinute = 10

// RefreshLimitPerMinute is how many refresh/logout calls one client IP may
// make per minute. Higher than AuthLimitPerMinute: every open tab and device
// behind the same IP refreshes on its own.
const RefreshLimitPerMinute = 30

type AuthRouter struct {
	handler        *handler.AuthHandler
	authLimiter    *middleware.RateLimiter
	refreshLimiter *middleware.RateLimiter
	// DisableRegister makes POST /auth/register answer 403. Local sign-up
	// does not verify the email, so it could squat a Supabase user's address.
	DisableRegister bool
}

// NewAuthRouter creates a new instance of AuthRouter
func NewAuthRouter(handler *handler.AuthHandler) *AuthRouter {
	return &AuthRouter{
		handler: handler,
		authLimiter: middleware.NewRateLimiter(AuthLimitPerMinute, time.Minute, middleware.ClientIPKey,
			"Quá nhiều lần thử, vui lòng thử lại sau ít phút"),
		refreshLimiter: middleware.NewRateLimiter(RefreshLimitPerMinute, time.Minute, middleware.ClientIPKey,
			"Quá nhiều yêu cầu làm mới phiên, vui lòng thử lại sau ít phút"),
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
		// Public: the access token may already be expired. The refresh token
		// in the body is the credential.
		authRoute.With(r.refreshLimiter.Middleware).Post("/refresh", r.handler.Refresh)
		authRoute.With(r.refreshLimiter.Middleware).Post("/logout", r.handler.Logout)

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
