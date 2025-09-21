package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// New creates a new router with all routes configured
func New(db *gorm.DB, logger *zap.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Initialize repositories
	userRepo := repository.NewUserRepo(db)
	categoryRepo := repository.NewCategoryRepo(db)

	// Initialize services
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)
	categoryService := service.NewCategoryService(categoryRepo)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(authService, logger)
	userHandler := handler.NewUserHandler(userService, logger)
	categoryHandler := handler.NewCategoryHandler(categoryService, logger)

	// Initialize routers
	authRouter := NewAuthRouter(authHandler, logger)
	userRouter := NewUserRouter(userHandler, logger)
	categoryRouter := NewCategoryRouter(categoryHandler, logger)

	// Register all routes
	r.Route("/api/v1", func(apiRouter chi.Router) {
		authRouter.RegisterRoutes(apiRouter)
		userRouter.RegisterRoutes(apiRouter)
		categoryRouter.RegisterRoutes(apiRouter)
	})

	// Register Swagger UI route
	AddSwaggerRoute(r)

	return r
}
