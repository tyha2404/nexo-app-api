package router

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
	// MaxBodyBytes caps request bodies. When 0,
	// middleware.DefaultMaxBodyBytes is used.
	MaxBodyBytes int64
	// DisableSwagger hides /swagger (production).
	DisableSwagger bool
	// DisableLocalRegister turns off POST /auth/register, so accounts are
	// only created through Supabase Auth, which verifies the email.
	DisableLocalRegister bool
}

// Option mutates Options.
type Option func(*Options)

// WithCORSAllowedOrigins sets the CORS origin allowlist.
func WithCORSAllowedOrigins(origins []string) Option {
	return func(o *Options) { o.CORSAllowedOrigins = origins }
}

// WithSwaggerDisabled hides the Swagger UI and spec.
func WithSwaggerDisabled(disabled bool) Option {
	return func(o *Options) { o.DisableSwagger = disabled }
}

// WithLocalRegisterDisabled turns off local (non-Supabase) registration.
func WithLocalRegisterDisabled(disabled bool) Option {
	return func(o *Options) { o.DisableLocalRegister = disabled }
}

// New creates a new router with all routes configured. cfg is the
// configuration loaded in cmd/server/main.go (see NewContainer).
func New(cfg *config.Config, db *gorm.DB, logger *zap.Logger, opts ...Option) *chi.Mux {
	options := Options{}
	for _, opt := range opts {
		opt(&options)
	}
	if options.CORSAllowedOrigins == nil {
		options.CORSAllowedOrigins = config.ParseOrigins(config.DefaultCORSAllowedOrigins)
	}
	if options.MaxBodyBytes == 0 {
		options.MaxBodyBytes = middleware.DefaultMaxBodyBytes
	}

	r := chi.NewRouter()

	// Panic recovery first so it covers every other middleware and handler.
	r.Use(middleware.Recoverer(logger))
	// Add logging and CORS middleware
	r.Use(middleware.NewCORSMiddleware(options.CORSAllowedOrigins))
	r.Use(middleware.LoggingMiddleware(logger, cfg.IsProduction()))
	r.Use(middleware.MaxBodyBytes(options.MaxBodyBytes))

	// Initialize dependency container
	container := NewContainer(cfg, db, logger)

	// Initialize routers using handlers from the container
	healthRouter := NewHealthRouter(container.HealthHandler)
	authRouter := NewAuthRouter(container.AuthHandler)
	authRouter.DisableRegister = options.DisableLocalRegister
	userRouter := NewUserRouter(container.UserHandler)
	userRouter.RoleLookup = func(ctx context.Context, id uuid.UUID) (string, error) {
		u, err := container.UserRepo.GetByID(ctx, id)
		if err != nil {
			return "", err
		}
		return u.Role, nil
	}
	categoryRouter := NewCategoryRouter(container.CategoryHandler)
	costRouter := NewCostRouter(container.CostHandler)
	transactionRouter := NewTransactionRouter(container.TransactionHandler)
	tagRouter := NewTagRouter(container.TransactionHandler)
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
		tagRouter.RegisterRoutes(apiRouter)
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
	if !options.DisableSwagger {
		AddSwaggerRoute(r)
	}

	return r
}
