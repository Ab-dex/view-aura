package tracing

import (
	"context"

	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

func provideTracingProvider(ctx context.Context, cfg *config.Config) (*Provider, error) {
	return New(ctx, cfg.Tracing)
}

// ProviderSet wires *tracing.Provider into the dependency graph.
// The Provider's Shutdown method must be registered in the app lifecycle hooks
// so spans are flushed before the process exits.
var ProviderSet = wire.NewSet(provideTracingProvider)
