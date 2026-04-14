package r2

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

func provideR2Client(cfg *config.Config) *Client {
	return New(cfg.R2)
}

// ProviderSet wires *r2.Client into the dependency graph.
// Modules that need R2 (upload, moderation) declare *r2.Client as a
// constructor parameter — Wire resolves it here.
var ProviderSet = wire.NewSet(provideR2Client)
