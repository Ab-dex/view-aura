package app

import (
	"context"
	"fmt"

	"github.com/Ab-dex/view-aura/internal/app/middleware"
	"github.com/Ab-dex/view-aura/internal/contract"
	authsvc "github.com/Ab-dex/view-aura/internal/modules/auth/service"
	"github.com/Ab-dex/view-aura/internal/platform/cache"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/Ab-dex/view-aura/internal/platform/db"
	"github.com/Ab-dex/view-aura/internal/platform/resilience"
)

// ProvideDB connects to Postgres.  This is the ONE hard startup requirement —
// if the database is unreachable the app exits immediately because no business
// operation is possible without it.
func ProvideDB(ctx context.Context, cfg *config.Config) (*db.Pool, error) {
	return db.New(ctx, cfg.DB)
}

// ProvideRedis attempts to connect to Redis.
//
// Unlike ProvideDB, a Redis failure is non-fatal.  The function returns nil
// rather than an error so Wire can propagate a nullable *cache.Client through
// the entire dependency graph.  Every downstream component that accepts
// *cache.Client must guard against nil:
//
//   - rate limiting → fail open (allow all requests)
//   - HTTP cache    → skip (cache miss on every request)
//   - session store → in-memory fallback
//   - resilient producer / mailer → skip Tier 2, go straight to Tier 3 file outbox
//
// If Redis.Addr is empty the function returns nil immediately — no dial attempted.
func ProvideRedis(ctx context.Context, cfg *config.Config) *cache.Client {
	if cfg.Redis.Addr == "" {
		fmt.Println("app: Redis.Addr not set — running without Redis (degraded mode)")
		return nil
	}
	client, err := cache.New(ctx, cfg.Redis)
	if err != nil {
		// Log and continue — Redis being down must not crash the API.
		fmt.Printf("app: Redis unavailable (%v) — running without Redis (degraded mode)\n", err)
		return nil
	}
	return client
}

func ProvideAuthMiddleware(tokens authsvc.TokenService) contract.AuthMiddleware {
	return contract.AuthMiddleware(middleware.Auth(tokens))
}

func ProvideRequestIDMiddleware() contract.RequestIDMiddleware {
	return contract.RequestIDMiddleware(middleware.RequestID())
}

func ProvideRecoveryMiddleware() contract.RecoveryMiddleware {
	return contract.RecoveryMiddleware(middleware.Recovery())
}

func ProvideLoggerMiddleware() contract.LoggerMiddleware {
	return contract.LoggerMiddleware(middleware.StructuredLogger())
}

// RegisterOutboxWorker registers the OutboxWorker as an app lifecycle hook
// so it starts when the app starts and shuts down cleanly on SIGTERM.
//
// Call this after InitializeApp() returns, before app.Run():
//
//	a, _ := InitializeApp(ctx, cfg)
//	app.RegisterOutboxWorker(a, outboxWorker)
//	a.Run()
func RegisterOutboxWorker(a *App, worker *resilience.OutboxWorker) {
	a.Hooks.OnStart(func(ctx context.Context) error {
		worker.Start(ctx)
		return nil
	})
	a.Hooks.OnShutdown(func(_ context.Context) error {
		worker.Stop()
		return nil
	})
}
