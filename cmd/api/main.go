// Package main is the ViewAura API entry point.
//
// ViewAura REST API — the backend for every client (web, iOS, Android, smart TV).
// All endpoints follow JSON:API-style error responses with machine-readable codes.
//
// # Authentication
//
// Most write endpoints and all personal-data endpoints require a Bearer JWT in
// the Authorization header:
//
//	Authorization: Bearer <access_token>
//
// Tokens are obtained from POST /api/v1/users/login or POST /api/v1/users/register.
// The access token expires in 15 minutes; use POST /api/v1/users/refresh to rotate
// the pair before expiry.
//
// # Error response shape
//
// All errors are returned as:
//
//	{
//	  "error": {
//	    "code":    "VALIDATION_ERROR",
//	    "message": "human-readable description",
//	    "details": { ... }   // optional structured payload
//	  }
//	}
//
//	@title       Viewaura API
//	@version     1.0
//	@description REST API for the Viewaura movie platform.
//
//	@contact.name  Viewaura Engineering
//	@contact.email api@viewaura.com
//	@contact.url   https://viewaura.com
//
//	@license.name MIT
//	@license.url  https://opensource.org/licenses/MIT
//
//	@host     localhost:8080
//	@BasePath /api/v1
//
//	@schemes http https
//
//	@securityDefinitions.apikey BearerAuth
//	@in                         header
//	@name                       Authorization
//	@description                JWT access token — prefix with "Bearer "
//
//	@tag.name        Auth
//	@tag.description Registration, login, token refresh, logout
//
//	@tag.name        Users
//	@tag.description Profile, preferences, sessions
//
//	@tag.name        Movies
//	@tag.description Catalog, cast/crew, streaming availability, filming locations
//
//	@tag.name        People
//	@tag.description Cast, crew, filmographies
//
//	@tag.name        Genres
//	@tag.description Genre taxonomy
//
//	@tag.name        Ratings
//	@tag.description Multi-dimensional ratings and aggregated stats
//
//	@tag.name        Reviews
//	@tag.description Written and video reviews, reactions, reports
//
//	@tag.name        Watchlist
//	@tag.description Personal watch-status entries
//
//	@tag.name        Lists
//	@tag.description Custom curated movie lists
//
//	@tag.name        Social
//	@tag.description Follow graph, activity feed, discussion threads, challenges
//
//	@tag.name        Notifications
//	@tag.description In-app inbox, preferences, push token registration
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/rs/zerolog/log"
)

func main() {
	cfgPath := flag.String("config", "config/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a, err := InitializeApp(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize app")
	}

	log.Info().
		Str("env", cfg.App.Env).
		Str("version", cfg.App.Version).
		Msg("starting " + cfg.App.Name)

	srvErr := make(chan error, 1)
	go func() {
		srvErr <- a.Server.ListenAndServe()
	}()

	select {
	case err := <-srvErr:
		if err != nil {
			log.Error().Err(err).Msg("server stopped unexpectedly")
			os.Exit(1)
		}

	case <-ctx.Done():
		log.Info().Msg("shutdown signal received — draining connections")

		shutCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
		defer cancel()

		if err := a.Server.Shutdown(shutCtx); err != nil {
			log.Error().Err(err).Msg("graceful shutdown error")
			os.Exit(1)
		}
	}

	log.Info().Msg("view-aura exited cleanly")
}
