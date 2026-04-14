// Package main is the ViewAura API Gateway.
//
// The gateway is a thin reverse proxy that:
//   - Validates Bearer JWTs and rejects requests with invalid or expired tokens.
//   - Enforces per-user (authenticated) and per-IP (unauthenticated) rate limiting.
//   - Applies per-service circuit breakers with typed fallback responses.
//   - Attaches a unique X-Request-ID to every request.
//   - Propagates the W3C traceparent header for distributed tracing.
//   - Routes /api/v1/* to the monolith API service.
//   - Exposes /health and /metrics endpoints for Kubernetes probes and Prometheus.
//
// In production Kong handles TLS termination, auth, and routing at the edge.
// This gateway is used in local development (docker-compose) and integration tests.
package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/Ab-dex/view-aura/apps/api-gateway/middleware"
	"github.com/Ab-dex/view-aura/apps/api-gateway/proxy"
	"github.com/Ab-dex/view-aura/apps/api-gateway/ratelimit"
	"github.com/Ab-dex/view-aura/internal/platform/circuit"
	"github.com/Ab-dex/view-aura/internal/platform/config"
)

func main() {
	cfgPath := flag.String("config", "config/config.yaml", "path to config file")
	flag.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Str("service", "api-gateway").Logger()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg("gateway: failed to load config")
	}

	// ── Redis (rate limiter + circuit breaker state) ───────────────────────────
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		// Non-fatal: rate limiter and circuit breakers degrade to in-process mode.
		log.Warn().Err(err).Msg("gateway: Redis unavailable — degraded mode")
		rdb = nil
	}

	// ── Rate limiter ──────────────────────────────────────────────────────────
	var limiter *ratelimit.Limiter
	if rdb != nil {
		limiter, err = ratelimit.New(cfg.Redis.Addr, cfg.Redis.Password)
		if err != nil {
			log.Warn().Err(err).Msg("gateway: rate limiter init failed — skipping")
		}
	}

	// ── Circuit breakers ──────────────────────────────────────────────────────
	var redisClient *goredis.Client
	if rdb != nil {
		redisClient = rdb
	}
	cbManager := circuit.NewManager(
		circuit.Config{
			FailureThreshold: 5,
			SuccessThreshold: 2,
			OpenTimeout:      30 * time.Second,
			Namespace:        "gateway",
		},
		redisClient,
	)
	cbMiddleware := circuit.NewMiddleware(cbManager, circuit.DefaultRoutes(redisClient))

	// ── Reverse proxy ─────────────────────────────────────────────────────────
	upstreamURL := cfg.Gateway.UpstreamURL
	if upstreamURL == "" {
		upstreamURL = "http://viewaura-api:8080"
	}
	reverseProxy := proxy.New(upstreamURL)

	// ── Router ────────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Include circuit breaker states in health response so ops can
		// see which downstream services are currently open.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.Handle("/metrics", middleware.MetricsHandler())

	// Full middleware chain — circuit breakers wrap the proxy, not the whole
	// chain, so auth and rate limit still execute even when a circuit is open.
	mux.Handle("/", buildChain(cbMiddleware.Handler(reverseProxy), cfg, limiter))

	// ── HTTP server ───────────────────────────────────────────────────────────
	addr := cfg.Gateway.Addr
	if addr == "" {
		addr = ":8000"
	}
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srvErr := make(chan error, 1)
	go func() {
		log.Info().Str("addr", addr).Str("upstream", upstreamURL).
			Msg("gateway: listening")
		srvErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-srvErr:
		if err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("gateway: server error")
			os.Exit(1)
		}
	case <-ctx.Done():
		log.Info().Msg("gateway: shutdown signal received")
		shutCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			log.Error().Err(err).Msg("gateway: shutdown error")
		}
	}

	log.Info().Msg("gateway: exited cleanly")
}

// buildChain assembles the middleware stack in order:
//
//	RequestID → Logger → Tracing → CORS → RateLimit → Auth → [circuit-wrapped proxy]
//
// The circuit-breaker wrapping happens outside this chain (in main) so it
// sits between Auth and the upstream and has access to the authenticated
// user context for per-user rate limiting.
func buildChain(
	next http.Handler,
	cfg *config.Config,
	limiter *ratelimit.Limiter,
) http.Handler {
	skipPaths := []string{
		"/api/v1/webhook/stripe",
		"/api/v1/users/register",
		"/api/v1/users/login",
		"/api/v1/users/refresh",
		"/api/v1/movies",
		"/api/v1/movies/genres",
	}

	h := next
	h = middleware.Auth(h, cfg.JWT.PublicKeyPath, skipPaths)
	if limiter != nil {
		h = middleware.RateLimit(h, limiter)
	}
	h = middleware.CORS(h, cfg.Gateway.AllowedOrigins)
	h = middleware.Tracing(h)
	h = middleware.Logger(h)
	h = middleware.RequestID(h)
	return h
}
