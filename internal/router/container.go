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
	TargetRepo      repository.TargetRepository
	DebtRepo        repository.DebtRepository
	PresetRepo      repository.PresetRepository
	WalletRepo      repository.WalletRepository

	// Services
	AuthService        service.AuthService
	UserService        service.UserService
	CategoryService    service.CategoryService
	CostService        service.CostService
	TransactionService service.TransactionService
	NLPService         service.NLPService
	BudgetService      service.BudgetService
	AlertService       service.AlertService
	ReportService      service.ReportService
	TargetService      service.TargetService
	DebtService        service.DebtService
	PresetService      service.PresetService
	WalletService      service.WalletService

	// Handlers
	HealthHandler      *handler.HealthHandler
	AuthHandler        *handler.AuthHandler
	UserHandler        *handler.UserHandler
	CategoryHandler    *handler.CategoryHandler
	CostHandler        *handler.CostHandler
	TransactionHandler *handler.TransactionHandler
	NLPHandler         *handler.NLPHandler
	BudgetHandler      *handler.BudgetHandler
	AlertHandler       *handler.AlertHandler
	ReportHandler      *handler.ReportHandler
	TargetHandler      *handler.TargetHandler
	DebtHandler        *handler.DebtHandler
	PresetHandler      *handler.PresetHandler
	WalletHandler      *handler.WalletHandler
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
	targetRepo := repository.NewTargetRepository(db)
	debtRepo := repository.NewDebtRepository(db)
	presetRepo := repository.NewPresetRepository(db)
	walletRepo := repository.NewWalletRepository(db)

	// 2. Initialize Services
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)
	categoryService := service.NewCategoryService(categoryRepo)
	costService := service.NewCostService(costRepo)
	transactionService := service.NewTransactionService(transactionRepo, categoryRepo, budgetRepo, alertRepo)
	nlpService := service.NewNLPService(categoryRepo)
	budgetService := service.NewBudgetService(budgetRepo, categoryRepo)
	alertService := service.NewAlertService(alertRepo)
	reportService := service.NewReportService(transactionRepo)
	targetService := service.NewTargetService(targetRepo)
	debtService := service.NewDebtService(debtRepo)
	presetService := service.NewPresetService(presetRepo, categoryRepo)
	walletService := service.NewWalletService(walletRepo)

	// 3. Initialize Handlers
	healthHandler := handler.NewHealthHandler(db, logger)
	authHandler := handler.NewAuthHandler(authService, logger)
	userHandler := handler.NewUserHandler(userService, logger)
	categoryHandler := handler.NewCategoryHandler(categoryService, logger)
	costHandler := handler.NewCostHandler(costService, logger)
	nlpHandler := handler.NewNLPHandler(nlpService, logger)
	transactionHandler := handler.NewTransactionHandler(transactionService, nlpHandler, logger)
	budgetHandler := handler.NewBudgetHandler(budgetService, logger)
	alertHandler := handler.NewAlertHandler(alertService)
	reportHandler := handler.NewReportHandler(reportService)
	targetHandler := handler.NewTargetHandler(targetService, logger)
	debtHandler := handler.NewDebtHandler(debtService, logger)
	presetHandler := handler.NewPresetHandler(presetService, logger)
	walletHandler := handler.NewWalletHandler(walletService, logger)

	return &Container{
		DB:                 db,
		Logger:             logger,
		UserRepo:           userRepo,
		CategoryRepo:       categoryRepo,
		CostRepo:           costRepo,
		TransactionRepo:    transactionRepo,
		BudgetRepo:         budgetRepo,
		AlertRepo:          alertRepo,
		TargetRepo:         targetRepo,
		DebtRepo:           debtRepo,
		PresetRepo:         presetRepo,
		WalletRepo:         walletRepo,
		AuthService:        authService,
		UserService:        userService,
		CategoryService:    categoryService,
		CostService:        costService,
		TransactionService: transactionService,
		NLPService:         nlpService,
		BudgetService:      budgetService,
		AlertService:       alertService,
		ReportService:      reportService,
		TargetService:      targetService,
		DebtService:        debtService,
		PresetService:      presetService,
		WalletService:      walletService,
		HealthHandler:      healthHandler,
		AuthHandler:        authHandler,
		UserHandler:        userHandler,
		CategoryHandler:    categoryHandler,
		CostHandler:        costHandler,
		TransactionHandler: transactionHandler,
		NLPHandler:         nlpHandler,
		BudgetHandler:      budgetHandler,
		AlertHandler:       alertHandler,
		ReportHandler:      reportHandler,
		TargetHandler:      targetHandler,
		DebtHandler:        debtHandler,
		PresetHandler:      presetHandler,
		WalletHandler:      walletHandler,
	}
}
