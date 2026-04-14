package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

func main() {
	// ── Load config
	cfgPath := flag.String("config", "config/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg("worker: failed to load config")
	}

	// ── Initialise logger ─────────────────────────────────────────────────────
	logger.Init(cfg.Log.Level, cfg.Log.Format)

	log.Info().
		Str("env", cfg.App.Env).
		Str("task_queue", cfg.Temporal.TaskQueue).
		Msg("worker: starting")

	// ── Build dependency graph via Wire ───────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := InitializeWorker(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("worker: failed to initialise")
	}

	// ── Graceful shutdown on SIGTERM / SIGINT ─────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Run(ctx)
	}()

	select {
	case sig := <-quit:
		log.Info().Str("signal", sig.String()).Msg("worker: shutdown signal received")
		cancel()
	case err := <-errCh:
		if err != nil {
			log.Error().Err(err).Msg("worker: run error")
		}
		cancel()
	}

	// Wait for the run loop to exit cleanly.
	if err := <-errCh; err != nil {
		log.Error().Err(err).Msg("worker: exited with error")
		os.Exit(1)
	}

	log.Info().Msg("worker: shutdown complete")
}
