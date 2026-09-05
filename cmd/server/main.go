// @title Nexo API
// @version 1.0
// @description Nexo API documentation
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/db"
	"github.com/tyha2404/nexo-app-api/internal/logger"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/router"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"github.com/tyha2404/nexo-app-api/internal/worker"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialize JWT before using it
	util.InitJWT(cfg)

	logg, err := logger.New(cfg.LogLevel, cfg.AppEnv)
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer logg.Sync()

	gormDB, err := db.NewPostgres(cfg, logg)
	if err != nil {
		logg.Sugar().Fatalf("failed to connect db: %v", err)
	}

	// Initialize and start background workers
	rolloverRepo := repository.NewRolloverRepository(gormDB)
	rolloverService := service.NewRolloverService(rolloverRepo, logg)
	rolloverWorker := worker.NewMonthlyRolloverWorker(rolloverService, logg)
	rolloverWorker.Start(context.Background())

	pushSubRepo := repository.NewPushSubscriptionRepository(gormDB)
	notifService := service.NewNotificationService(pushSubRepo, cfg, logg)
	stmtRepo := repository.NewCreditCardStatementRepository(gormDB)
	debtRepo := repository.NewDebtRepository(gormDB)
	targetRepo := repository.NewTargetRepository(gormDB)
	txRepo := repository.NewTransactionRepository(gormDB)
	notifWorker := worker.NewNotificationWorker(notifService, stmtRepo, debtRepo, targetRepo, txRepo, pushSubRepo, logg)
	notifWorker.Start(context.Background())

	r := router.New(gormDB, logg)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		logg.Sugar().Infow("starting server", "addr", srv.Addr)
		logg.Sugar().Infoln("swagger docs: http://localhost:" + cfg.Port + "/swagger/index.html")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logg.Sugar().Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	logg.Sugar().Info("shutting down server...")

	// Stop background workers
	rolloverWorker.Stop()
	notifWorker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logg.Sugar().Fatalf("server forced to shutdown: %v", err)
	}

	// Close Database connection pool
	sqlDB, err := gormDB.DB()
	if err == nil {
		logg.Sugar().Info("closing database connections...")
		_ = sqlDB.Close()
	}
}
