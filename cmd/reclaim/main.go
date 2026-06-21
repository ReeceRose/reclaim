package main

import (
	"log/slog"
	"os"

	"reclaim/internal/config"
	"reclaim/internal/startup"
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

	slog.Info("startup checks passed")
}
