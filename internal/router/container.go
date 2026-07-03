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
	BudgetRepo      repository.BudgetRepository
	AlertRepo       repository.AlertRepository

	// Services
	AuthService        service.AuthService
	UserService        service.UserService
	CategoryService    service.CategoryService
	CostService        service.CostService
	TransactionService service.TransactionService
	BudgetService      service.BudgetService
	AlertService       service.AlertService
	ReportService      service.ReportService

	// Handlers
	HealthHandler      *handler.HealthHandler
	AuthHandler        *handler.AuthHandler
	UserHandler        *handler.UserHandler
	CategoryHandler    *handler.CategoryHandler
	CostHandler        *handler.CostHandler
	TransactionHandler *handler.TransactionHandler
	BudgetHandler      *handler.BudgetHandler
	AlertHandler       *handler.AlertHandler
	ReportHandler      *handler.ReportHandler
}

// NewContainer initializes and wires all dependencies
func NewContainer(db *gorm.DB, logger *zap.Logger) *Container {
	// 1. Initialize Repositories
	userRepo := repository.NewUserRepo(db)
	categoryRepo := repository.NewCategoryRepo(db)
	costRepo := repository.NewCostRepo(db)
	transactionRepo := repository.NewTransactionRepository(db)
	budgetRepo := repository.NewBudgetRepository(db)
	alertRepo := repository.NewAlertRepository(db)

	// 2. Initialize Services
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)
	categoryService := service.NewCategoryService(categoryRepo)
	costService := service.NewCostService(costRepo)
	transactionService := service.NewTransactionService(transactionRepo, categoryRepo, budgetRepo, alertRepo)
	budgetService := service.NewBudgetService(budgetRepo, categoryRepo)
	alertService := service.NewAlertService(alertRepo)
	reportService := service.NewReportService(transactionRepo)

	// 3. Initialize Handlers
	healthHandler := handler.NewHealthHandler(db, logger)
	authHandler := handler.NewAuthHandler(authService, logger)
	userHandler := handler.NewUserHandler(userService, logger)
	categoryHandler := handler.NewCategoryHandler(categoryService, logger)
	costHandler := handler.NewCostHandler(costService, logger)
	transactionHandler := handler.NewTransactionHandler(transactionService, logger)
	budgetHandler := handler.NewBudgetHandler(budgetService, logger)
	alertHandler := handler.NewAlertHandler(alertService)
	reportHandler := handler.NewReportHandler(reportService)

	return &Container{
		DB:                 db,
		Logger:             logger,
		UserRepo:           userRepo,
		CategoryRepo:       categoryRepo,
		CostRepo:           costRepo,
		TransactionRepo:    transactionRepo,
		BudgetRepo:         budgetRepo,
		AlertRepo:          alertRepo,
		AuthService:        authService,
		UserService:        userService,
		CategoryService:    categoryService,
		CostService:        costService,
		TransactionService: transactionService,
		BudgetService:      budgetService,
		AlertService:       alertService,
		ReportService:      reportService,
		HealthHandler:      healthHandler,
		AuthHandler:        authHandler,
		UserHandler:        userHandler,
		CategoryHandler:    categoryHandler,
		CostHandler:        costHandler,
		TransactionHandler: transactionHandler,
		BudgetHandler:      budgetHandler,
		AlertHandler:       alertHandler,
		ReportHandler:      reportHandler,
	}
}
