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
	"syscall"
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

	// "Today"/"this month" are computed in the users' timezone, not the host's.
	if err := util.SetAppTimezone(cfg.AppTimezone); err != nil {
		log.Fatalf("invalid APP_TIMEZONE: %v", err)
	}

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

	// Web Push: disabled (endpoints answer 503) when VAPID keys are missing.
	pushService := router.NewPushServiceFromConfig(gormDB, cfg, logg)
	var reminderWorker *worker.DailyReminderWorker
	if pushService.Enabled() {
		if os.Getenv("VAPID_SUBJECT") == "" {
			logg.Warn("VAPID_SUBJECT is not set; using a placeholder subject (Apple may reject it)")
		}
		reminderService := service.NewPushReminderService(repository.NewReminderRepository(gormDB), pushService, logg)
		reminderWorker = worker.NewDailyReminderWorker(reminderService, logg)
		reminderWorker.Start()
	} else {
		logg.Info("web push disabled: VAPID_PUBLIC_KEY / VAPID_PRIVATE_KEY not set")
	}

	r := router.New(gormDB, logg,
		router.WithCORSAllowedOrigins(cfg.CORSAllowedOrigins),
		router.WithPushService(pushService),
	)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// WriteTimeout bounds normal JSON responses. The SSE chat stream
		// (/chat/stream) lifts it per request with http.ResponseController,
		// so long AI answers are not cut off.
		WriteTimeout:   60 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		logg.Sugar().Infow("starting server", "addr", srv.Addr)
		logg.Sugar().Infoln("swagger docs: http://localhost:" + cfg.Port + "/swagger/index.html")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logg.Sugar().Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	// PM2 stops/reloads with SIGINT and falls back to SIGTERM; handle both.
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logg.Sugar().Info("shutting down server...")

	// Stop background workers
	rolloverWorker.Stop()
	if reminderWorker != nil {
		reminderWorker.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logg.Sugar().Fatalf("server forced to shutdown: %v", err)
	}
	// Let in-flight background pushes finish (each is time-bounded).
	pushService.Wait()

	// Close Database connection pool
	sqlDB, err := gormDB.DB()
	if err == nil {
		logg.Sugar().Info("closing database connections...")
		_ = sqlDB.Close()
	}
}
