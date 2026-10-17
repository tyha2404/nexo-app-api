package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Options configures cross-cutting HTTP behaviour of the router.
type Options struct {
	// CORSAllowedOrigins is the browser-origin allowlist. When nil,
	// config.DefaultCORSAllowedOrigins is used.
	CORSAllowedOrigins []string
}

// Option mutates Options.
type Option func(*Options)

// WithCORSAllowedOrigins sets the CORS origin allowlist.
func WithCORSAllowedOrigins(origins []string) Option {
	return func(o *Options) { o.CORSAllowedOrigins = origins }
}

// New creates a new router with all routes configured
func New(db *gorm.DB, logger *zap.Logger, opts ...Option) *chi.Mux {
	options := Options{}
	for _, opt := range opts {
		opt(&options)
	}
	if options.CORSAllowedOrigins == nil {
		options.CORSAllowedOrigins = config.ParseOrigins(config.DefaultCORSAllowedOrigins)
	}

	r := chi.NewRouter()

	// Add logging and CORS middleware
	r.Use(middleware.NewCORSMiddleware(options.CORSAllowedOrigins))
	r.Use(middleware.LoggingMiddleware(logger))

	// Initialize dependency container
	container := NewContainer(db, logger)

	// Initialize routers using handlers from the container
	healthRouter := NewHealthRouter(container.HealthHandler)
	authRouter := NewAuthRouter(container.AuthHandler, logger)
	userRouter := NewUserRouter(container.UserHandler, logger)
	categoryRouter := NewCategoryRouter(container.CategoryHandler, logger)
	costRouter := NewCostRouter(container.CostHandler, logger)
	transactionRouter := NewTransactionRouter(container.TransactionHandler, middleware.AuthMiddleware)
	budgetRouter := NewBudgetRouter(container.BudgetHandler)
	alertRouter := NewAlertRouter(container.AlertHandler)
	reportRouter := NewReportRouter(container.ReportHandler)
	targetRouter := NewTargetRouter(container.TargetHandler)
	debtRouter := NewDebtRouter(container.DebtHandler)
	presetRouter := NewPresetRouter(container.PresetHandler)
	walletRouter := NewWalletRouter(container.WalletHandler)
	ccStatementRouter := NewCreditCardStatementRouter(container.CreditCardStatementHandler)
	chatRouter := NewChatRouter(container.ChatHandler)

	// Register all routes
	r.Route("/api/v1", func(apiRouter chi.Router) {
		healthRouter.RegisterRoutes(apiRouter)
		authRouter.RegisterRoutes(apiRouter)
		userRouter.RegisterRoutes(apiRouter)
		categoryRouter.RegisterRoutes(apiRouter)
		costRouter.RegisterRoutes(apiRouter)
		transactionRouter.RegisterRoutes(apiRouter)
		budgetRouter.RegisterRoutes(apiRouter)
		alertRouter.RegisterRoutes(apiRouter)
		reportRouter.RegisterRoutes(apiRouter)
		targetRouter.RegisterRoutes(apiRouter)
		debtRouter.RegisterRoutes(apiRouter)
		presetRouter.RegisterRoutes(apiRouter)
		walletRouter.RegisterRoutes(apiRouter)
		ccStatementRouter.RegisterRoutes(apiRouter)
		chatRouter.RegisterRoutes(apiRouter)
	})

	// Register Swagger UI route
	AddSwaggerRoute(r)

	return r
}
