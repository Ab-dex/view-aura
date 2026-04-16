package stripeclient

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

func ProvideStripeClient(cfg *config.Config) *Client {
	return New(cfg.Stripe)
}

// ProviderSet wires *stripe.Client into the dependency graph.
// Modules that need Stripe (payment) declare *stripe.Client as a constructor
// parameter — Wire resolves it here.
var ProviderSet = wire.NewSet(ProvideStripeClient)
