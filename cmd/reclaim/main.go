package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"reclaim/internal/api"
	"reclaim/internal/config"
	"reclaim/internal/startup"
	"reclaim/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config invalid", "err", err)
		os.Exit(1)
	}

	if err := startup.CheckBinaries(); err != nil {
		slog.Error("dependency check failed", "err", err)
		os.Exit(1)
	}

	if err := startup.CheckMounts(cfg.MoviesPath, cfg.TVPath); err != nil {
		slog.Error("mount check failed", "err", err)
		os.Exit(1)
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("database init failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if cfg.ResetAuth {
		if err := db.Settings.ResetAuth(context.Background()); err != nil {
			slog.Error("auth reset failed", "err", err)
			os.Exit(1)
		}
		slog.Warn("RESET_AUTH: credentials cleared — first-run setup required")
	}

	if !db.Settings.IsSetupComplete() {
		slog.Info("first-run setup mode", "hint", "open the app and complete /setup")
	}

	slog.Info("startup checks passed")

	handler := api.New(db.Settings, cfg.DisableAuth).Handler()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}
