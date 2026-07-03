package router

import (
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Container holds all instantiated dependencies for the application
type Container struct {
	DB     *gorm.DB
	Logger *zap.Logger

	// Repositories
	UserRepo        repository.UserRepo
	CategoryRepo    repository.CategoryRepo
	CostRepo        repository.CostRepo
	TransactionRepo repository.TransactionRepository

	// Services
	AuthService        service.AuthService
	UserService        service.UserService
	CategoryService    service.CategoryService
	CostService        service.CostService
	TransactionService service.TransactionService

	// Handlers
	HealthHandler      *handler.HealthHandler
	AuthHandler        *handler.AuthHandler
	UserHandler        *handler.UserHandler
	CategoryHandler    *handler.CategoryHandler
	CostHandler        *handler.CostHandler
	TransactionHandler *handler.TransactionHandler
}

// NewContainer initializes and wires all dependencies
func NewContainer(db *gorm.DB, logger *zap.Logger) *Container {
	// 1. Initialize Repositories
	userRepo := repository.NewUserRepo(db)
	categoryRepo := repository.NewCategoryRepo(db)
	costRepo := repository.NewCostRepo(db)
	transactionRepo := repository.NewTransactionRepository(db)

	// 2. Initialize Services
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)
	categoryService := service.NewCategoryService(categoryRepo)
	costService := service.NewCostService(costRepo)
	transactionService := service.NewTransactionService(transactionRepo, categoryRepo)

	// 3. Initialize Handlers
	healthHandler := handler.NewHealthHandler(db, logger)
	authHandler := handler.NewAuthHandler(authService, logger)
	userHandler := handler.NewUserHandler(userService, logger)
	categoryHandler := handler.NewCategoryHandler(categoryService, logger)
	costHandler := handler.NewCostHandler(costService, logger)
	transactionHandler := handler.NewTransactionHandler(transactionService, logger)

	return &Container{
		DB:                 db,
		Logger:             logger,
		UserRepo:           userRepo,
		CategoryRepo:       categoryRepo,
		CostRepo:           costRepo,
		TransactionRepo:    transactionRepo,
		AuthService:        authService,
		UserService:        userService,
		CategoryService:    categoryService,
		CostService:        costService,
		TransactionService: transactionService,
		HealthHandler:      healthHandler,
		AuthHandler:        authHandler,
		UserHandler:        userHandler,
		CategoryHandler:    categoryHandler,
		CostHandler:        costHandler,
		TransactionHandler: transactionHandler,
	}
}
