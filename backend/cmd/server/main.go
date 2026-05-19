package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/api"
	"github.com/abnervoynich/yuno-code/backend/internal/api/handlers"
	"github.com/abnervoynich/yuno-code/backend/internal/config"
	"github.com/abnervoynich/yuno-code/backend/internal/reconciliation"
	"github.com/abnervoynich/yuno-code/backend/internal/repository"
)

func main() {
	cfg := config.Load()

	level := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	db, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Repositories
	txnRepo := repository.NewTransactionRepo(db)
	settleRepo := repository.NewSettlementRepo(db)
	recoRepo := repository.NewReconciliationRepo(db)

	// Services
	recoSvc := reconciliation.NewService(txnRepo, settleRepo, recoRepo)

	// Handlers
	txnHandler := handlers.NewTransactionHandler(txnRepo)
	settleHandler := handlers.NewSettlementHandler(settleRepo, cfg.MaxUploadMB)
	recoHandler := handlers.NewReconciliationHandler(recoSvc, recoRepo)

	// Router
	router := api.NewRouter(txnHandler, settleHandler, recoHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port, "env", cfg.Environment)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}
