package router

import (
	"github.com/tyha2404/nexo-app-api/internal/config"
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
	UserRepo                repository.UserRepo
	CategoryRepo            repository.CategoryRepo
	CostRepo                repository.CostRepo
	TransactionRepo         repository.TransactionRepository
	BudgetRepo              repository.BudgetRepository
	AlertRepo               repository.AlertRepository
	TargetRepo              repository.TargetRepository
	DebtRepo                repository.DebtRepository
	PresetRepo              repository.PresetRepository
	WalletRepo              repository.WalletRepository
	CreditCardStatementRepo repository.CreditCardStatementRepository
	ChatRepo                repository.ChatRepository
	KnowledgeRepo           repository.KnowledgeRepository
	PushSubscriptionRepo    repository.PushSubscriptionRepository

	// Services
	AuthService                service.AuthService
	UserService                service.UserService
	CategoryService            service.CategoryService
	CostService                service.CostService
	TransactionService         service.TransactionService
	BudgetService              service.BudgetService
	AlertService               service.AlertService
	ReportService              service.ReportService
	TargetService              service.TargetService
	DebtService                service.DebtService
	PresetService              service.PresetService
	WalletService              service.WalletService
	CreditCardStatementService service.CreditCardStatementService
	RequestyService            service.RequestyService
	RAGService                 service.RAGService
	ChatService                service.ChatService
	NotificationService        service.NotificationService

	// Handlers
	HealthHandler              *handler.HealthHandler
	AuthHandler                *handler.AuthHandler
	UserHandler                *handler.UserHandler
	CategoryHandler            *handler.CategoryHandler
	CostHandler                *handler.CostHandler
	TransactionHandler         *handler.TransactionHandler
	BudgetHandler              *handler.BudgetHandler
	AlertHandler               *handler.AlertHandler
	ReportHandler              *handler.ReportHandler
	TargetHandler              *handler.TargetHandler
	DebtHandler                *handler.DebtHandler
	PresetHandler              *handler.PresetHandler
	WalletHandler              *handler.WalletHandler
	CreditCardStatementHandler *handler.CreditCardStatementHandler
	ChatHandler                *handler.ChatHandler
	NotificationHandler        *handler.NotificationHandler
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
	ccStatementRepo := repository.NewCreditCardStatementRepository(db)
	chatRepo := repository.NewChatRepository(db)
	knowledgeRepo := repository.NewKnowledgeRepository(db)
	pushSubscriptionRepo := repository.NewPushSubscriptionRepository(db)

	cfg, _ := config.LoadConfig()
	if cfg == nil {
		cfg = &config.Config{}
	}

	// 2. Initialize Services
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)
	categoryService := service.NewCategoryService(categoryRepo)
	costService := service.NewCostService(costRepo)
	transactionService := service.NewTransactionService(transactionRepo, categoryRepo, budgetRepo, alertRepo)
	budgetService := service.NewBudgetService(budgetRepo, categoryRepo)
	alertService := service.NewAlertService(alertRepo)
	reportService := service.NewReportService(transactionRepo)
	targetService := service.NewTargetService(targetRepo)
	debtService := service.NewDebtService(debtRepo)
	presetService := service.NewPresetService(presetRepo, categoryRepo)
	walletService := service.NewWalletService(walletRepo)
	ccStatementService := service.NewCreditCardStatementService(ccStatementRepo, walletRepo)
	requestyService := service.NewRequestyService(cfg, logger)
	ragService := service.NewRAGService(knowledgeRepo, requestyService, logger)
	chatService := service.NewChatService(
		chatRepo,
		requestyService,
		ragService,
		transactionService,
		reportService,
		budgetService,
		debtService,
		walletService,
		targetService,
		categoryRepo,
		transactionRepo,
		walletRepo,
		logger,
	)
	notificationService := service.NewNotificationService(pushSubscriptionRepo, cfg, logger)

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
	targetHandler := handler.NewTargetHandler(targetService, logger)
	debtHandler := handler.NewDebtHandler(debtService, logger)
	presetHandler := handler.NewPresetHandler(presetService, logger)
	walletHandler := handler.NewWalletHandler(walletService, logger)
	ccStatementHandler := handler.NewCreditCardStatementHandler(ccStatementService, logger)
	chatHandler := handler.NewChatHandler(chatService, logger)
	notificationHandler := handler.NewNotificationHandler(notificationService, logger)

	return &Container{
		DB:                         db,
		Logger:                     logger,
		UserRepo:                   userRepo,
		CategoryRepo:               categoryRepo,
		CostRepo:                   costRepo,
		TransactionRepo:            transactionRepo,
		BudgetRepo:                 budgetRepo,
		AlertRepo:                  alertRepo,
		TargetRepo:                 targetRepo,
		DebtRepo:                   debtRepo,
		PresetRepo:                 presetRepo,
		WalletRepo:                 walletRepo,
		CreditCardStatementRepo:    ccStatementRepo,
		ChatRepo:                   chatRepo,
		KnowledgeRepo:              knowledgeRepo,
		PushSubscriptionRepo:       pushSubscriptionRepo,
		AuthService:                authService,
		UserService:                userService,
		CategoryService:            categoryService,
		CostService:                costService,
		TransactionService:         transactionService,
		BudgetService:              budgetService,
		AlertService:               alertService,
		ReportService:              reportService,
		TargetService:              targetService,
		DebtService:                debtService,
		PresetService:              presetService,
		WalletService:              walletService,
		CreditCardStatementService: ccStatementService,
		RequestyService:            requestyService,
		RAGService:                 ragService,
		ChatService:                chatService,
		NotificationService:        notificationService,
		HealthHandler:              healthHandler,
		AuthHandler:                authHandler,
		UserHandler:                userHandler,
		CategoryHandler:            categoryHandler,
		CostHandler:                costHandler,
		TransactionHandler:         transactionHandler,
		BudgetHandler:              budgetHandler,
		AlertHandler:               alertHandler,
		ReportHandler:              reportHandler,
		TargetHandler:              targetHandler,
		DebtHandler:                debtHandler,
		PresetHandler:              presetHandler,
		WalletHandler:              walletHandler,
		CreditCardStatementHandler: ccStatementHandler,
		ChatHandler:                chatHandler,
		NotificationHandler:        notificationHandler,
	}
}
