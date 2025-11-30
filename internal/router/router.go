package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// New creates a new router with all routes configured
func New(db *gorm.DB, logger *zap.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Add logging middleware to log query strings
	r.Use(middleware.LoggingMiddleware(logger))

	// Initialize repositories
	userRepo := repository.NewUserRepo(db)
	categoryRepo := repository.NewCategoryRepo(db)
	costRepo := repository.NewCostRepo(db)

	// Initialize services
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)
	categoryService := service.NewCategoryService(categoryRepo)
	costService := service.NewCostService(costRepo)

	// Initialize handlers
	healthHandler := handler.NewHealthHandler(db, logger)
	authHandler := handler.NewAuthHandler(authService, logger)
	userHandler := handler.NewUserHandler(userService, logger)
	categoryHandler := handler.NewCategoryHandler(categoryService, logger)
	costHandler := handler.NewCostHandler(costService, logger)

	// Initialize routers
	healthRouter := NewHealthRouter(healthHandler)
	authRouter := NewAuthRouter(authHandler, logger)
	userRouter := NewUserRouter(userHandler, logger)
	categoryRouter := NewCategoryRouter(categoryHandler, logger)
	costRouter := NewCostRouter(costHandler, logger)

	// Register health check routes (outside API versioning)

	// Register all routes
	r.Route("/api/v1", func(apiRouter chi.Router) {
		healthRouter.RegisterRoutes(apiRouter)
		authRouter.RegisterRoutes(apiRouter)
		userRouter.RegisterRoutes(apiRouter)
		categoryRouter.RegisterRoutes(apiRouter)
		costRouter.RegisterRoutes(apiRouter)
	})

	// Register Swagger UI route
	AddSwaggerRoute(r)

	return r
}
