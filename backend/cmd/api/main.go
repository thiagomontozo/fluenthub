package main

import (
	"context"
	"errors"
	"github.com/thiagomontozo/fluenthub/backend/internal/assessment"
	"github.com/thiagomontozo/fluenthub/backend/internal/billing"
	"github.com/thiagomontozo/fluenthub/backend/internal/certificates"
	"github.com/thiagomontozo/fluenthub/backend/internal/config"
	"github.com/thiagomontozo/fluenthub/backend/internal/database"
	"github.com/thiagomontozo/fluenthub/backend/internal/http/router"
	"github.com/thiagomontozo/fluenthub/backend/internal/notifications"
	"github.com/thiagomontozo/fluenthub/backend/internal/records"
	"github.com/thiagomontozo/fluenthub/backend/internal/scheduler"
	"github.com/thiagomontozo/fluenthub/backend/internal/setup"
	"github.com/thiagomontozo/fluenthub/backend/internal/storage"
	"github.com/thiagomontozo/fluenthub/backend/internal/support"
	"github.com/thiagomontozo/fluenthub/backend/internal/users"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "error", err)
		os.Exit(1)
	}
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	db, err := database.Open(rootCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database startup failed", "error", err)
		os.Exit(1)
	}
	store, err := storage.NewLocal(cfg.StoragePath, cfg.MaxUploadBytes)
	if err != nil {
		logger.Error("storage startup failed", "error", err)
		db.Close()
		os.Exit(1)
	}
	hub := notifications.NewHub(db)
	go hub.Run(rootCtx)
	userStore := users.NewStore(db)
	billingService := billing.NewService(db, billing.MockProvider{})
	notificationService := notifications.NewService(db, hub)
	handler := router.New(router.Dependencies{DB: db, Storage: store, Users: userStore, Certificates: certificates.New(db), Hub: hub, Billing: billingService, Notifications: notificationService, Assessment: assessment.New(db), Support: support.NewService(db), Records: records.New(db), Setup: setup.New(db), WebOrigin: cfg.WebOrigin, Environment: cfg.Environment})
	server := &http.Server{Addr: ":" + cfg.Port, Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 0, IdleTimeout: 60 * time.Second}
	sched := scheduler.New(logger, time.Minute, func(context.Context) error { return nil })
	go sched.Run(rootCtx)
	go func() {
		logger.Info("FluentHub API listening", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server stopped unexpectedly", "error", err)
			stop()
		}
	}()
	<-rootCtx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	hub.Close()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown timed out", "error", err)
	}
	_ = store.Close()
	db.Close()
	logger.Info("shutdown complete")
}
