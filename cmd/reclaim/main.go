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
	"reclaim/internal/scanner"
	"reclaim/internal/startup"
	"reclaim/internal/store"
	"reclaim/internal/worker"
	"reclaim/web"
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

	// Live holds the runtime-mutable settings (encode window, probe concurrency,
	// scan interval) so PUT /api/settings takes effect without a restart.
	live := config.NewLive(cfg)

	sc, err := scanner.New(db, cfg, scanner.WithLiveConfig(live))
	if err != nil {
		slog.Error("scanner init failed", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Scanner runs the startup scan then drives the watcher + scheduled rescan.
	go sc.Start(ctx)

	apiSrv := api.New(api.Deps{
		Store:       db,
		Scanner:     sc,
		Live:        live,
		MoviesPath:  cfg.MoviesPath,
		TVPath:      cfg.TVPath,
		DisableAuth: cfg.DisableAuth,
		StaticFS:    web.FS(),
	})

	// Worker executes encodes within the window and pushes progress over the
	// server's WS hub; the API drives it to cancel running jobs.
	wk := worker.New(db, live, apiSrv.Hub(), []string{cfg.MoviesPath, cfg.TVPath})
	apiSrv.SetCanceller(wk)
	go wk.Run(ctx)

	handler := apiSrv.Handler()

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

	cancel() // stop scanner

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}
