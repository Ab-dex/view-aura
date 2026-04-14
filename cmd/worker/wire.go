package main

import (
	"context"

	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/events"
	"github.com/Ab-dex/view-aura/internal/platform/cache"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/Ab-dex/view-aura/internal/platform/db"
	"github.com/Ab-dex/view-aura/internal/platform/r2"
	stripeclient "github.com/Ab-dex/view-aura/internal/platform/stripe"
	"github.com/Ab-dex/view-aura/internal/platform/temporal"
)

// ─── Infrastructure providers for the worker ─────────────────────────────────

func provideDB(ctx context.Context, cfg *config.Config) (*db.Pool, error) {
	return db.New(ctx, cfg.DB)
}

func provideRedis(ctx context.Context, cfg *config.Config) (*cache.Client, error) {
	return cache.New(ctx, cfg.Redis)
}

// ─── Wire initialiser ─────────────────────────────────────────────────────────

// InitializeWorker builds the worker binary's dependency graph.
//
// The worker is NOT an HTTP server — it:
//  1. Connects to Temporal and registers all workflow + activity implementations.
//  2. Starts a Kafka consumer loop (moderation decisions, upload events).
//  3. Uses the same DB, Redis, R2, Stripe, and Temporal clients as the API
//     binary so there is a single source of truth for infrastructure config.
//
// Platform dependencies:
//   - *db.Pool          — needed by upload and moderation activities
//   - *cache.Client     — needed by quota and upload session activities
//   - *r2.Client        — needed by upload pipeline activities (copy/delete objects)
//   - *stripe.Client    — needed by payment activities (subscription status sync)
//   - temporal.Client   — needed by the worker to register and poll
//   - events.Producer   — publishes workflow outcome events back to Kafka
//   - events.Consumer   — consumes uploads.completed and moderation topics
func InitializeWorker(ctx context.Context, cfg *config.Config) (*Worker, error) {
	wire.Build(
		// ── Infrastructure ────────────────────────────────────────────────────
		provideDB,
		provideRedis,

		// ── Platform extensions ───────────────────────────────────────────────
		r2.ProviderSet,
		stripeclient.ProviderSet,
		temporal.ProviderSet,

		// ConsumerSet provides both Producer (for publishing outcomes) and
		// Consumer (for reading trigger events).
		events.WorkerProviderSet,

		// ── Worker root ───────────────────────────────────────────────────────
		NewWorker,
	)

	return nil, nil
}
